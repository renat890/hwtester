package tui

import (
	"bytes"
	"embed"
	"factorytest/internal/config"
	"strings"
	"text/template"

	tea "charm.land/bubbletea/v2"
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

type Model struct {
	currentScreen screen
	cfg config.Config
}

func NewModel(cfg config.Config) Model {
	return Model{
		currentScreen: startScreen,
		cfg: cfg,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			m.currentScreen = runScreen
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
	s := strings.Builder{}

	switch m.currentScreen {
	case startScreen:
		s.WriteString("Тестирование 68хх по следующим пунктам:\n\n")
		tests := []string{"ОЗУ", "ПЗУ", "Порты COM", "Порты Ethernet",
		"Порты USB", "Стресс-тестирование (система охлаждения)"}
		for _, val := range tests {
			s.WriteString(val)
			s.WriteByte(byte('\n'))
		}
		s.WriteByte(byte('\n'))
		s.WriteString("Параметры для тестирования:\n")
		s.WriteString(genConfString(m.cfg))
	}

	return tea.NewView(s.String())
}

func genConfString(cfg config.Config) string {
	var buffer bytes.Buffer
	if err := tmplConf.ExecuteTemplate(&buffer, "conf.txt", cfg); err != nil {
		return "не удалось создать текст конфигурации"
	}

	return buffer.String()
}

