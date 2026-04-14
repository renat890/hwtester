package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"math"
	"net"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"text/template"
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
	name  string
	write float64
	read  float64
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

func (h *HardwareUsage) Info(logCh chan hw.LogMsg) ([]DiskInfo, error) {
	// const nvme = "lsblk -d -b -n -N -o NAME,SIZE -J"
	const scsi = "lsblk -d -b -n -S -o NAME,SIZE -J"
	// const virt = "lsblk -d -b -n -v -o NAME,SIZE -J"
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  "Начаты тесты дисков",
		Stamp: time.Now(),
	}
	disks := []DiskInfo{}

	// nvme - не все OS поддерживают nvme
	// virtIO - точно также
	for _, cmd := range []string{scsi} {
		args := strings.Split(cmd, " ")
		cmdC := args[0]
		cmdA := args[1:]

		cmd := exec.Command(cmdC, cmdA...)
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("Не удалось выполнить команду для получения дисков %v", err)
		}

		var o output
		err = json.Unmarshal(out, &o)
		if err != nil {
			return nil, fmt.Errorf("Не удалось преобразовать данные в структуры дисков %v", err)
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
			logCh <- hw.LogMsg{
				Level: hw.INFO,
				Text:  fmt.Sprintf("Измерение скорости диска %s", disk.Name),
				Stamp: time.Now(),
			}
			speeds, err := h.checkSpeedDisks(name)
			if err != nil {
				chErr <- fmt.Errorf("Ошибка при проверки скорости диска %v", err)
				return
			}
			ch <- speeds
		}(disk.Name)
	}

	wg.Wait()
	close(ch)

	select {
	case err := <-chErr:
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

	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  "Закончены тесты дисков",
		Stamp: time.Now(),
	}

	return disks, nil
}

func (h *HardwareUsage) checkSpeedDisks(name string) (diskSpeed, error) {
	rdName := "/dev/" + name
	fd, err := syscall.Open(rdName, syscall.O_RDONLY|syscall.O_SYNC, 0)
	if err != nil {
		return diskSpeed{}, fmt.Errorf("Не удалось открыть дескриптор диска для чтения %v", err)
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
			return diskSpeed{}, fmt.Errorf("Не удалось проверить запись в сетевой дескриптор для чтения %v", err)
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
				return diskSpeed{}, fmt.Errorf("Не удалось проверить скорость записи в диск %v", err)
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

func (h *HardwareUsage) Load(ctx context.Context, dur time.Duration, logCh chan hw.LogMsg) (CpuInfo, error) {
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
	go getTemperature(ctx1, freq, ch, logCh)

	for num := range runtime.NumCPU() {
		go load(ctx1, num, logCh)
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
		AvgTemp:     avgTemp,
		EndTemp:     lastTempCore,
		StartTemp:   startTmp,
		Name:        getCpuName(),
	}, nil
}

func getCpuName() string {
	info, err := cpu.Info()
	if err != nil || len(info) == 0 {
		return "unknown"
	}
	return info[0].ModelName
}

func load(ctx context.Context, worker int, logCh chan hw.LogMsg) {
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  fmt.Sprintf("Запущен worker с номером - %d", worker),
		Stamp: time.Now(),
	}

	var i int64
workLoad:
	for {
		select {
		case <-ctx.Done():
			break workLoad
		default:
			i++
			if i == 100_000 {
				i = 0
			}
		}
	}
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  fmt.Sprintf("Остановлен worker с номером - %d", worker),
		Stamp: time.Now(),
	}
}

func getTemperature(ctx context.Context, freq time.Duration, ch chan []sensors.TemperatureStat, logCh chan hw.LogMsg) {
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  "Запущен сборщик показаний температуры",
		Stamp: time.Now(),
	}
	attempt := 1
