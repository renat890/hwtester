package report

import (
	"bytes"
	"encoding/json"
	"factorytest/internal/hw"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	testCases := []struct {
		desc     string
		expected hw.Status
		results  []hw.TestResult
	}{
		{
			desc:     "все тесты pass",
			expected: hw.Pass,
			results: []hw.TestResult{
				{
					Name:     "Тест ОЗУ",
					Status:   hw.Pass,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
				{
					Name:     "Тест ПЗУ",
					Status:   hw.Pass,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
			},
		},
		{
			desc:     "один тест fail",
			expected: hw.Fail,
			results: []hw.TestResult{
				{
					Name:     "Тест ОЗУ",
					Status:   hw.Pass,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
				{
					Name:     "Тест ПЗУ",
					Status:   hw.Fail,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
					Details: "есть какая-то проблема",
				},
			},
		},
		{
			desc:     "есть skip",
			expected: hw.Pass,
			results: []hw.TestResult{
				{
					Name:     "Тест ОЗУ",
					Status:   hw.Pass,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
				{
					Name:     "Тест ПЗУ",
					Status:   hw.Skip,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
			},
		},
		{
			desc:     "есть error",
			expected: hw.Fail,
			results: []hw.TestResult{
				{
					Name:     "Тест ОЗУ",
					Status:   hw.Pass,
					Duration: time.Second.Round(time.Second),
					Metrics: map[string]any{
						"1": float64(2),
					},
				},
				{
					Name:     "Тест ПЗУ",
					Status:   hw.Error,
					Duration: time.Second.Round(time.Second),
					Details: "ошибка получения ПЗУ",
				},
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			meta := Meta{
				Date:       time.Now().Round(0),
				DeviceName: "ARIS 6820",
				VersionOS:  "1.8.1",
			}
			reportTest := Generate(meta, tC.results)
			expected := Report{
				Meta:        meta,
				Results:     tC.results,
				FinalResult: tC.expected,
			}
			assert.Equal(t, expected, reportTest)

			reportJ, err := json.Marshal(reportTest)
			assert.NoError(t, err)

			var reportTrip Report
			err = json.Unmarshal(reportJ, &reportTrip)
			assert.NoError(t, err)

			assert.Equal(t, expected, reportTrip)
		})
	}
}

func TestWriteJSON(t *testing.T) {
	expected := Report{
		Meta: Meta{
			Date:       time.Now().Round(0),
			DeviceName: "ARIS 6820",
			VersionOS:  "1.8.1",
		},
		Results: []hw.TestResult{
			{
				Name:     "Тест ОЗУ",
				Status:   hw.Pass,
				Duration: time.Second.Round(time.Second),
				Metrics: map[string]any{
					"1": float64(2),
				},
			},
			{
				Name:     "Тест ПЗУ",
				Status:   hw.Pass,
				Duration: time.Second.Round(time.Second),
				Metrics: map[string]any{
					"1": float64(2),
				},
			},
		},
		FinalResult: hw.Pass,
	}



	var buffer bytes.Buffer
	err := WriteJSON(&buffer, expected)
	assert.NoError(t, err)
	var actualReport Report
	err = json.Unmarshal(buffer.Bytes(), &actualReport)
	assert.NoError(t, err)
	assert.Equal(t, expected, actualReport)
}