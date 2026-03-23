package tests

import (
	"context"
	"errors"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// =============== мок ===============

type mockCpuTest struct {
	info CpuInfo
	err error
}

func (m *mockCpuTest) Load(ctx context.Context, dur time.Duration) (CpuInfo, error) {
	return m.info, m.err
}

// =============== тесты ==============

var cfg = config.Stress{
	MaxHeat: 85,
	Gradient: 40,
	Duration: 60 * time.Second,
}

func TestStressCPU(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		desc	string
		info CpuInfo
		expected hw.Status
	}{
		{
			desc: "Норм температура -> Pass",
			info: CpuInfo{
				MaxTempCore: 84,
				AvgTemp: 72,
				StartTemp: 48,
				Name: "i3-9100",
			},
			expected: hw.Pass,
			
		},
		{
			desc: "Превышение максимальной температуры -> Fail",
			info: CpuInfo{
				MaxTempCore: 86,
				AvgTemp: 72,
				StartTemp: 48,
				Name: "i3-9100",
			},
			expected: hw.Fail,
			
		},
		{
			desc: "Превышение градиента температуры -> Fail",
			info: CpuInfo{
				EndTemp: 84,
				AvgTemp: 72,
				StartTemp: 38,
				Name: "i3-9100",
			},
			expected: hw.Fail,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testCPU := NewTestCPU(&mockCpuTest{info: tC.info}, cfg)
			var result hw.TestResult = testCPU.Run(ctx)
			assert.Equal(t, tC.expected, result.Status)
		})
	}
}

func TestStressCPUError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCPU := NewTestCPU(&mockCpuTest{err: errors.New("hardware error")}, cfg)
	var result hw.TestResult = testCPU.Run(ctx)
	assert.Equal(t, hw.Error, result.Status)
}