getter:
	for {
		select {
		case <-ctx.Done():
			close(ch)
			break getter
		default:
			info, err := sensors.SensorsTemperatures()
			if err != nil {
				logCh <- hw.LogMsg{
					Level: hw.WARN,
					Text:  fmt.Sprintf("ошибка опроса датчика температуры, попытка №%d", attempt),
					Stamp: time.Now(),
				}
				attempt++
				continue
			}
			ch <- info
		}
		time.Sleep(freq)
	}
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  "Остановлен сборщик показаний температуры",
		Stamp: time.Now(),
	}
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

func (h *HardwareUsage) EchoTest(ctx context.Context, logCh chan hw.LogMsg) (COMInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return COMInfo{}, errors.New("Не удалось получить COM порты")
	}
	for _, port := range ports {
		logCh <- hw.LogMsg{
			Level: hw.INFO,
			Text:  fmt.Sprintf("Найден порт: %v", port),
			Stamp: time.Now(),
		}
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
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)

		logCh <- hw.LogMsg{
			Level: hw.INFO,
			Text:  fmt.Sprintf("%d попытка прохождения теста с COM-портами", i+1),
			Stamp: time.Now(),
		}

		for rPort, wPort := range pairs {
			logCh <- hw.LogMsg{
				Level: hw.INFO,
				Text:  fmt.Sprintf("Тестирование пары rPort %s, wPort %s", rPort, wPort),
				Stamp: time.Now(),
			}

			wg.Add(2)
			go func(r string) {
				defer wg.Done()
				portRead(ctx, r, result, logCh)
			}(rPort)
			go func(w string) {
				defer wg.Done()
				portWrite(ctx, w, logCh)
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
			logCh <- hw.LogMsg{
				Level: hw.WARN,
				Text:  "Неудачная попытка прохождения теста. Перезапускаю тест.",
				Stamp: time.Now(),
			}
		}
	}

	final := COMInfo{Result: false, TestPorts: ports}
	if tests {
		final.Result = true
	}
	return final, nil
}

func portRead(ctx context.Context, name string, ch chan bool, logCh chan hw.LogMsg) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  err.Error(),
			Stamp: time.Now(),
		}
		ch <- false
		return
	}
	defer port.Close()

	if err := port.SetReadTimeout(500 * time.Millisecond); err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  err.Error(),
			Stamp: time.Now(),
		}
		ch <- false
		return
	}

	if err := port.ResetInputBuffer(); err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  err.Error(),
			Stamp: time.Now(),
		}
		ch <- false
		return
	}

	var final strings.Builder
	buf := make([]byte, 8)
	numMsg := 1
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  fmt.Sprintf("Запущен порт читатель rPort %s", name),
		Stamp: time.Now(),
	}
reader:
	for {
		select {
		case <-ctx.Done():
			ch <- false
			return
		default:
			n, err := port.Read(buf)
			if err != nil {
				logCh <- hw.LogMsg{
					Level: hw.WARN,
					Text:  fmt.Sprintf("Ошибка чтения из COM-порта: %s", err.Error()),
					Stamp: time.Now(),
				}
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

func portWrite(ctx context.Context, name string, logCh chan hw.LogMsg) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  err.Error(),
			Stamp: time.Now(),
		}
		return
	}
	defer port.Close()
	logCh <- hw.LogMsg{
		Level: hw.INFO,
		Text:  fmt.Sprintf("Запущен порт писатель wPort %s", name),
		Stamp: time.Now(),
	}
	if ctx.Err() != nil {
		return
	}
	_, err = port.Write(([]byte(msg)))
	if err != nil {
		logCh <- hw.LogMsg{
			Level: hw.WARN,
			Text:  fmt.Sprintf("Ошибка записи в COM-порт: %s", err.Error()),
			Stamp: time.Now(),
		}
	}
}

