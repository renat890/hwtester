package report

import (
	"factorytest/internal/hw"
	"time"
)

type Report struct {
	Meta        Meta            `json:"meta"`
	Results     []hw.TestResult `json:"results"`
	FinalResult hw.Status       `json:"final_results"`
}

type Meta struct {
	Date       time.Time `json:"date"`
	DeviceName string    `json:"device_name"`
	VersionOS  string    `json:"version_os"`
}
