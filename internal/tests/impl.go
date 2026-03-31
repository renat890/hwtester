package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/diskfs/go-diskfs"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
	"go.bug.st/serial"
)

var ErrSensorsNotFound = errors.New("датчики температуры coretemp не найдены")

type output struct {
	BlockDevices []device `json:"blockdevices"`
}

type device struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type diskSpeed struct {
	name string
	write float64
	read float64
}

func bytesToMBytes(bytes uint64) int {
	return int(float64(bytes) / math.Pow(2, 20) * 1.049)
}

type HardwareUsage struct{}

func (h *HardwareUsage) GetMemory() (int, error) {
	ram, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return bytesToMBytes(ram.Total), nil

}

func (h *HardwareUsage) Info() ([]DiskInfo, error) {
	const nvme = "lsblk -d -b -n -N -o NAME,SIZE -J"
	const scsi = "lsblk -d -b -n -S -o NAME,SIZE -J"
	const virt = "lsblk -d -b -n -v -o NAME,SIZE -J"

	disks := []DiskInfo{}

	for _, cmd := range []string{nvme, scsi, virt} {
		args := strings.Split(cmd, " ")
		cmdC := args[0]
		cmdA := args[1:]

		cmd := exec.Command(cmdC, cmdA...)
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		var o output
		err = json.Unmarshal(out, &o)
		if err != nil {
			return nil, err
		}

		for _, info := range o.BlockDevices {
			disks = append(disks, DiskInfo{
				Name:     info.Name,
				VolumeMB: bytesToMBytes(info.Size),
			})
		}

	}

	ch := make(chan diskSpeed, len(disks))
	chErr := make(chan error, len(disks))
	var wg sync.WaitGroup
	wg.Add(len(disks))

	for _, disk := range disks {
		go func(name string) {
			defer wg.Done()
			speeds, err := h.checkSpeedDisks(name)
			if err != nil {
				chErr <- err
				return 
			}
			ch <- speeds
		}(disk.Name)
	}

	wg.Wait()
	close(ch)

	select {
	case err := <- chErr:
		return nil, err
	default:
	}
	close(chErr)

	speedResults := []diskSpeed{}
	for val := range ch {
		speedResults = append(speedResults, val)
	}

	for i, disk := range disks {
		for _, speedI := range speedResults {
			if speedI.name == disk.Name {
				disks[i].ReadMBPerSec = int(speedI.read)
				disks[i].WriteMBPerSec = int(speedI.write)
			}
		}
	}

	return disks, nil
}

func (h *HardwareUsage) checkSpeedDisks(name string) (diskSpeed, error) {
	rdName := "/dev/" + name
	fd, err := syscall.Open(rdName, syscall.O_RDONLY|syscall.O_DIRECT, 0)
	if err != nil {
		return diskSpeed{}, err
	}
	defer syscall.Close(fd)
	buf := make([]byte, 32*1024)

	sum := 0
	start := time.Now()
	for {
		if sum >= 512_000_000 {
			break
		}
		n, err := syscall.Read(fd, buf)
		if err != nil {
			return diskSpeed{} ,err
		}
		sum += n
	}
	end := time.Since(start).Nanoseconds()
	readSpeed := toMbPerSec(sum, end)
	
	wrtName := rdName 
	const (
		nvmeSuf = "p77"
		scsiSuf = "77"
	)
	if strings.HasPrefix(name, "nvme") {
		wrtName += nvmeSuf
	} else {
		wrtName += scsiSuf
	}
	var writeSpeed float64
	fdWrt, err := syscall.Open(wrtName, syscall.O_WRONLY|syscall.O_DIRECT, 0)
	if err != nil {
		writeSpeed = 0.0
	} else {
		defer syscall.Close(fdWrt)
		sum = 0
		start = time.Now()
		for {
			if sum >= 512_000_000 {
				break
			}
			n, err := syscall.Write(fdWrt, buf)
			if err != nil {
				return diskSpeed{} ,err
			}
			sum += n
		}
		end = time.Since(start).Nanoseconds()
		writeSpeed = toMbPerSec(sum, end)
	}
	

	return diskSpeed{write: writeSpeed, read: readSpeed, name: name}, nil
}

func toMbPerSec(bytes int, nanoSec int64) float64 {
	mb := float64(bytes) / 1_000_000
	sec := float64(nanoSec) / 1_000_000_000
	return mb / sec
}

func (h *HardwareUsage) Load(ctx context.Context, dur time.Duration) (CpuInfo, error) {
	ch := make(chan []sensors.TemperatureStat)

	infoT, err := sensors.SensorsTemperatures()
	if err != nil {
		return CpuInfo{}, err
	}
	startTmp, err := getCoreTemp(infoT)
	if err != nil {
		return CpuInfo{}, err
	}

	ctx1, cancel := context.WithTimeout(ctx, dur)
	defer cancel()

	freq := 5 * time.Second
	go getTemperature(ctx1, freq, ch)

	for num := range runtime.NumCPU() {
		go load(ctx1, num)
	}

	temperatures := []float64{}

	for val := range ch {
		temp, err := getCoreTemp(val)
		if err != nil {
			continue
		}
		temperatures = append(temperatures, temp)
	}
	
	maxT := 0.0
	if len(temperatures) > 0 {
		maxT = slices.Max(temperatures)
	}
	
	var lastTempCore float64
	var avgTemp float64
	lastTemp, err := sensors.SensorsTemperatures()
	if err == nil {
		lastTempCore, err = getCoreTemp(lastTemp)
		if err != nil {
			lastTempCore = 0.0
		}
		avgTemp, err = getAvgTemp(lastTemp)
		if err != nil {
			avgTemp = 0.0
		}
	}

	return CpuInfo{
		MaxTempCore: maxT,
		AvgTemp: avgTemp,
		EndTemp: lastTempCore,
		StartTemp: startTmp,
		Name: getCpuName(),
	}, nil
}

