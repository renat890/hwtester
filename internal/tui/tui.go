package tui

import (
	"context"
	"embed"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"strings"
	"text/template"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

//go:embed template/*.txt
var templateFS embed.FS

var tmplConf = template.Must(template.New("conf").ParseFS(templateFS, "template/*.txt"))

type screen int

const (
	startScreen screen = iota
	runScreen
	resultScreen
)

const (
	padding = 1
	border  = 1
)

type TestDoneMsg struct {
	Result hw.TestResult
}

type AllDoneMsg struct {
	Results []hw.TestResult
	Final   hw.Status
}

type ModelRunner interface {
	Run(ctx context.Context, tests []hw.HWTest, ch chan hw.TestResult, logCh chan hw.LogMsg) []hw.TestResult
}

type Model struct {
	currentScreen  screen
	cfg            config.Config
	mRunner        ModelRunner
	ch             chan hw.TestResult
	results        []hw.TestResult
	cancel         context.CancelFunc
	final          hw.Status
	logs           []hw.LogMsg
	fullLogs       []hw.LogMsg
	logCh          chan hw.LogMsg
	currentTest    string
	currentTestIdx int
	spin           spinner.Model
	version        string
	width          int
	height         int
	prog           progress.Model

	tests []hw.HWTest
}

func NewModel(cfg config.Config, mRunner ModelRunner, tests []hw.HWTest, version string) Model {
	return Model{
		currentScreen: startScreen,
		cfg:           cfg,
		mRunner:       mRunner,
		tests:         tests,
		logs:          []hw.LogMsg{},
		spin: spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(spinnerStyle),
		),
		prog: progress.New(
			progress.WithScaled(true),
			progress.WithColors(lipgloss.Green),
			progress.WithFillCharacters('█', '░'),
			progress.WithoutPercentage(),
		),
		version: version,
	}
}

func (m Model) Results() []hw.TestResult {
	return m.results
}

