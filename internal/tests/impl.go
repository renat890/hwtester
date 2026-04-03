package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"factorytest/internal/config"
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

func (h *HardwareUsage) Load(ctx context.Context, dur time.Duration, logCh chan string) (CpuInfo, error) {
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

func load(ctx context.Context, worker int, logCh chan string) {
	logCh <- fmt.Sprintf("Запущен worker с номером - %d\n", worker)
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
	logCh <- fmt.Sprintf("Остановлен worker с номером - %d\n", worker)
}

func getTemperature(ctx context.Context, freq time.Duration, ch chan []sensors.TemperatureStat, logCh chan string) {
	logCh <- fmt.Sprintln("Запущен сборщик показаний температуры")
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
				logCh <- fmt.Sprintf("ошибка опроса датчика температуры, попытка №%d\n", attempt)
				attempt++
				continue
			} 
			ch <- info 
		}
		time.Sleep(freq)
	}
	logCh <- fmt.Sprintln("Остановлен сборщик показаний температуры")
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

func (h *HardwareUsage) EchoTest(ctx context.Context, logCh chan string) (COMInfo, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return COMInfo{}, errors.New("Не удалось получить COM порты")
	}
	for _, port := range ports {
		logCh <- fmt.Sprintf("Найден порт: %v\n", port)
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

		logCh <- fmt.Sprintf("%d попытка прохождения теста с COM-портами\n", i+1)
		for rPort, wPort := range pairs {
			logCh <- fmt.Sprintf("Тестирование пары rPort %s, wPort %s\n", rPort, wPort)
			wg.Add(2)
			go func(r string)  {
				defer wg.Done()
				portRead(ctx, r, result, logCh)
			}(rPort)
			go func(w string) {
				defer wg.Done()
				portWrite(ctx, w, logCh)
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
			logCh <- fmt.Sprintln("Неудачная попытка прохождения теста. Перезапускаю тест.")
		}
	}

	final := COMInfo{Result: false, TestPorts: ports}
	if tests {
		final.Result = true
	} 
	return final, nil
}

