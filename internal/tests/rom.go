package tests

import (
	"context"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"fmt"
	"time"
)

type DiskInfo struct {
	Name          string
	VolumeMB      int
	WriteMBPerSec int
	ReadMBPerSec  int
}

type DisksGetter interface {
	Info() ([]DiskInfo, error)
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

func (r *ROM) Run(ctx context.Context) (result hw.TestResult) {
	// обусловлено, что вычилсяется приближенно
	const threshold float64 = 0.01
	start := time.Now()
	result = hw.TestResult{Name: r.name, Status: hw.Pass}

	defer func() { result.Duration = time.Since(start) }()

	actualDisks, err := r.dg.Info()
	if err != nil {
		result.Status = hw.Error
		result.Details = "Ошибка получения информации о дисках"
		return result
	}

	if nums := len(actualDisks); nums != r.conf.Nums {
		result.Status = hw.Fail
		result.Details = fmt.Sprintf("В системе установлено %d дисков, а должно быть %d дисков", nums, r.conf.Nums)
	}

	for _, disk := range actualDisks {
		if isPassValue(r.conf.ValueMBEach, disk.VolumeMB, threshold) {
			continue
		}
		result.Status = hw.Fail
		result.Details = fmt.Sprintf("Для диска %s объем %d, а должно быть %d", disk.Name, disk.VolumeMB, r.conf.ValueMBEach)
	}

	result.Metrics = map[string]any{}
	result.Metrics["nums_disks"] = len(actualDisks)
	result.Metrics["volume_each_disk"] = r.conf.ValueMBEach
	return result
}

func (r *ROM) Name() string {
	return r.name
}
