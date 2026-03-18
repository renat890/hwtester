package main

import (
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"factorytest/internal/report"
	"factorytest/internal/runner"
	"factorytest/internal/tests"
	"factorytest/internal/tui"
	"fmt"
	"log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

func main() {
	fmt.Fprint(os.Stdout, "hello, factory test!\n")
	cfg, err := config.Load("/home/renat/code/hwtester/configs/default.yml")
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %s\n", err)
	}
	impl := tests.HardwareUsage{}
	hwTests := []hw.HWTest{
		tests.NewTestRAM(&impl, cfg.RAM.ValueMB),
		tests.NewTestRom(&impl, cfg.ROM),
	}

	r := runner.NewTestRunner()
	model := tui.NewModel(*cfg, r, hwTests)
	modelR, errR := tea.NewProgram(model).Run()
	if errR != nil {
		log.Fatalf("не удалось корректно завершить программу: %v", errR)
	}
	model = modelR.(tui.Model)
	// TODO: пока заглушка, в будущем должна программа создавать
	meta := report.Meta{
		Date: time.Now(),
		DeviceName: "68xx",
		VersionOS: "1",
	}
	rep := report.Generate(meta, model.Results())

	htmlFile, err := os.OpenFile("report.html", os.O_CREATE | os.O_RDWR, 0644)
	if err != nil {
		log.Printf("не удалось открыть файл для записи %v", err)
	}
	defer htmlFile.Close()

	jsonFile, err := os.OpenFile("report.json", os.O_CREATE | os.O_RDWR, 0644)
	if err != nil {
		log.Printf("не удалось открыть файл для записи %v", err)
	}
	defer jsonFile.Close()

	err = report.WriteHTML(htmlFile, rep)
	if err != nil {
		log.Printf("не удалось записать данные в файл: %v", err)
	}
	err = report.WriteJSON(jsonFile, rep)
	if err != nil {
		log.Printf("не удалось записать данные в файл: %v", err)
	}
}
