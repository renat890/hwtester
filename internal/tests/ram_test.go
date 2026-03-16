package tests

import (
	"context"
	"errors"
	"factorytest/internal/hw"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============ мок получения RAM =============

type mockMem struct {
	memV int
	err error
}

func(m *mockMem) GetMemory() (int, error) {
	return m.memV, m.err
}

// =============== тесты ==============

func TestRam(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factMem := 65536

	testCases := []struct {
		desc	string
		testMem int
		expected hw.TestResult
	}{
		{
			desc: "+0.0%",
			testMem: factMem,
			expected: hw.TestResult{Status: hw.Pass},
		},
		{
			desc: "+1.5%",
			testMem: 66519,
			expected: hw.TestResult{Status: hw.Pass},
		},
		{
			desc: "+3%",
			testMem: 67502,
			expected: hw.TestResult{Status: hw.Fail},
		},
		{
			desc: "-2",
			testMem: 64226,
			expected: hw.TestResult{Status: hw.Pass, },
		},
		
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testRAM := NewTestRAM(&mockMem{memV: int(tC.testMem)}, 65536)

			actual := testRAM.Run(ctx)
			assert.Equal(t, tC.expected.Status, actual.Status)
		})
	}
}

func TestRamError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testRAM := NewTestRAM(&mockMem{memV:  65536, err: errors.New("hardware fail")}, 65536)
	actual := testRAM.Run(ctx)
	assert.Equal(t, hw.Error, actual.Status)

}