func portRead(ctx context.Context, name string, ch chan bool, logCh chan string) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		logCh <- fmt.Sprintln(err)
		ch <- false
		return
	}
	defer port.Close()

	if err := port.SetReadTimeout(500 * time.Millisecond); err !=  nil {
		logCh <- fmt.Sprintln(err)
		ch <- false
		return
	}

	if err := port.ResetInputBuffer(); err != nil {
		logCh <- fmt.Sprintln(err)
		ch <- false
		return
	}

	var final strings.Builder
	buf := make([]byte, 8)
	numMsg := 1 
	logCh <- fmt.Sprintf("Запущен порт читатель rPort %s\n", name)
	reader:
	for {
		select {
		case <- ctx.Done():
			ch <- false
			return
		default:
			n, err := port.Read(buf)
			if err != nil {
				logCh <- fmt.Sprintf("Ошибка чтения из COM-порта: %s\n", err.Error())
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

func portWrite(ctx context.Context, name string, logCh chan string) {
	port, err := serial.Open(name, &serial.Mode{
		BaudRate: 115200,
	})
	if err != nil {
		logCh <- fmt.Sprintln(err)
		return
	}
	defer port.Close()
	logCh <- fmt.Sprintf("Запущен порт писатель wPort %s\n", name)
	if ctx.Err() != nil {
		return
	}
	_, err = port.Write(([]byte(msg)))
	if err != nil {
		logCh <- fmt.Sprintf("Ошибка записи в COM-порт: %s\n", err.Error())
	}
}

// для работы с флешками
func (h *HardwareUsage) GetUSBInfo(ctx context.Context, logCh chan string) (USBInfo, error) {
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
			logCh <- fmt.Sprintln(err)
			continue
		}

		fs, err := disk.GetFilesystem(1)
		if err != nil {
			logCh <- fmt.Sprintln(err)
			disk.Close()
			continue
		}

		b, err := fs.ReadFile(testFileName)
		if err != nil {
			logCh <- fmt.Sprintln("Не удалось прочитать файл")
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
			logCh <- fmt.Sprintf("ошибка закрытия флешки: %s\n", err.Error())
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
		"preTest": "ip netns add {{.ns}}",
		"postTest": "ip netns delete {{.ns}}",
		"preEachEth": `sh -c "ip link set {{.eth}} netns {{.ns}} && ip netns exec {{.ns}} ip addr add {{.ip}}/24 dev {{.eth}} && ip netns exec {{.ns}} ip link set {{.eth}} up"`,
		"postEachEth": "ip netns exec {{.ns}} ip link set {{.eth}} netns 1",
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
	return err
}

func (h *HardwareUsage) GetEthernetsInfo(ctx context.Context, eths []config.Ethernet, logCh chan string) (PortsInfo, error) {
	portsInfoRes := PortsInfo{}
	// Загружаю все возможные шаблоны
	tmpls, err := loadTemplates()
	if err != nil {
		return PortsInfo{}, err
	}
	type args struct {
		ns string
		ip string
		eth string
	}
	var cmd bytes.Buffer
	
	// выполняю действия перед тестами сетевых интерфейсов
	err = tmpls["preTest"].Execute(&cmd, args{ns: nameNetNamespace})
	if err != nil {
		return PortsInfo{}, err
	}
	if err = runCmd(cmd.String()); err != nil {
		return PortsInfo{}, err
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
		func ()  {
			// перед запуском тестов на порту для каждого клиента
			cmd.Reset()
			if err = tmpls["preEachEth"].Execute(&cmd, args{ns: nameNetNamespace, eth: client.Name, ip: client.Ip}); err != nil {
				logCh <- fmt.Sprintln(err)
				return 
			}
			if err = runCmd(cmd.String()); err != nil {
				logCh <- fmt.Sprintln(err)
				return 
			}
			// в конце каждого теста
			defer func ()  {
				cmd.Reset()
				if err = tmpls["postEachEth"].Execute(&cmd, args{ns: nameNetNamespace, eth: client.Name}); err != nil {
					logCh <- fmt.Sprintln(err)
					return  
				}
				if err = runCmd(cmd.String()); err != nil {
					logCh <- fmt.Sprintln(err)
				}
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
			defer cancel()

			// сам тест портов
			go listen(ctx, result, server.Ip, server.Port, logCh)
			time.Sleep(500 * time.Millisecond)

			cmd.Reset()
			// TODO: законфижить путь до бинарника отрпавителя
			if err = runCmd(fmt.Sprintf("ip netns exec %s ./eth-util --mode=client --ip=%s", nameNetNamespace, server.Ip)); err != nil {
				logCh <- fmt.Sprintln(err)
			}

			actual := <- result
			if actual != expectedCount {
				logCh <- fmt.Sprintln("Для пары тест сетевых портов провален")
				logCh <- fmt.Sprintln("Получено пакетов: ", actual)
				portsInfoRes.PacketsLoss += (expectedCount - actual)
			}
			portsInfoRes.Ports = append(portsInfoRes.Ports, server.Name)
		}()
		
		
		
	}

	// выполняю действия после тестов сетевых интерфейсов
	cmd.Reset()
	err = tmpls["postTest"].Execute(&cmd, args{ns: nameNetNamespace})
	if err != nil {
		return PortsInfo{}, err
	}
	if err = runCmd(cmd.String()); err != nil {
		return PortsInfo{}, err
	}

	return portsInfoRes, nil
}

const (
	expectedMsg = "expected message"
	expectedCount = 1_000
)

func listen(ctx context.Context, result chan int, ip, port string, logCh chan string) {
	address := fmt.Sprintf("%s:%s", ip, port)
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		logCh <- fmt.Sprintln(err)
		result <- 0
		return
	}
	defer conn.Close()
	if err = conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		logCh <- fmt.Sprintln("Не удалось установить дедлайн на чтение")
		result <- 0
		return
	}

	count := 0

	go func ()  {
		<- ctx.Done()
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