package runner

import (
	"context"
	"factorytest/internal/hw"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============= мок тестов ==============

type MockTest struct {
	name   string
	status hw.Status
	cancel context.CancelFunc
}

func (m MockTest) Name() string {
	return m.name
}

func (m MockTest) Run(ctx context.Context, logCh chan string) hw.TestResult {
	if m.cancel != nil {
		m.cancel()
	}
	return hw.TestResult{Name: m.name, Status: m.status}
}

// ============= тесты ==================

func TestRun(t *testing.T) {
	testCases := []struct {
		desc     string
		expected []hw.TestResult
		tests    []hw.HWTest
	}{
		{
			desc: "тестов 3",
			expected: []hw.TestResult{
				hw.TestResult{Name: "Mock1", Status: hw.Pass},
				hw.TestResult{Name: "Mock2", Status: hw.Fail},
				hw.TestResult{Name: "Mock3", Status: hw.Pass},
			},
			tests: []hw.HWTest{
				MockTest{name: "Mock1", status: hw.Pass},
				MockTest{name: "Mock2", status: hw.Fail},
				MockTest{name: "Mock3", status: hw.Pass},
			},
		},
		{
			desc:     "пустой список тестов",
			expected: []hw.TestResult{},
			tests:    []hw.HWTest{},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			runner := NewTestRunner()
			result := runner.Run(ctx, tC.tests, make(chan hw.TestResult, 4), make(chan string, 10))
			assert.Equal(t, tC.expected, result)
		})
	}
}

func TestRunWithCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	expected := []hw.TestResult{
		hw.TestResult{Name: "Mock1", Status: hw.Pass},
		hw.TestResult{Name: "Mock2", Status: hw.Skip},
		hw.TestResult{Name: "Mock3", Status: hw.Skip},
	}
	tests := []hw.HWTest{
		MockTest{name: "Mock1", status: hw.Pass, cancel: cancel},
		MockTest{name: "Mock2", status: hw.Pass},
		MockTest{name: "Mock3", status: hw.Pass},
	}

	t.Run("отмена контекста для теста", func(t *testing.T) {
		runner := NewTestRunner()
		result := runner.Run(ctx, tests, make(chan hw.TestResult, 4), make(chan string, 10))
		assert.Equal(t, expected, result)
	})
}
