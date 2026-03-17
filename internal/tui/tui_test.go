package tui

import (
	"factorytest/internal/config"
	"strconv"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

var cfg = config.Config{
	RAM: config.RAM{
		ValueMB: 65536,
	},
	ROM: config.ROM{
		Nums:         4,
		ValueMBEach:  1048576,
		MinReadVMBs:  500,
		MinWriteVMBs: 500,
	},
	Ports: config.Ports{
		Ethernets: []string{"enp0s2", "enp0s1", "enp0s3", "enp0s4"},
		COM:       []string{"/dev/ttyS0", "/dev/ttyS1"},
	},
	USBFlash: config.USBFlash{
		MountPoint: "/mnt/usb",
		Filename:   "test.txt",
	},
	Stress: config.Stress{
		MaxHeat:  85,
		Gradient: 40,
		Duration: 10 * time.Minute,
	},
	OptionalFlags: config.OptionalFlags{
		Ports:    true,
		USBFlash: true,
		Stress:   true,
	}}

func TestTui(t *testing.T) {
	m := NewModel(cfg)
	assert.Equal(t, startScreen, m.currentScreen)

	button := tea.KeyPressMsg{
		Code: tea.KeyEnter,
	}
	
	newModel, _ := m.Update(button)
	updated := newModel.(Model)
	assert.Equal(t, runScreen, updated.currentScreen)
}

func TestView(t *testing.T) {
	m := NewModel(cfg)
	assert.Equal(t, startScreen, m.currentScreen)
	var v tea.View = m.View()
	assert.Contains(t, v.Content, strconv.Itoa(cfg.RAM.ValueMB))
	assert.Contains(t, v.Content, strconv.Itoa(cfg.ROM.Nums))
	assert.Contains(t, v.Content, strconv.Itoa(cfg.ROM.MinReadVMBs))
	assert.Contains(t, v.Content, strconv.Itoa(cfg.ROM.ValueMBEach))
	assert.Contains(t, v.Content, cfg.Stress.Duration.String())
	assert.Contains(t, v.Content, strconv.Itoa(cfg.Stress.Gradient))
}

func TestQuit(t *testing.T) {
	m := NewModel(cfg)
	button := tea.KeyPressMsg{
		Code: 'q',
	}
	_, cmd := m.Update(button)

	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	assert.True(t, ok)
}

