package report

import (
	"factorytest/internal/hw"
	"io"
)

func Generate(meta Meta, results []hw.TestResult) Report {
	final := hw.Pass

	for _, result := range results {
		if result.Status == hw.Fail || result.Status == hw.Error {
			final = hw.Fail
			break
		}
	}

	return Report{
		Meta:        meta,
		Results:     results,
		FinalResult: final,
	}
}

func WriteJSON(w io.Writer, r Report) error {
	return nil
}
