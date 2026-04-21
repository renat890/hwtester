package tui

import (
	"context"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"strconv"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

// ============= мок =============

type mockRunner struct {
	results []hw.TestResult
}

func (m mockRunner) Run(ctx context.Context, tests []hw.HWTest, ch chan hw.TestResult, logCh chan hw.LogMsg) []hw.TestResult {
	for _, val := range m.results {
		if ctx.Err() != nil {
			return m.results
		} else {
			ch <- val
		}
	}

	return m.results
}

type mockTest struct {
	name string
	res hw.TestResult
}

func (m *mockTest) Name() string {
	return m.name
}

func (m *mockTest) Run(ctx context.Context, logCh chan hw.LogMsg) hw.TestResult {
	return m.res
}

// ============= тесты ===========

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
		Ethernets: []config.Ethernet{
			{Name: "enp1s0", Ip: "192.168.0.101", Port:"8765"},
            {Name: "enp2s0", Ip: "192.168.0.102", Port:"8765"},
            {Name: "enp3s0", Ip: "193.168.0.101", Port:"8765"},
            {Name: "enp4s0", Ip: "193.168.0.101", Port:"8765"},
		},
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
	m := NewModel(cfg, mockRunner{}, []hw.HWTest{}, "")
	assert.Equal(t, startScreen, m.currentScreen)

	button := tea.KeyPressMsg{
		Code: tea.KeyEnter,
	}

	newModel, _ := m.Update(button)
	updated := newModel.(Model)
	assert.Equal(t, runScreen, updated.currentScreen)
}

func TestView(t *testing.T) {
	m := NewModel(cfg, mockRunner{}, []hw.HWTest{}, "")
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
	m := NewModel(cfg, mockRunner{}, []hw.HWTest{}, "")
	button := tea.KeyPressMsg{
		Code: 'q',
	}
	_, cmd := m.Update(button)

	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	assert.True(t, ok)
}

func TestOneRun(t *testing.T) {
	mockTests := []hw.HWTest{
		&mockTest{name: "проверка ОЗУ"},
	}
	m := NewModel(cfg, mockRunner{}, mockTests, "")
	assert.Equal(t, startScreen, m.currentScreen)

	button := tea.KeyPressMsg{
		Code: tea.KeyEnter,
	}
	newModel, _ := m.Update(button)
	updated := newModel.(Model)
	assert.Equal(t, runScreen, updated.currentScreen)

	newModel, _ = updated.Update(TestDoneMsg{Result: hw.TestResult{Status: hw.Pass, Name: "проверка ОЗУ"}})
	updated = newModel.(Model)

	var v tea.View = updated.View()
	assert.Contains(t, v.Content, string(hw.Pass))
	assert.Contains(t, v.Content, "проверка ОЗУ")
	assert.Equal(t, runScreen, updated.currentScreen)
}

func TestAll(t *testing.T) {
	m := NewModel(cfg, mockRunner{}, []hw.HWTest{}, "")
	assert.Equal(t, startScreen, m.currentScreen)

	button := tea.KeyPressMsg{
		Code: tea.KeyEnter,
	}
	newModel, _ := m.Update(button)
	updated := newModel.(Model)
	assert.Equal(t, runScreen, updated.currentScreen)

	newModel, _ = updated.Update(AllDoneMsg{
		Results: []hw.TestResult{
			{Name: "Mock1", Status: hw.Pass},
			{Name: "Mock2", Status: hw.Fail},
			{Name: "Mock3", Status: hw.Pass},
		},
		Final: hw.Fail,
	})
	updated = newModel.(Model)
	assert.Equal(t, resultScreen, updated.currentScreen)

	var v tea.View = updated.View()
	assert.Contains(t, v.Content, string(hw.Pass))
	assert.Contains(t, v.Content, "Mock1")
	assert.Contains(t, v.Content, string(hw.Fail))
	assert.Contains(t, v.Content, "Mock2")
	assert.Contains(t, v.Content, "Mock3")
}
