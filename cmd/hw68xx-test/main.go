package main

import (
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"factorytest/internal/report"
	"factorytest/internal/runner"
	"factorytest/internal/tests"
	"factorytest/internal/tui"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

var version = "dev"

func main() {
	if os.Getuid() != 0 {
		log.Fatal("программа должна быть запущена с правами суперпользователя")
	}
	
	cfgPath := flag.String("config", "./default.yml", "path to config file in .yml format")
	flag.Parse()

	fmt.Fprint(os.Stdout, "hello, factory test!\n")
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию: %s\n", err)
	}
	impl := tests.HardwareUsage{}
	hwTests := []hw.HWTest{
		tests.NewTestRAM(&impl, cfg.RAM.ValueMB),
		tests.NewTestRom(&impl, cfg.ROM),
		tests.NewTestCPU(&impl, cfg.Stress),
		tests.NewEthernetsTest(&impl, cfg.Ports),
		tests.NewTestCOM(&impl, cfg.Ports),
		tests.NewUSBTest(&impl, cfg.USBFlash),
	}

	r := runner.NewTestRunner()
	model := tui.NewModel(*cfg, r, hwTests, version)
	modelR, errR := tea.NewProgram(model).Run()
	if errR != nil {
		log.Fatalf("не удалось корректно завершить программу: %v", errR)
	}
	model = modelR.(tui.Model)
	// TODO: пока заглушка, в будущем должна программа создавать
	meta := report.Meta{
		Date:       time.Now(),
		DeviceName: "68xx",
		VersionOS:  "1",
	}
	rep := report.Generate(meta, model.Results())

	htmlFile, err := os.OpenFile("report.html", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("не удалось открыть файл для записи %v", err)
	}
	defer htmlFile.Close()

	jsonFile, err := os.OpenFile("report.json", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("не удалось открыть файл для записи %v", err)
	}
	defer jsonFile.Close()

	if htmlFile != nil {
		err = report.WriteHTML(htmlFile, rep)
		if err != nil {
			log.Printf("не удалось записать данные в файл: %v", err)
		}
	}
	
	if jsonFile != nil {
		err = report.WriteJSON(jsonFile, rep)
		if err != nil {
			log.Printf("не удалось записать данные в файл: %v", err)
		}
	}
}
