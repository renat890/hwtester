package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"strings"

	"github.com/diskfs/go-diskfs"
)

func main() {
	// Получить все устройства, размер которых задан в конфиге
	// у меня флешка 8 гб
	const sizeInMB int = 8000
	disks, err := scsiDevices()
	if err != nil {
		log.Panic(err)
	}
	diskNames := make([]string, 0)
	for _, val := range disks {
		isValidSize := math.Abs(float64(sizeInMB - val.VolumeMB)) < float64(sizeInMB / 100 * 2)
		if isValidSize {
			name := "/dev/" + strings.TrimSpace(val.Name)
			diskNames = append(diskNames, name)
		}
	}

	for _, val := range diskNames {
		fmt.Println(val)
	}

	// для каждого устройства пройтись по файловой системе
	// открыть файл, заданный в конфиге
	// Прочитать содержимое, которое либо удовлетворяет заранее созданной константе, либо в конфиге определено содержимое файла
	const testFileName = "test.txt"
	actual := 0
	for _, diskName := range diskNames {
		disk, err := diskfs.Open(diskName)
		if err != nil {
			log.Panic(err)
		}


		fs, err := disk.GetFilesystem(1)
		if err != nil {
			log.Panic(err)
		}

		b, err := fs.ReadFile(testFileName)
		if err != nil {
			log.Println("Не удалось прочитать файл")
		}
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

	// ТЕст пройден, если количество подключенных флешек равно количеству верно прочитанных файлов с флешек
	if len(diskNames) == actual {
		log.Println(true)
	} else {
		log.Println(false)
	}
}

type output struct {
	BlockDevices []device `json:"blockdevices"`
}

type device struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
}

type DiskInfo struct {
	Name          string
	VolumeMB      int
	WriteMBPerSec int
	ReadMBPerSec  int
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

func bytesToMBytes(bytes uint64) int {
	return int(float64(bytes) / math.Pow(2, 20) * 1.049)
}