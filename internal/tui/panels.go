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
	leftColWidth = 30
	rightColWidth = 80
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

func multiRow(params []paramCfg, label string, width int) string {
	innerWidth := width - 2 * border - 2 * padding
	fields := []string{label,}
	for _, param := range params {
		fields = append(fields, paramRow(param, innerWidth))
	}
	return borderStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, fields...))
}

func testsPanel(tests []hw.HWTest) string {
	// для выравнивания с левым блоком
	const heightTests = 30
	rows := []string{head2Style.Render("ТЕСТЫ К ЗАПУСКУ"),}
	for _, t := range tests {
		rows = append(rows, fmt.Sprintf("%s %s", checkMark, t.Name()))
	}
	itog := fmt.Sprintf("Итого: %d тестов", len(tests))
	rows = append(rows, "", itog)
	left := lipgloss.JoinVertical(lipgloss.Left, rows...)
	left = borderStyle.Width(leftColWidth).Height(heightTests).Render(left)
	return left
}

func ramPanel(cfg config.RAM, width int) string {
	// для выравнивания с ПЗУ
	const heightRam = 7
	label := head2Style.Render("ОЗУ")

	innerWidth := width / 2 - 2 * border - 2 * padding
	strRam := paramRow(paramCfg{"Объем, Мб", strconv.Itoa(cfg.ValueMB)}, innerWidth)

	return borderStyle.Width(width / 2).Height(heightRam).
	Render(lipgloss.JoinVertical(lipgloss.Left, label, strRam))
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
	
	field := borderStyle.Width(rightColWidth).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, t.Render()),
	)

	return field
}

func comPanel(coms []string, width int) string {
	// для выравнивания с USB 
	const heightCOM = 5
	label := head2Style.Render("COM")
	tmp := strings.Join(coms, " | ")
	field := borderStyle.Width(rightColWidth / 2).Height(heightCOM).Render(
		lipgloss.JoinVertical(lipgloss.Left, label, tmp),
	)

	return field
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