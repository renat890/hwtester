package tests

import (
	"context"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"time"
)

type CpuInfo struct {
	MaxTempCore float64 // максимальная температура одного ядра
	AvgTemp float64 // средняя температура по всем ядрам
	StartTemp float64
	EndTemp float64
	Name string
}

type StressInfoGetter interface {
	Load(ctx context.Context, dur time.Duration, logCh chan string) (CpuInfo, error)
}

type CPU struct {
	name string
	cfg config.Stress
	stressGetter StressInfoGetter
}

func NewTestCPU(sig StressInfoGetter, cfg config.Stress) *CPU {
	return &CPU{
		name: "Тест ЦПУ под нагрузкой",
		cfg: cfg,
		stressGetter: sig,
	}
}

func (c *CPU) Run(ctx context.Context, logCh chan string) (result hw.TestResult) {
	start := time.Now()
	result.Name = c.name
	result.Status = hw.Pass
	defer func ()  {
		result.Duration = time.Since(start)
	}()
	
	stressInfo, err := c.stressGetter.Load(ctx, c.cfg.Duration, logCh)
	if err != nil {
		result.Status = hw.Error
		return result
	}

	if checkMaxTemp(stressInfo.MaxTempCore, c.cfg.MaxHeat) {
		result.Status = hw.Fail
		result.Details += "Превышена максимальная температура 1 ядра\n"
	}

	if checkGradientTemp(stressInfo.EndTemp, stressInfo.StartTemp, c.cfg.Gradient) {
		result.Status = hw.Fail
		result.Details += "Превышен градиент температуры\n"
	}

	result.Metrics = map[string]any{
		"cpu_name": stressInfo.Name,
		"cpu_max_temp_core": stressInfo.MaxTempCore,
		"cpu_avg_temp": stressInfo.AvgTemp,
	}

	return result
}

func (c *CPU) Name() string {
	return c.name
}

func checkMaxTemp(maxTemp float64, setpoint int) bool {
	return maxTemp > float64(setpoint)
}

func checkGradientTemp(endTemp, startTemp float64, setpoint int) bool {
	return (endTemp - startTemp) > float64(setpoint)
}