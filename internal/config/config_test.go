package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var validConf = Config{
	RAM: RAM{
		ValueMB: 65536,
	},
	ROM: ROM{
		Nums:         4,
		ValueMBEach:  1048576,
		MinReadVMBs:  500,
		MinWriteVMBs: 500,
	},
	Ports: Ports{
		Ethernets: []Ethernet{
			{Name: "enp1s0", Address: "192.168.0.101:8765"},
            {Name: "enp2s0", Address: "192.168.0.102:8765"},
            {Name: "enp3s0", Address: "193.168.0.101:8765"},
            {Name: "enp4s0", Address: "193.168.0.101:8765"},
		},
		COM:       []string{"/dev/ttyS0", "/dev/ttyS1"},
		PacketsLoss: 0,
	},
	USBFlash: USBFlash{
		MountPoint: "/mnt/usb",
		Filename:   "test.txt",
	},
	Stress: Stress{
		MaxHeat:  85,
		Gradient: 40,
		Duration: 10 * time.Minute,
	},
	OptionalFlags: OptionalFlags{
		Ports:    true,
		USBFlash: true,
		Stress:   true,
	},
}

func TestLoad(t *testing.T) {
	testCases := []struct {
		name       string
		pathToFile string
		err        error
		expected   *Config
	}{
		{
			name:       "успешная загрузка",
			pathToFile: "./testdata/testconf.yml",
			expected:   &validConf,
		},
		{
			name:       "невалидный yaml",
			pathToFile: "./testdata/invalidconf.yml",
			err:        ErrInvalidYaml,
		},
		{
			name:       "несуществующий путь",
			pathToFile: "./abc/def",
			err:        ErrConfigNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			conf, err := Load(testCase.pathToFile)
			if testCase.err != nil {
				assert.Error(t, err)
				assert.ErrorIs(t, err, testCase.err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, conf)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	invalidConf := Config{}
	err := validateConfig(invalidConf)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrValidateConfig)
}
