package tests

import (
	"context"
	"factorytest/internal/hw"
	"math"
	"time"
)

const threshold float64 = 0.02

type MemGetter interface {
	GetMemory() (int, error)
}

type RAM struct {
	name string
	expectedMB int
	memGetter MemGetter
}

func NewTestRAM(mg MemGetter, expectedMem int) *RAM {
	// Возможно, сюда стоит добавить генерацию имени, но это уже после
	return &RAM{
		name: "Тест ОЗУ",
		expectedMB: expectedMem,
		memGetter: mg,
	}
}

func (r *RAM) Run(ctx context.Context) hw.TestResult {
	start := time.Now()
	result := hw.TestResult{Name: r.name}

	actualMem, err := r.memGetter.GetMemory()
	if err != nil {
		result.Duration = time.Since(start)
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
	result.Metrics["deviation_pct"] = math.Abs(float64(actualMem) / float64(r.expectedMB) * 100 - 100)

	result.Duration = time.Since(start)
	return result
}

func isPassValue(expected, actual int, threshold float64) bool {
	minMb := float64(expected) * (1 - threshold)
	maxMb := float64(expected) * (1 + threshold)
	return float64(actual) >= minMb && float64(actual) <= maxMb
}

func (r *RAM) Name() string {
	return r.name
}