// для работы с флешками
func (h *HardwareUsage) GetUSBInfo(ctx context.Context, logCh chan hw.LogMsg) (USBInfo, error) {
	// TODO: вынести в конфигурационный файл размер флешки
	const sizeInMB int = 8000
	disks, err := scsiDevices()
	// TODO: добавить детальное описание ошибок
	if err != nil {
		return USBInfo{}, err
	}
	diskNames := make([]string, 0)
	for _, val := range disks {
		isValidSize := math.Abs(float64(sizeInMB-val.VolumeMB)) < float64(sizeInMB/100*2)
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
			logCh <- hw.LogMsg{
				Level: hw.ERR,
				Text:  err.Error(),
				Stamp: time.Now(),
			}
			continue
		}

		fs, err := disk.GetFilesystem(1)
		if err != nil {
			logCh <- hw.LogMsg{
				Level: hw.ERR,
				Text:  err.Error(),
				Stamp: time.Now(),
			}
			disk.Close()
			continue
		}

		b, err := fs.ReadFile(testFileName)
		if err != nil {
			logCh <- hw.LogMsg{
				Level: hw.ERR,
				Text:  "Не удалось прочитать файл",
				Stamp: time.Now(),
			}
			disk.Close()
			continue
		}
		// TODO: вынести в конфигурационный файл ожидаемое содержимое
		const expectedString = "is opened testing file"
		// убираем спецсимвол в конце строки
		a, _, _ := strings.Cut(string(b), "\n")
		if a == expectedString {
			actual++
		}

		if err = disk.Close(); err != nil {
			logCh <- hw.LogMsg{
				Level: hw.ERR,
				Text:  fmt.Sprintf("ошибка закрытия флешки: %s", err.Error()),
				Stamp: time.Now(),
			}
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

// тест сетевых портов

const nameNetNamespace = "testns"

func loadTemplates() (map[string]*template.Template, error) {
	result := map[string]*template.Template{}
	patterns := map[string]string{
		"preTest":     "ip netns add {{.NS}}",
		"postTest":    "ip netns delete {{.NS}}",
		"preEachEth":  `sh -c "ip link set {{.Eth}} netns {{.NS}} && ip netns exec {{.NS}} ip addr add {{.IP}}/24 dev {{.Eth}} && ip netns exec {{.NS}} ip link set {{.Eth}} up"`,
		"postEachEth": "ip netns exec {{.NS}} ip link set {{.Eth}} netns 1",
	}

	for name, pattern := range patterns {
		tmpl, err := template.New(name).Parse(pattern)
		if err != nil {
			return nil, err
		}
		result[name] = tmpl
	}

	return result, nil
}

func splitCmd(cmd string) (string, []string) {
	cmds := strings.Split(cmd, " ")
	return cmds[0], cmds[1:]
}

func runCmd(cmd string) error {
	cmdC, cmdA := splitCmd(cmd)
	_, err := exec.Command(cmdC, cmdA...).Output()
	if err != nil {
		exErr, ok := err.(*exec.ExitError)
		if !ok {
			return err
		}
		return fmt.Errorf("ошибка выполнения команды %v", string(exErr.Stderr))
	}
	return nil
}

func (h *HardwareUsage) GetEthernetsInfo(ctx context.Context, eths []config.Ethernet, logCh chan hw.LogMsg) (PortsInfo, error) {
	portsInfoRes := PortsInfo{}
	// Загружаю все возможные шаблоны
	tmpls, err := loadTemplates()
	if err != nil {
		return PortsInfo{}, err
	}
	type args struct {
		NS  string
		IP  string
		Eth string
	}
	var cmd bytes.Buffer

	// выполняю действия перед тестами сетевых интерфейсов
	err = tmpls["preTest"].Execute(&cmd, args{NS: nameNetNamespace})
	if err != nil {
		return PortsInfo{}, fmt.Errorf("ошибка создания шаблона пре-теста %v", err)
	}
	if err = runCmd(cmd.String()); err != nil {
		return PortsInfo{}, fmt.Errorf("ошибка выполнения пре-теста %v %s", err, cmd.String())
	}

	// создание пар портов
	if len(eths) != 4 {
		return PortsInfo{}, errors.New("количество портов не равно 4")
	}
	pairs := map[config.Ethernet]config.Ethernet{
		eths[0]: eths[1],
		eths[1]: eths[0],
		eths[2]: eths[3],
		eths[3]: eths[2],
	}

	// их перебор
	result := make(chan int)

	for client, server := range pairs {
		func() {
			// перед запуском тестов на порту для каждого клиента
			cmd.Reset()
			if err = tmpls["preEachEth"].Execute(&cmd, args{NS: nameNetNamespace, Eth: client.Name, IP: client.Ip}); err != nil {
				logCh <- hw.LogMsg{
					Level: hw.ERR,
					Text:  err.Error(),
					Stamp: time.Now(),
				}
				return
			}
			if err = runCmd(cmd.String()); err != nil {
				logCh <- hw.LogMsg{
					Level: hw.ERR,
					Text:  err.Error(),
					Stamp: time.Now(),
				}
				return
			}
			// в конце каждого теста
			defer func() {
				cmd.Reset()
				if err = tmpls["postEachEth"].Execute(&cmd, args{NS: nameNetNamespace, Eth: client.Name}); err != nil {
					logCh <- hw.LogMsg{
						Level: hw.ERR,
						Text:  err.Error(),
						Stamp: time.Now(),
					}
					return
				}
				if err = runCmd(cmd.String()); err != nil {
					logCh <- hw.LogMsg{
						Level: hw.ERR,
						Text:  err.Error(),
						Stamp: time.Now(),
					}
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// сам тест портов
			go listen(ctx, result, server.Ip, server.Port, logCh)
			time.Sleep(500 * time.Millisecond)

			cmd.Reset()
			// TODO: законфижить путь до бинарника отрпавителя
			if err = runCmd(fmt.Sprintf("ip netns exec %s ./eth-util --mode=client --ip=%s", nameNetNamespace, server.Ip)); err != nil {
				logCh <- hw.LogMsg{
					Level: hw.ERR,
					Text:  err.Error(),
					Stamp: time.Now(),
				}
			}

			actual := <-result
			if actual != expectedCount {
				logCh <- hw.LogMsg{
					Level: hw.ERR,
					Text:  fmt.Sprintf("Для пары тест сетевых портов провален. Получено пакетов: %d", actual),
				}
				portsInfoRes.PacketsLoss += (expectedCount - actual)
			}
			portsInfoRes.Ports = append(portsInfoRes.Ports, server.Name)
		}()
	}

	// выполняю действия после тестов сетевых интерфейсов
	cmd.Reset()
	err = tmpls["postTest"].Execute(&cmd, args{NS: nameNetNamespace})
	if err != nil {
		return PortsInfo{}, fmt.Errorf("ошибка создания шаблона пост-тестовых действий %v", err)
	}
	if err = runCmd(cmd.String()); err != nil {
		return PortsInfo{}, fmt.Errorf("ошибка выполнения пост-тестовых действий %v", err)
	}

	return portsInfoRes, nil
}

const (
	expectedMsg   = "expected message"
	expectedCount = 1_000
)

func listen(ctx context.Context, result chan int, ip, port string, logCh chan hw.LogMsg) {
	address := fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  err.Error(),
			Stamp: time.Now(),
		}
		result <- 0
		return
	}
	defer conn.Close()
	if err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		logCh <- hw.LogMsg{
			Level: hw.ERR,
			Text:  "Не удалось установить дедлайн на чтение",
			Stamp: time.Now(),
		}
		result <- 0
		return
	}

	count := 0

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	for {
		packet := make([]byte, 1024)
		n, _, err := conn.ReadFrom(packet)
		if err != nil {
			break
		}
		if string(packet[:n]) == expectedMsg {
			count++
		}
		if count == expectedCount {
			break
		}
	}

	result <- count
}
