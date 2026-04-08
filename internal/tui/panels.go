package tui

import (
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const (
	minLeftColWidth = 40
	minRightColWidth = 80
	minColHeight = 30

	checkMark = "✓"
)

type paramCfg struct {
	Name string 
	Value string
}

func paramRow(p paramCfg, width int) string {
	dots := width - lipgloss.Width(p.Name) - lipgloss.Width(p.Value) - 2
	if dots < 0 {
		dots = 0
	}
	return fmt.Sprintf("%s %s %s", p.Name, strings.Repeat(".", dots), p.Value)
}

func multiRow(params []paramCfg, label string, width int, height ...int) string {
	innerWidth := width - 2 * border - 2 * padding
	fields := []string{label,}
	for _, param := range params {
		fields = append(fields, paramRow(param, innerWidth))
	}
	if len(height) != 1 {
		return borderStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, fields...))
	}
	return borderStyle.Width(width).Height(height[0]).Render(lipgloss.JoinVertical(lipgloss.Left, fields...))
}

func testsPanel(tests []hw.HWTest, heights ...int) string {
	rows := []string{head2Style.Render("ТЕСТЫ К ЗАПУСКУ"),}
	for _, t := range tests {
		rows = append(rows, fmt.Sprintf("%s %s", checkMark, t.Name()))
	}
	itog := fmt.Sprintf("Итого: %d тестов", len(tests))
	rows = append(rows, "", itog)
	left := lipgloss.JoinVertical(lipgloss.Left, rows...)
	if len(heights) != 1 {
		return  borderStyle.Width(minLeftColWidth).Render(left)
	}
	return  borderStyle.Width(minLeftColWidth).Height(heights[0]).Render(left)
}

func ramPanel(cfg config.RAM, width int, height ...int) string {
	label := head2Style.Render("ОЗУ")
	cfgRAM := []paramCfg{
		{"Объем, Мб", strconv.Itoa(cfg.ValueMB)},
	}
	return multiRow(cfgRAM, label, width, height...)
}

func romPanel(cfg config.ROM, width int) string {
	label := head2Style.Render("ПЗУ")
	cfgROM := []paramCfg{
		{"Дисков", fmt.Sprintf("%d", cfg.Nums)},
		{"Объем диска", fmt.Sprintf("%d", cfg.ValueMBEach)},
		{"Скорость чтения", fmt.Sprintf("%d МБ/с", cfg.MinReadVMBs)},
		{"Скорость записи", fmt.Sprintf("%d МБ/с", cfg.MinWriteVMBs)},
	}

	return multiRow(cfgROM, label, width)
}

func ethPanel(ports []config.Ethernet , width int) string {
	label := head2Style.Render("ETHERNET")
	headers := []string{"интерфейс","ip-адрес","порт"}
	rows := [][]string{}
	for _, eth := range ports {
		rows = append(rows, []string{eth.Name, eth.Ip, eth.Port})
	}
	innerWidth := width - 2 * border - 2 * padding
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Width(innerWidth).
		BorderBottom(false).BorderColumn(false).BorderLeft(false).BorderRight(false).BorderTop(false)
	
	field := borderStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, t.Render()),
	)

	return field
}

func comPanel(coms []string, width int, heights ...int) string {
	label := head2Style.Render("COM")
	tmp := strings.Join(coms, " | ")
	if len(heights) != 1 {
		return borderStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, label, tmp))
	}
	return borderStyle.Width(width).Height(heights[0]).Render(lipgloss.JoinVertical(lipgloss.Left, label, tmp))
}

func usbPanel(cfg config.USBFlash, width int) string {
	label := head2Style.Render("USB FLASH")
	usb := []paramCfg{
		{"Точка монтирования", cfg.MountPoint},
		{"Имя файла", cfg.Filename},
	}

	return multiRow(usb, label, width)
}

func stressPanel(cfg config.Stress, width int) string {
	label := head2Style.Render("СТРЕСС-ТЕСТ")
	stress := []paramCfg{
		{"Макс.нагрев", fmt.Sprintf("%d °C", cfg.MaxHeat)},
		{"Градиент", fmt.Sprintf("%d °C", cfg.Gradient)},
		{"Длительность", cfg.Duration.String()},
	}

	return multiRow(stress, label, width)
}

func currentTestsPanel(res []hw.TestResult, current, spinner string, width int) string {
	label := head2Style.Render("ХОД ТЕСТИРОВАНИЯ 68ХХ:")
	tests := make([]string, len(res), len(res) + 1)
	for i, val := range res {
		tests[i] = fmt.Sprintf("%s\t%s", val.Name, statusWithStyle(val.Status))
	}
	// добавление текущего теста
	tests = append(tests, fmt.Sprintf("%s\t%s", current, spinner))
	renderTests := lipgloss.JoinVertical(lipgloss.Left, tests...)

	return borderStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, renderTests),
	)
}

func logsPanel(logs []string, width int) string {
	label := head2Style.Render("ЛОГ ВЫПОЛНЕНИЯ")
	logsField := lipgloss.JoinVertical(lipgloss.Left, logs...)	


	return borderStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, label, logsField))
}

