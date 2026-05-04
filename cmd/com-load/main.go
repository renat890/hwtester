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
	err = yaml.NewDecoder(fileConf).Decode(&conf)
	if err != nil {
		log.Fatalf("Не удалось загрузить конфигурацию %s", err)
	}

	return &conf
}

func main() {
	log.Println("Добро пожаловать в утилиту тестирования COM-портов!")

	var portOne, portTwo, path string
	flag.StringVar(&portOne, "first", "/dev/ttyS0", "first com port for testing")
	flag.StringVar(&portTwo, "second", "/dev/ttyS1", "second com port for testing")
	flag.StringVar(&path, "path", "", "path to config.yml with settings")

	flag.Parse()

	conf := mustConf(path)

	if len(conf.Ports)%2 != 0 {
		log.Fatal("Количество портов должно быть чётным!")
	}

	pairs := map[string]string{}
	for i := range len(conf.Ports) {
		if i%2 == 0 {
			pairs[conf.Ports[i]] = conf.Ports[i+1]
			pairs[conf.Ports[i+1]] = conf.Ports[i]
		}
	}

	// lost := make(chan int)
	// defer close(lost)

	ctx, cancel := context.WithTimeout(context.Background(), conf.Duration)
	defer cancel()

	numWorker := 1
	for first, second := range pairs {
		go testPair(ctx, first, second, numWorker)
		numWorker++
	}

	<-ctx.Done()

}

func testPair(ctx context.Context, first, second string, numWorker int) {
	log.Printf("Запущен воркер тестирования пары %s - %s с номером %d\n", first, second, numWorker)

	pairs := map[string]string{
		first:  second,
		second: first,
	}

	for {
		if ctx.Err() != nil {
			log.Printf("ОСТАНОВЛЕН воркер тестирования пары %s - %s с номером %d\n", first, second, numWorker)
			return
		}
		var testsAttempt = 3
		var wg sync.WaitGroup
		var tests bool
		result := make(chan bool, 1)
		defer close(result)

		for i := range testsAttempt {
			tests = true
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

			log.Printf("%d попытка прохождения теста с COM-портами\n", i+1)
			for rPort, wPort := range pairs {
				log.Printf("Тестирование пары rPort %s, wPort %s\n", rPort, wPort)
				wg.Add(2)
				go func(r string) {
					defer wg.Done()
					// b := portRead(ctx, r, result)
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
	}

	var testsAttempt = 3

	// for i := range testsAttempt {

	// }

	var wg sync.WaitGroup
	result := make(chan bool, 1)
	defer close(result)

	var tests bool
	// testsAttempt := 3

	for i := range testsAttempt {
		tests = true
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

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
