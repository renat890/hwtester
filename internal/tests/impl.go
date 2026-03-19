package tests

import (
	"encoding/json"
	"math"
	"os/exec"
	"strings"

	"github.com/shirou/gopsutil/v4/mem"
)

type output struct {
	BlockDevices []device `json:"blockdevices"`
}

type device struct {
	Name string `json:"name"`
	Size uint64 `json:"size"`
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

	return disks, nil
}
