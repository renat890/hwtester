package tui

import (
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/progress"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
)

const (
	minLeftColWidth = 40
	minRightColWidth = 80
	minColHeight = 40
	footerHeight = 10

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

func testsPanel(tests []hw.HWTest, width int, heights ...int) string {
	rows := []string{head2Style.Render("ТЕСТЫ К ЗАПУСКУ"),}
	for _, t := range tests {
		rows = append(rows, fmt.Sprintf("%s %s", checkMark, t.Name()))
	}
	itog := fmt.Sprintf("Итого: %d тестов", len(tests))
	rows = append(rows, "", itog)
	left := lipgloss.JoinVertical(lipgloss.Left, rows...)
	width -= 2 * padding
	if len(heights) != 1 {
		return  borderStyle.Width(width).Render(left)
	}
	return  borderStyle.Width(width).Height(heights[0]).Render(left)
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
	innerWidth := width - 2 * border - 3 * padding
	tableWidth := width - padding
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Width(innerWidth).
		BorderBottom(false).BorderColumn(false).BorderLeft(false).BorderRight(false).BorderTop(false)
	
	field := borderStyle.Width(tableWidth).Render(
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
	width -= padding
	return multiRow(stress, label, width)
}

// Панели для runScreen

func progressPanel(p progress.Model, res []hw.TestResult, cur, all, width int) string {
	resCount := map[hw.Status]int{
		hw.Error: 0,
		hw.Pass: 0,
		hw.Fail: 0,
	}
	for _, val := range res {
		switch val.Status {
		case hw.Error:
			resCount[hw.Error]++
		case hw.Pass:
			resCount[hw.Pass]++
		case hw.Fail:
			resCount[hw.Fail]++
		}
	}

	label := "Прогресс:"
	stat := fmt.Sprintf("%d / %d      %s %s %s %s",
		cur, all,
		passStyle.Render(fmt.Sprintf("✔ %d пройдено", resCount[hw.Pass])),
		failStyle.Render(fmt.Sprintf("✘ %d провалено", resCount[hw.Fail])),
		errorStyle.Render(fmt.Sprintf("◑ %d проблемно", resCount[hw.Error])),
		accentStyle.Render(fmt.Sprintf("○ %d ожидают", all - cur)),
	)

	progressWidth := width - lipgloss.Width(label) - lipgloss.Width(stat) - 4 * padding - 2 * border

	p.SetWidth(progressWidth)
	progressBar := p.ViewAs(float64(cur)/float64(all))
	progressBar = progressStyle.Render(progressBar)
	

	field := lipgloss.NewStyle().Padding(0, padding).Width(width).
			BorderTop(true).BorderForeground(lipgloss.Color(borderColor)).BorderStyle(lipgloss.NormalBorder()).
			Render(
				lipgloss.JoinHorizontal(
					lipgloss.Right, label, progressBar, stat,
				),
			)

	return field
}

func currentTestsPanel(res []hw.TestResult, all int, current, spinner string, width int) string {
	label := head2Style.Render("ХОД ТЕСТИРОВАНИЯ 68ХХ:")
	
	tests := make([]string, len(res), len(res) + 1)
	for i, val := range res {
		innerWidth := width - 2 * padding - 2 * border - lipgloss.Width(val.Name) - lipgloss.Width(string(val.Status))
		tests[i] = fmt.Sprintf("%s%s%s", val.Name, strings.Repeat(" ", innerWidth), statusWithStyle(val.Status))
	}
	// добавление текущего теста
	if all != len(res) {
		tests = append(tests, fmt.Sprintf("%s\t%s", current, spinner))
	}
	
	renderTests := lipgloss.JoinVertical(lipgloss.Left, tests...)

	return borderStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, renderTests),
	)
}

func logsPanel(logs []hw.LogMsg, width int) string {
	label := head2Style.Render("ЛОГ ВЫПОЛНЕНИЯ")

	logsStr := make([]string, len(logs))
	for i := range logs {
		logsStr[i] = fmt.Sprintf("%s %s %s", logs[i].Stamp.Format("15:04:05"), levelWithStyle(logs[i].Level), logs[i].Text)
	}
	logsField := lipgloss.JoinVertical(lipgloss.Left, logsStr...)	


	return borderStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, label, logsField))
}

func levelWithStyle(level hw.LogLevel) string {
	switch level {
	case hw.INFO:
		return infoLevelStyle.Render(string(level))
	case hw.WARN:
		return warnLevelStyle.Render(string(level))
	case hw.ERR:
		return errLevelStyle.Render(string(level))
	default:
		return string(level)
	}
}