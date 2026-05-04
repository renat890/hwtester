package tests

import (
	"context"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"strings"
	"time"
)

type DiskInfo struct {
	Name          string
	VolumeMB      int
	WriteMBPerSec int
	ReadMBPerSec  int
}

type DisksGetter interface {
	Info(logCh chan hw.LogMsg) ([]DiskInfo, error)
}

type ROM struct {
	name string
	dg   DisksGetter
	conf config.ROM
}

func NewTestRom(dg DisksGetter, conf config.ROM) *ROM {
	return &ROM{
		name: "Тест ПЗУ",
		dg:   dg,
		conf: conf,
	}
}

// TODO: на подумать. result.details - возможно стоит превратить в массив string,
// если не требуется останавливать тесты и проверить вообще всё
func (r *ROM) Run(ctx context.Context, logCh chan hw.LogMsg) (result hw.TestResult) {
	// обусловлено, что вычилсяется приближенно
	const threshold float64 = 0.01
	start := time.Now()
	result = hw.TestResult{Name: r.name, Status: hw.Pass}

	defer func() { result.Duration = time.Since(start) }()

	actualDisks, err := r.dg.Info(logCh)
	if err != nil {
		result.Status = hw.Error
		result.Details = "Ошибка получения информации о дисках " + err.Error()
		return result
	}

	var details strings.Builder

	if nums := len(actualDisks); nums != r.conf.Nums {
		result.Status = hw.Fail
		details.WriteString(fmt.Sprintf("В системе установлено %d дисков, а должно быть %d дисков\n", nums, r.conf.Nums))
	}

	for _, disk := range actualDisks {
		if !isPassValue(r.conf.ValueMBEach, disk.VolumeMB, threshold) {
			details.WriteString(fmt.Sprintf("Для диска %s объем %d, а должно быть %d\n", disk.Name, disk.VolumeMB, r.conf.ValueMBEach))
			result.Status = hw.Fail
		}
		if disk.ReadMBPerSec < r.conf.MinReadVMBs {
			details.WriteString(fmt.Sprintf("Для диска %s скорость чтения %d, а должно быть больше %d\n", disk.Name, disk.ReadMBPerSec, r.conf.MinReadVMBs))
			result.Status = hw.Fail
		}

		if disk.WriteMBPerSec < r.conf.MinWriteVMBs {
			details.WriteString(fmt.Sprintf("Для диска %s скорость записи %d, а должно быть больше %d \n", disk.Name, disk.WriteMBPerSec, r.conf.MinWriteVMBs))
			result.Status = hw.Fail
		}
	}

	result.Details = details.String()

	result.Metrics = map[string]any{}
	result.Metrics["nums_disks"] = len(actualDisks)
	result.Metrics["volume_each_disk"] = r.conf.ValueMBEach

	for _, disk := range actualDisks {
		key := fmt.Sprintf("Скорость чтения диска %s в MБ/c", disk.Name)
		result.Metrics[key] = disk.ReadMBPerSec
		key = fmt.Sprintf("Скорость записи диска %s в MБ/c", disk.Name)
		result.Metrics[key] = disk.WriteMBPerSec
	}

	return result
}

func (r *ROM) Name() string {
	return r.name
}
