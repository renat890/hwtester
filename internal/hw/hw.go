package hw

import (
	"context"
	"time"
)

type Status string

const (
	Pass  = Status("Pass")
	Fail  = Status("Fail")
	Skip  = Status("Skip")
	Error = Status("Error")
)

type HWTest interface {
	Name() string
	Run(ctx context.Context) TestResult
}

type TestResult struct {
	Name     string
	Status   Status // Pass / Fail / Skip / Error
	Duration time.Duration
	Details  string
	Metrics  map[string]any // произвольные метрики для отчёта
}
