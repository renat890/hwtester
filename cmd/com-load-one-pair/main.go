package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"gopkg.in/yaml.v3"
)

const msg = "stop"

type Config struct {
	Ports    []string      `yaml:"ports"`
	Duration time.Duration `yaml:"duration"`
}

func mustConf(path string) *Config {
	fileConf, err := os.Open(path)
	if err != nil {
		log.Fatalf("Не удалось открыть файл по пути %s %v", path, err)
	}
	var conf Config
	err = yaml.NewDecoder(fileConf).Decode(&conf)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию %s", err)
	}

	return &conf
}

func main() {
	log.Println("Добро пожаловать в утилиту тестирования COM-портов!")

	var portOne, portTwo, durationStr string
	flag.StringVar(&portOne, "first", "/dev/ttyS0", "first com port for testing")
	flag.StringVar(&portTwo, "second", "/dev/ttyS1", "second com port for testing")
	flag.StringVar(&durationStr, "duration", "10s", "test duration")

	flag.Parse()

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Fatalf("Не удалось преобразовать длительность тестов %s : %s", durationStr, err.Error())
	}

	logname := fmt.Sprintf("com_test_log_%s.json", time.Now().Format("20060102_150405000"))
	fileLog, err := os.OpenFile(logname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("Не удалось открыть файл для логирования %s", err.Error())
	}
	defer fileLog.Close()

	logger := slog.New(slog.NewJSONHandler(fileLog, nil))

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	go testPair(ctx, portOne, portTwo, logger)

	<-ctx.Done()

}

type testResult struct {
	state bool
	str   string
}

func testPair(ctx context.Context, first, second string, logger *slog.Logger) {
	pairs := map[string]string{
		first:  second,
		second: first,
	}

	iterNum := 1
	log.Printf("ЗАПУЩЕН воркер тестирования пары %s - %s\n", first, second)
	const sleppTime = 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			log.Printf("ОСТАНОВЛЕН воркер тестирования пары %s - %s\n", first, second)
			return
		}

		var wg sync.WaitGroup
		var tests bool
		result := make(chan testResult, 1)
		defer close(result)

		tests = true
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

		// log.Printf("%d попытка прохождения теста с COM-портами\n", iterNum)
		iterNum++

		for rPort, wPort := range pairs {
			// log.Printf("Тестирование пары rPort %s, wPort %s\n", rPort, wPort)
			wg.Add(2)
			go func(r string) {
				defer wg.Done()
				portRead(ctx, r, result)
			}(rPort)
			time.Sleep(sleppTime)
			go func(w string) {
				defer wg.Done()
				portWrite(ctx, w)
			}(wPort)
			wg.Wait()

			res, ok := <-result
			if !ok || !res.state {
				tests = false
			}

			logger.Info("Результат тестирования пары COM-портов",
				"first_port", first,
				"second_port", second,
				"test_passed", tests,
				"iteration", iterNum-1,
				"result_str", res.str,
			)
		}
		cancel()
	}
}

func portRead(ctx context.Context, name string, ch chan testResult) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		log.Println(err)
		ch <- testResult{state: false, str: "Ошибка открытия COM-порта"}
		return
	}
	defer port.Close()

	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		log.Println(err)
		ch <- testResult{state: false, str: "Ошибка установки таймаута чтения"}
		return
	}

	if err := port.ResetInputBuffer(); err != nil {
		log.Println(err)
		ch <- testResult{state: false, str: "Ошибка сброса буфера ввода"}
		return
	}

	var final strings.Builder
	buf := make([]byte, 8)
	numMsg := 1
	log.Printf("Запущен порт читатель rPort %s\n", name)
reader:
	for {
		select {
		case <-ctx.Done():
			ch <- testResult{state: false, str: "Таймаут чтения"}
			return
		default:
			n, err := port.Read(buf)
			if err != nil {
				log.Printf("Ошибка чтения из COM-порта: %s\n", err.Error())
			}
			numMsg++
			if n == 0 {
				continue
			}
			// log.Printf("Получено сообщение #%d: %s\n",numMsg, string(buf[:n]))
			final.WriteString(string(buf[:n]))

			if strings.Contains(final.String(), "stop") {
				break reader
			}
		}
	}
	if msg == final.String() {
		ch <- testResult{state: true, str: ""}
	} else {
		ch <- testResult{state: false, str: final.String()}
	}
}

func portWrite(ctx context.Context, name string) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		log.Println(err)
		return
	}
	defer port.Close()
	log.Printf("Запущен порт писатель wPort %s\n", name)
	if ctx.Err() != nil {
		return
	}
	_, err = port.Write(([]byte(msg)))
	if err != nil {
		log.Printf("Ошибка записи в COM-порт: %s\n", err.Error())
	}
}
