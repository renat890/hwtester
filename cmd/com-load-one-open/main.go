package main

import (
	"context"
	"errors"
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

	openPortOne, err := openReadPort(portOne)
	if err != nil {
		log.Fatalf("Не удалось открыть порт %s для чтения: %s", portOne, err.Error())
	}

	openPortTwo, err := openWritePort(portTwo)
	if err != nil {
		log.Fatalf("Не удалось открыть порт %s для записи: %s", portTwo, err.Error())
	}

	// Тут начинаются тесты duratioin делим на 2
	testDuration := duration / 2

	// итерация 1
	ctx1, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	go testPair(ctx1, openPortOne, openPortTwo, portOne, portTwo, logger)
	<-ctx1.Done()
	openPortOne.Close()
	openPortTwo.Close()

	// итерация 2
	openPortOne, err = openWritePort(portOne)
	if err != nil {
		log.Fatalf("Не удалось открыть порт %s для записи: %s", portOne, err.Error())
	}
	openPortTwo, err = openReadPort(portTwo)
	if err != nil {
		log.Fatalf("Не удалось открыть порт %s для чтения: %s", portTwo, err.Error())
	}

	ctx2, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	go testPair(ctx2, openPortTwo, openPortOne, portTwo, portOne, logger)
	<-ctx2.Done()
	openPortOne.Close()
	openPortTwo.Close()
}

type testResult struct {
	state bool
	str   string
}

func testPair(ctx context.Context, openRead, openWrite serial.Port, read, write string, logger *slog.Logger) {
	iterNum := 1
	log.Printf("ЗАПУЩЕН воркер тестирования пары %s - %s\n", read, write)
	const sleppTime = 500 * time.Millisecond

	for {
		if ctx.Err() != nil {
			log.Printf("ОСТАНОВЛЕН воркер тестирования пары %s - %s\n", read, write)
			return
		}

		var wg sync.WaitGroup
		tests := true
		result := make(chan testResult, 1)
		defer close(result)

		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

		iterNum++

		wg.Add(2)
		go func(r serial.Port) {
			defer wg.Done()
			portRead(ctx, r, read, result)
		}(openRead)
		time.Sleep(sleppTime)
		go func(w serial.Port) {
			defer wg.Done()
			portWrite(ctx, w, write)
		}(openWrite)
		wg.Wait()

		res, ok := <-result
		if !ok || !res.state {
			tests = false
		}

		logger.Info("Результат тестирования пары COM-портов",
			"first_port", read,
			"second_port", write,
			"test_passed", tests,
			"iteration", iterNum-1,
			"result_str", res.str,
		)
		cancel()
	}
}

func openReadPort(name string) (serial.Port, error) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		log.Println(err)
		return nil, errors.New("Ошибка открытия COM-порта")

	}

	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		log.Println(err)
		return nil, errors.New("Ошибка установки таймаута чтения")
	}

	if err := port.ResetInputBuffer(); err != nil {
		log.Println(err)
		return nil, errors.New("Ошибка сброса буфера ввода")
	}
	log.Printf("Запщуен порт читатель %s\n", name)
	return port, nil
}

func openWritePort(name string) (serial.Port, error) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		return nil, errors.New("Ошибка открытия COM-порта")
	}
	log.Printf("Запущен порт писатель wPort %s\n", name)
	return port, nil
}

func portRead(ctx context.Context, port serial.Port, name string, ch chan testResult) {
	var final strings.Builder
	buf := make([]byte, 8)
	numMsg := 1
	log.Printf("Чтение на порту rPort %s\n", name)

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

func portWrite(ctx context.Context, port serial.Port, name string) {
	log.Printf("Запись в wPort %s\n", name)
	if ctx.Err() != nil {
		return
	}
	_, err := port.Write(([]byte(msg)))
	if err != nil {
		log.Printf("Ошибка записи в COM-порт: %s\n", err.Error())
	}
}
