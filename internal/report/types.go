package report

import (
	"factorytest/internal/hw"
	"time"
)

type Report struct {
	Meta Meta
	Results []hw.TestResult
	FinalResult hw.Status
}

type Meta struct {
	Data time.Time
	DeviceName string
	VersionOS string
}