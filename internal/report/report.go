package report

import (
	"encoding/json"
	"errors"
	"factorytest/internal/hw"
	"fmt"
	"io"
)

var ErrWriteJSON = errors.New("не смог записать в JSON")

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
	const prefix = ""
	const indent = "  "
	body, err := json.MarshalIndent(r, prefix, indent)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteJSON, err)
	}
	_, err = w.Write(body)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrWriteJSON, err)
	}
	return nil
}
