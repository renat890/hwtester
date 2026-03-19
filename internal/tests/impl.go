package tests

import (
	"encoding/json"
	"math"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v4/mem"
)

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