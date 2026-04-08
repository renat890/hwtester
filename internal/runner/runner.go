package runner

import (
	"context"
	"factorytest/internal/hw"
)

type runner struct{}

func NewTestRunner() *runner {
	return &runner{}
}

func (r *runner) Run(ctx context.Context, tests []hw.HWTest, ch chan hw.TestResult, logCh chan hw.LogMsg) []hw.TestResult {
	result := []hw.TestResult{}

	for _, test := range tests {
		var testResult hw.TestResult
		if ctx.Err() != nil {
			testResult = hw.TestResult{Name: test.Name(), Status: hw.Skip}
		} else {
			testResult = test.Run(ctx, logCh)

		}
		ch <- testResult
		result = append(result, testResult)
	}

	return result
}