func (m Model) FullLogs() []hw.LogMsg {
	return m.fullLogs
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: сделать зависимой от размера экрана
	const logsSize = 40
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		if m.width <= (minLeftColWidth + minRightColWidth) {
			m.width = minLeftColWidth + minRightColWidth
		}
		m.height = msg.Height
		if m.height <= minColHeight {
			m.height = minColHeight
		}
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "enter":
			if m.currentScreen == resultScreen {
				return m, tea.Quit
			}
			if m.currentScreen == runScreen {
				break
			}
			m.currentScreen = runScreen
			m.ch = make(chan hw.TestResult)
			m.logCh = make(chan hw.LogMsg, 100)
			ctx, cancel := context.WithCancel(context.Background())
			m.cancel = cancel
			go func() {
				m.mRunner.Run(ctx, m.tests, m.ch, m.logCh)
				close(m.ch)
				close(m.logCh)
			}()
			return m, tea.Batch(m.waitForResult(m.ch), m.waitForLog(m.logCh), m.spin.Tick)
		case "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			if m.ch != nil {
				go func() {
					for range m.ch {
					}
				}()
			}
			if m.logCh != nil {
				go func() {
					for range m.logCh {
					}
				}()
			}

			m.currentScreen = resultScreen
			return m, nil
		}
	case TestDoneMsg:
		// тут ещё и указываем тест, который выполняется в текущий момент
		if m.currentTestIdx < (len(m.tests) - 1) {
			m.currentTestIdx++
		}

		m.results = append(m.results, msg.Result)
		return m, m.waitForResult(m.ch)
	case hw.LogMsg:
		m.logs = append(m.logs, msg)
		m.fullLogs = append(m.fullLogs, msg)
		// тут перестроение буфера для логов
		if tmp := len(m.logs); tmp > logsSize {
			m.logs = m.logs[tmp-logsSize:]
		}
		return m, m.waitForLog(m.logCh)
	case LastLogMsg:
		last := hw.LogMsg{Level: hw.INFO, Text: string(msg), Stamp: time.Now()}
		m.logs = append(m.logs, last)
		m.fullLogs = append(m.fullLogs, last)
		// тут перестроение буфера для логов
		if tmp := len(m.logs); tmp > logsSize {
			m.logs = m.logs[tmp-logsSize:]
		}
		return m, nil
	case AllDoneMsg:
		m.currentScreen = resultScreen
		// time.Sleep(30 * time.Second) // УДАЛИТЬ!
		// остановка спиннера
		m.spin = spinner.New()
		m.final = msg.Final
		m.results = msg.Results
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() tea.View {
	s := strings.Builder{}

	leftColWidth := m.width/3 - 2*padding
	rightColWidth := m.width - leftColWidth - 2*padding

	switch m.currentScreen {
	case startScreen:
		title := headStyle.Render(fmt.Sprintf("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ %s - стартовая конфигурация", m.version))

		fieldROM := romPanel(m.cfg.ROM, rightColWidth/2)
		fieldRam := ramPanel(m.cfg.RAM, rightColWidth/2, lipgloss.Height(fieldROM))
		fieldEth := ethPanel(m.cfg.Ports.Ethernets, rightColWidth)
		fieldUSB := usbPanel(m.cfg.USBFlash, rightColWidth/2)
		fieldCOM := comPanel(m.cfg.Ports.COM, rightColWidth/2, lipgloss.Height(fieldUSB))
		fieldStress := stressPanel(m.cfg.Stress, rightColWidth)

		right := borderStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			head2Style.Render("Параметры"),
			lipgloss.JoinHorizontal(lipgloss.Left, fieldRam, fieldROM),
			fieldEth,
			lipgloss.JoinHorizontal(lipgloss.Left, fieldCOM, fieldUSB),
			fieldStress,
		))

		// формирование блока с тестами
		left := testsPanel(m.tests, leftColWidth, lipgloss.Height(right))
		// все тело экрана
		body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

		footer := "Enter - запустить q - выход"
		s.WriteString(lipgloss.JoinVertical(lipgloss.Left, title, body, footer))
	case runScreen:
		s.Reset()
		// TODO: разобщить титул
		title := headStyle.Render(fmt.Sprintf("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ %s 00:00:00 68XX s/n 00000", m.version))

		progressBar := progressPanel(m.prog, m.results, len(m.results), len(m.tests), m.width)
		s.WriteByte(byte('\n'))

		// блок с логам
		right := logsPanel(m.logs, rightColWidth)
		// Блок с текущими тестами
		// TODO: подумать с шириной левого блока, пока константа
		currentTest := m.tests[m.currentTestIdx].Name()
		left := currentTestsPanel(m.results, len(m.tests), currentTest, m.spin.View(), leftColWidth, lipgloss.Height(right))

		s.WriteString(lipgloss.JoinVertical(
			lipgloss.Left,
			title,
			progressBar,
			lipgloss.JoinHorizontal(lipgloss.Top, left, right)),
		)
	case resultScreen:
		s.Reset()
		title := headStyle.Render(fmt.Sprintf("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ %s 68XX · s/n 00000 Завершено: 01-01-1970 00:00:00 Длительность: 00:00", m.version))
		s.WriteByte(byte('\n'))

		s.WriteString(lipgloss.JoinVertical(lipgloss.Left,
			title,
			generalResultPanel(m.final, m.results, m.width),
			resultsPanel(m.results, m.final, m.width),
		))
	}

	v := tea.NewView(s.String())
	// Полный экран и цвет другой для текста
	v.AltScreen = true
	v.ForegroundColor = lipgloss.Color(defaultColor)

	return v
}

func (m Model) waitForResult(ch chan hw.TestResult) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			final := hw.Pass

			for _, val := range m.results {
				if val.Status == hw.Error || val.Status == hw.Fail {
					final = hw.Fail
					break
				}
			}

			return AllDoneMsg{
				Results: m.results,
				Final:   final,
			}
		}

		return TestDoneMsg{Result: result}
	}
}

type LastLogMsg string

func (m Model) waitForLog(ch chan hw.LogMsg) tea.Cmd {
	return func() tea.Msg {
		str, ok := <-ch
		if !ok {
			return LastLogMsg("Тестирование окончено")
		}
		return hw.LogMsg(str)
	}
}
