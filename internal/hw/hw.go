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

type LogLevel string

const (
	INFO = LogLevel("info")
	WARN = LogLevel("warn")
	ERR  = LogLevel("err")
)

type LogMsg struct {
	Level LogLevel
	Text string
	Stamp time.Time
}

type HWTest interface {
	Name() string
	Run(ctx context.Context, logCh chan LogMsg) TestResult
}

type TestResult struct {
	Name     string         `json:"name"`
	Status   Status         `json:"status"` // Pass / Fail / Skip / Error
	Duration time.Duration  `json:"duration"`
	Details  string         `json:"details"`
	Metrics  map[string]any `json:"metrics"` // произвольные метрики для отчёта
}
