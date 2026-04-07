package tui

import (
	"bytes"
	"context"
	"embed"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"slices"
	"strings"
	"text/template"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
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
	border = 1
)

type TestDoneMsg struct {
	Result hw.TestResult
}

type AllDoneMsg struct {
	Results []hw.TestResult
	Final   hw.Status
}



type ModelRunner interface {
	Run(ctx context.Context, tests []hw.HWTest, ch chan hw.TestResult, logCh chan string) []hw.TestResult
}

type Model struct {
	currentScreen screen
	cfg           config.Config
	mRunner       ModelRunner
	ch            chan hw.TestResult
	results       []hw.TestResult
	cancel        context.CancelFunc
	final         hw.Status
	logs 		  []string
	logCh         chan string
	currentTest   string
	spin          spinner.Model
	version string      

	tests []hw.HWTest
}

func NewModel(cfg config.Config, mRunner ModelRunner, tests []hw.HWTest, version string) Model {
	var cur string
	if len(tests) > 0 {
		cur = tests[0].Name()
	}

	return Model{
		currentScreen: startScreen,
		cfg:           cfg,
		mRunner:       mRunner,
		tests:         tests,
		logs: []string{},
		currentTest: cur,
		spin: spinner.New(
			spinner.WithSpinner(spinner.Points),
			spinner.WithStyle(spinnerStyle),
		),
		version: version,
	}
}

func (m Model) Results() []hw.TestResult {
	return m.results
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// TODO: сделать зависимой от размера экрана
	const logsSize = 20
	switch msg := msg.(type) {
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
			m.logCh = make(chan string, 100)
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
				go func ()  {
					for range m.logCh {}	
				}()
			}

			m.currentScreen = resultScreen
			return m, nil
		}
	case TestDoneMsg:
		// тут ещё и указываем тест, который выполняется в текущий момент
		var curInd int
		if len(m.tests) != 0 {
			curInd = slices.IndexFunc(m.tests, func(t hw.HWTest) bool {
				return t.Name() == m.currentTest
			})
		}
		if curInd < (len(m.tests) - 1)  {
			m.currentTest = m.tests[curInd + 1].Name()
		}

		m.results = append(m.results, msg.Result)
		return m, m.waitForResult(m.ch)
	case LogMsg:
		m.logs = append(m.logs, string(msg))
		// тут перестроение буфера для логов
		if tmp := len(m.logs); tmp > logsSize {
			m.logs = m.logs[tmp - logsSize:]
		}
		return m, m.waitForLog(m.logCh)
	case AllDoneMsg:
		m.currentScreen = resultScreen
		// TODO: заменить на продолжение при помощи нажатия клавиши
		time.Sleep(1 * time.Second) // УБРАТЬ!
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

	switch m.currentScreen {
	case startScreen:
		s.Reset()
		title := headStyle.Render(fmt.Sprintf("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ %s - стартовая конфигурация", m.version))

		// формирование блока с тестами
		left := testsPanel(m.tests)

		fieldROM := romPanel(m.cfg.ROM, rightColWidth / 2)
		fieldRam := ramPanel(m.cfg.RAM, rightColWidth / 2)
		fieldEth := ethPanel(m.cfg.Ports.Ethernets, rightColWidth)
		fieldCOM := comPanel(m.cfg.Ports.COM, rightColWidth)
		fieldUSB := usbPanel(m.cfg.USBFlash, rightColWidth / 2)
		fieldStress := stressPanel(m.cfg.Stress, rightColWidth)

		right := borderStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
			head2Style.Render("Параметры"),
			lipgloss.JoinHorizontal(lipgloss.Left, fieldRam, fieldROM),
			fieldEth,
			lipgloss.JoinHorizontal(lipgloss.Left, fieldCOM, fieldUSB),
			fieldStress,
		))

		// все тело экрана
		body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

		footer := "Enter - запустить q - выход"
		s.WriteString(lipgloss.JoinVertical(lipgloss.Left, title, body, footer))
	case runScreen:
		s.Reset()
		s.WriteString(headStyle.Render("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ"))
		s.WriteByte(byte('\n'))

		var logBuilder strings.Builder
		logBuilder.WriteString(head2Style.Render("Лог:"))
		logBuilder.WriteString("\n\n")
		logBuilder.WriteString(strings.Join(m.logs, ""))

		logField := borderStyle.Render(logBuilder.String())

		var tmplBuilder strings.Builder

		tmplBuilder.WriteString(head2Style.Render("Ход тестирования 68хх:"))
		tmplBuilder.WriteString("\n\n")
		for _, val := range m.results {
			tmplBuilder.WriteString(fmt.Sprintf("%s\t%s\n", val.Name, styledStatus(val.Status)))
		}
		tmplBuilder.WriteString(fmt.Sprintf("%s\t%s", m.currentTest, m.spin.View()))
		runField := borderStyle.Render(tmplBuilder.String())

		// объединение в 2 столбца
		s.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, runField, logField))
	case resultScreen:
		s.Reset()
		s.WriteString(headStyle.Render("УТИЛИТА ТЕСТИРОВАНИЯ 68ХХ"))
		s.WriteByte(byte('\n'))

		s.WriteString("Результаты тестирования:\n")
		s.WriteString(genResultString(m.results))
		s.WriteString(fmt.Sprintf("Общий результат: %s\n", styledStatus(m.final)))
	}

	v := tea.NewView(s.String())
	// Полный экран и цвет другой для текста
	v.AltScreen = true
	v.ForegroundColor = lipgloss.Color(defaultColor)

	return v
}

func genConfString(cfg config.Config) string {
	var buffer bytes.Buffer
	if err := tmplConf.ExecuteTemplate(&buffer, "conf.txt", cfg); err != nil {
		return "не удалось создать текст конфигурации"
	}

	return buffer.String()
}

func styledStatus(status hw.Status) hw.Status {
	var newStatus string
	switch status {
	case hw.Pass:
		newStatus = passStyle.Render(string(hw.Pass))
	case hw.Error:
		newStatus = errorStyle.Render(string(hw.Error))
	case hw.Fail:
		newStatus = failStyle.Render(string(hw.Fail))
	}
	return hw.Status(newStatus)
}

func genResultString(items []hw.TestResult) string {
	itemsCopy := make([]hw.TestResult, len(items))
	copy(itemsCopy, items)
	// делаю результаты разных цветов
	for i := range itemsCopy {
		itemsCopy[i].Status = styledStatus(itemsCopy[i].Status )
	}

	tHeaders := []string{"Имя","Статус","Детали","Метрики"}
	tRows := [][]string{}

	for _, row := range itemsCopy {
		var metrs strings.Builder
		for key, val := range row.Metrics {
			metrs.WriteString(fmt.Sprintf("%s: %v\n", key, val))
		}
		tRows = append(tRows, []string{row.Name, string(row.Status), row.Details, metrs.String() })
	}
	
	t := table.New().
		Headers(tHeaders...).
		Rows(tRows...)

	out := fmt.Sprintf("%s\n", t.Render())
	return out
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

type LogMsg string
type LastLogMsg string

func (m Model) waitForLog(ch chan string) tea.Cmd {
	return func() tea.Msg {
		str, ok := <- ch
		if !ok {
			return LastLogMsg("")
		}
		return LogMsg(str)
	}
}