func getCpuName() string {
	info, err := cpu.Info()
	if err != nil || len(info) == 0 {
		return "unknown"
	}
	return info[0].ModelName
}

func load(ctx context.Context, worker int) {
	log.Printf("Запущен worker с номером - %d\n", worker)
	var i int64
	workLoad:
	for {
		select {
		case <- ctx.Done():
			break workLoad
		default:
			i++
			if i == 100_000 {
				i = 0
			}
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

const corePattern = "coretemp"

func getCoreTemp(info []sensors.TemperatureStat) (float64, error) {
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

	return maxTemp, nil
}

func getAvgTemp(info []sensors.TemperatureStat) (float64, error) {
	if len(info) == 0 {
		return 0, fmt.Errorf("пустой список")
	}

	sumTemps := 0.0
	numTempls := 0
	for _, temp := range info {
		if !strings.Contains(temp.SensorKey, corePattern) {
			continue
		}
		sumTemps += temp.Temperature
		numTempls++
	}

	if numTempls == 0 {
		return 0, fmt.Errorf("нет датчиков температуры ядра")
	}

	return sumTemps / float64(numTempls), nil
}

// для COM портов
const msg = "test com this big message and very big message abracodabra stop"

func (h *HardwareUsage) EchoTest(ctx context.Context) (COMInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return COMInfo{}, errors.New("Не удалось получить COM порты")
	}
	for _, port := range ports {
		log.Printf("Найден порт: %v\n", port)
	}
	if len(ports) < 2 {
		return COMInfo{}, errors.New("нет минимального количества портов COM")
	}
	// Обусловлено выбором /dev/ttyS0 и /dev/ttyS1
	pairs := map[string]string{
		ports[0]: ports[1],
		ports[1]: ports[0],
	}

	var wg sync.WaitGroup
	result := make(chan bool, 1)
	defer close(result)

	var tests bool
	testsAttempt := 3

	for i := range testsAttempt {
		tests = true
		ctx, cancel := context.WithTimeout(ctx, 2 * time.Second)

		log.Printf("%d попытка прохождения теста с COM-портами\n", i+1)
		for rPort, wPort := range pairs {
			log.Printf("Тестирование пары rPort %s, wPort %s\n", rPort, wPort)
			wg.Add(2)
			go func(r string)  {
				defer wg.Done()
				portRead(ctx, r, result)
			}(rPort)
			go func(w string) {
				defer wg.Done()
				portWrite(ctx, w)
			}(wPort) 
			wg.Wait()

			res, ok := <- result
			if !ok || !res {
				tests = false
			}
		}
		cancel()
		if tests {
			break
		} else {
			log.Println("Неудачная попытка прохождения теста. Перезапускаю тест.")
		}
	}

	final := COMInfo{Result: false, TestPorts: ports}
	if tests {
		final.Result = true
	} 
	return final, nil
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

	if err := port.SetReadTimeout(500 * time.Millisecond); err !=  nil {
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
		case <- ctx.Done():
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

// для работы с флешками
func (h *HardwareUsage) GetUSBInfo(ctx context.Context) (USBInfo, error) {
	// TODO: вынести в конфигурационный файл размер флешки
	const sizeInMB int = 8000
	disks, err := scsiDevices()
	// TODO: добавить детальное описание ошибок
	if err != nil {
		return USBInfo{}, err
	}
	diskNames := make([]string, 0)
	for _, val := range disks {
		isValidSize := math.Abs(float64(sizeInMB - val.VolumeMB)) < float64(sizeInMB / 100 * 2)
		if isValidSize {
			name := "/dev/" + strings.TrimSpace(val.Name)
			diskNames = append(diskNames, name)
		}
	}

	const testFileName = "test.txt"
	actual := 0
	for _, diskName := range diskNames {
		disk, err := diskfs.Open(diskName)
		if err != nil {
			log.Println(err)
			continue
		}

		fs, err := disk.GetFilesystem(1)
		if err != nil {
			log.Println(err)
			disk.Close()
			continue
		}

		b, err := fs.ReadFile(testFileName)
		if err != nil {
			log.Println("Не удалось прочитать файл")
			disk.Close()
			continue
		}
		// TODO: вынести в конфигурационный файл ожидаемое содержимое
		const expectedString = "is opened testing file"
		// убираем спецсимвол в конце строки
		a,_,_ := strings.Cut(string(b), "\n")
		if a == expectedString {
			actual++
		}

		if err = disk.Close(); err != nil {
			log.Printf("ошибка закрытия флешки: %s\n", err.Error())
		}
	}

	if len(diskNames) != actual {
		return USBInfo{Result: false}, nil
	}

	return USBInfo{Result: true}, nil
}

func scsiDevices() ([]DiskInfo, error) {
	
	cmd := exec.Command("lsblk", strings.Split("-d -b -n -S -o NAME,SIZE -J", " ")...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var o output
	err = json.Unmarshal(out, &o)
	if err != nil {
		return nil, err
	}
	disks := make([]DiskInfo, 0, 4)

	for _, info := range o.BlockDevices {
		disks = append(disks, DiskInfo{
			Name:     info.Name,
			VolumeMB: bytesToMBytes(info.Size),
		})
	}

	return disks, nil
}