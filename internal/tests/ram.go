package tests

import (
	"context"
	"factorytest/internal/hw"
	"math"
	"time"
)

type MemGetter interface {
	GetMemory() (int, error)
}

type RAM struct {
	name       string
	expectedMB int
	memGetter  MemGetter
}

func NewTestRAM(mg MemGetter, expectedMem int) *RAM {
	// Возможно, сюда стоит добавить генерацию имени, но это уже после
	return &RAM{
		name:       "Тест ОЗУ",
		expectedMB: expectedMem,
		memGetter:  mg,
	}
}

func (r *RAM) Run(ctx context.Context, logCh chan string) (result hw.TestResult) {
	const threshold float64 = 0.02

	start := time.Now()
	result = hw.TestResult{Name: r.name}
	defer func() { result.Duration = time.Since(start) }()

	actualMem, err := r.memGetter.GetMemory()
	if err != nil {
		result.Status = hw.Error
		return result
	}

	if isPassValue(r.expectedMB, actualMem, threshold) {
		result.Status = hw.Pass
	} else {
		result.Status = hw.Fail
	}

	result.Metrics = map[string]any{}
	result.Metrics["expected_mb"] = r.expectedMB
	result.Metrics["actual_mb"] = actualMem
	result.Metrics["deviation_pct"] = math.Abs(float64(actualMem)/float64(r.expectedMB)*100 - 100)

	return result
}

func (r *RAM) Name() string {
	return r.name
}
