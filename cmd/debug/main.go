package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/sensors"
)

var ErrSensorsNotFound = errors.New("датчики температуры coretemp не найдены")

func main() {
	dur := 60 * time.Second
	maxTemp := 85.0
	gradient := 40.0
	freq := 5 * time.Second
	ch := make(chan []sensors.TemperatureStat)

	log.Println("Запущен нагружатор")
	infoT, err := sensors.SensorsTemperatures()
	if err != nil {
		log.Fatal(err.Error())
	}
	startTmp, err := getCoreTemp(infoT)
	if err != nil {
		log.Fatal(err.Error())
	}
	log.Printf("Стартовая температура: %f", startTmp)
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	go getTemperature(ctx, freq, ch)

	for num := range runtime.NumCPU() {
		go load(ctx, num)
	}

	temperatures := []float64{}

	for val := range ch {
		temp, err := getCoreTemp(val)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("температура в текущий момент: %f", temp)
		temperatures = append(temperatures, temp)
	}
	
	maxT := slices.Max(temperatures)

	fmt.Printf("максимальная температура: %f\n", maxT)
	fmt.Printf("Превышен порог: %t\n", maxT > maxTemp)
	fmt.Printf("Превышен градиент: %t\n", (maxT - startTmp) > gradient)
}


func load(ctx context.Context, worker int) {
	log.Printf("Запущен worker с номером - %d\n", worker)
	workLoad:
	for {
		select {
		case <- ctx.Done():
			break workLoad
		default:
			i := 1
			_ = i + 1
		}
	}
	log.Printf("Остановлен worker с номером - %d", worker)
}

func getTemperature(ctx context.Context, freq time.Duration, ch chan []sensors.TemperatureStat) {
	log.Println("Запущен сборщик показаний температуры")
	attempt := 1
	getter:
	for {
		select {
		case <- ctx.Done():
			close(ch)
			break getter
		default:
			info, err := sensors.SensorsTemperatures()
			if err != nil {
				log.Printf("ошибка опроса датчика температуры, попытка №%d\n", attempt)
				attempt++
				continue
			} 
			ch <- info 
		}
		time.Sleep(freq)
	}
	log.Println("Остановлен сборщик показаний температуры")
}

func getCoreTemp(info []sensors.TemperatureStat) (float64, error) {
	const corePattern = "coretemp"
	if len(info) == 0 {
		return 0, fmt.Errorf("пустой список")
	}
	maxTemp := 0.0
	for _, temp := range info {
		if !strings.Contains(temp.SensorKey, corePattern) {
			continue
		}
		if temp.Temperature > maxTemp {
			maxTemp = temp.Temperature
		}
	}

	if maxTemp == 0 {
		return 0, ErrSensorsNotFound
	}

	// перевод в Цельсий, так как температура в фаренгейтах
	return farToCels(maxTemp), nil
}

func farToCels(farTemp float64) float64 {
	return (farTemp - 32) * (5.0 / 9.0)
}