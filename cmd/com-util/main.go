package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"
	"gopkg.in/yaml.v3"
)

const msg = "test com this big message and very big message abracodabra stop"

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
	err = yaml.NewDecoder(fileConf).Decode(conf)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию %s", err)
	}

	log.Println(conf)
	return &conf
}

func main() {
	log.Println("Добро пожаловать в утилиту тестирования COM-портов!")
	// log.Println("Сканирование доступных портов в системе:")
	// ports, err := serial.GetPortsList()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// for _, port := range ports {
	// 	log.Printf("Найден порт: %v\n", port)
	// }

	conf := mustConf()

	var portOne, portTwo, path string
	flag.StringVar(&portOne, "first", "/dev/ttyS0", "first com port for testing")
	flag.StringVar(&portTwo, "second", "/dev/ttyS1", "second com port for testing")
	flag.StringVar(&path, "path", "", "path to config.yml with settings")

	flag.Parse()
	// log.Println("Валидация заданных портов для тестирования:")
	// if !slices.Contains(ports, portOne) {
	// 	log.Fatalf("❌ COM-порт %s отсутствует в системе", portOne)
	// }
	// if !slices.Contains(ports, portTwo) {
	// 	log.Fatalf("❌ COM-порт %s отсутствует в системе", portTwo)
	// }
	// log.Println("✅ Порты валидны. Запуск теста")
	// pairs := map[string]string{
	// 	ports[0]: ports[1],
	// 	ports[1]: ports[0],
	// }
	pairs := map[string]string{
		portOne: portTwo,
		portTwo: portOne,
	}

	var wg sync.WaitGroup
	result := make(chan bool, 1)
	defer close(result)

	var tests bool
	testsAttempt := 3

	for i := range testsAttempt {
		tests = true
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

		log.Printf("%d попытка прохождения теста с COM-портами\n", i+1)
		for rPort, wPort := range pairs {
			log.Printf("Тестирование пары rPort %s, wPort %s\n", rPort, wPort)
			wg.Add(2)
			go func(r string) {
				defer wg.Done()
				portRead(ctx, r, result)
			}(rPort)
			go func(w string) {
				defer wg.Done()
				portWrite(ctx, w)
			}(wPort)
			wg.Wait()

			res, ok := <-result
			if !ok || !res {
				tests = false
			}
		}
		cancel()
		if tests {
			break
		} else {
			log.Println("❌ Неудачная попытка прохождения теста. Перезапускаю тест.")
		}
	}

	if tests {
		log.Println("✅ Тест пройден")
	} else {
		log.Println("❌ Тест не пройден")
	}

}

func portRead(ctx context.Context, name string, ch chan bool) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		log.Println(err)
		ch <- false
		return
	}
	defer port.Close()

	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		log.Println(err)
		ch <- false
		return
	}

	if err := port.ResetInputBuffer(); err != nil {
		log.Println(err)
		ch <- false
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
			ch <- false
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
		ch <- true
	} else {
		ch <- false
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
