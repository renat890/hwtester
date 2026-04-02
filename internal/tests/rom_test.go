package tests

import (
	"context"
	"errors"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ======== мок ==========

type mockRom struct {
	disks []DiskInfo
	err   error
}

func (m *mockRom) Info() ([]DiskInfo, error) {
	return m.disks, m.err
}

// ======== тест ==========

var conf = config.ROM{
	Nums:         4,
	ValueMBEach:  4096,
	MinReadVMBs:  500,
	MinWriteVMBs: 500,
}

func TestRom(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testCases := []struct {
		desc     string
		disks    []DiskInfo
		expected hw.Status
	}{
		{
			desc:     "4 диска, объемы совпадают PASS",
			disks:    []DiskInfo{
				{Name: "/dev/sda", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sdb", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sdc", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sde", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}},
			expected: hw.Pass,
		},
		{
			desc:     "3 диска вместо 4 FAIL",
			disks:    []DiskInfo{
				{Name: "/dev/sda", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sdb", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700},
				{Name: "/dev/sdc", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}},
			expected: hw.Fail,
		},
		{
			desc:     "4 диска, один отличен по объему FAIL",
			disks:    []DiskInfo{
				{Name: "/dev/sda", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sdb", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sdc", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sde", VolumeMB: 8192, WriteMBPerSec: 560, ReadMBPerSec: 700}},
			expected: hw.Fail,
		},
		{
			desc:     "Скорость чтения и записи ОК -> Pass",
			disks:    []DiskInfo{
				{Name: "/dev/sda", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/nvme0n1", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700},
				{Name: "/dev/sdc", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sde", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}},
			expected: hw.Pass,
		},
		{
			desc: "У одного диска неверная скорость чтения -> Fail",
			disks:    []DiskInfo{
				{Name: "/dev/sda", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/nvme0n1", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 100},
				{Name: "/dev/sdc", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}, 
				{Name: "/dev/sde", VolumeMB: 4096, WriteMBPerSec: 560, ReadMBPerSec: 700}},
			expected: hw.Fail,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testROM := NewTestRom(&mockRom{disks: tC.disks}, conf)
			actual := testROM.Run(ctx, make(chan string))
			assert.Equal(t, tC.expected, actual.Status)
			assert.NotZero(t, actual.Duration)
		})
	}
}

func TestRomError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testROM := NewTestRom(&mockRom{err: errors.New("hardware error")}, conf)
	actual := testROM.Run(ctx, make(chan string))
	assert.Equal(t, hw.Error, actual.Status)
}
