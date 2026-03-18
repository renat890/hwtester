package main

import (
	"encoding/json"
	"factorytest/internal/tests"
	"log"
	"math"
	"os/exec"
	"strings"
)

type output struct {
	BlockDevices []device `json:"blockdevices"`
}

type device struct {
	Name string `json:"name"`
	Size uint64    `json:"size"`

}

func bytesToMBytes(bytes uint64) int {
	return int(float64(bytes) / math.Pow(2, 20) * 1.049)
}

func main() {
	const nvme = "lsblk -d -b -n -N -o NAME,SIZE -J"
	const scsi = "lsblk -d -b -n -S -o NAME,SIZE -J"
	const virt = "lsblk -d -b -n -v -o NAME,SIZE -J"

	disks := []tests.DiskInfo{}

	for _, cmd := range []string{nvme,scsi,virt} {
		args := strings.Split(cmd, " ")
		cmdC := args[0]
		cmdA := args[1:]

		cmd := exec.Command(cmdC,  cmdA...)
		out, err := cmd.Output()
		if err != nil {
			log.Fatal(err)
		}

		var o output
		err = json.Unmarshal(out, &o)
		if err != nil {
			log.Fatal(err)
		}

		for _, info := range o.BlockDevices {
			disks = append(disks, tests.DiskInfo{
				Name: info.Name,
				VolumeMB: int(info.Size),
			})
		}

	}

	log.Println(disks)
}
