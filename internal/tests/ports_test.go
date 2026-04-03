package tests

import (
	"context"
	"errors"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type mockCOMTest struct {
	info COMInfo
	err error
}

func (m *mockCOMTest) EchoTest(ctx context.Context, logCh chan string) (COMInfo, error) {
	return m.info, m.err
}

var confPorts = config.Ports{
	COM: []string{"/dev/ttyS0", "/dev/ttyS1"},
	Ethernets: []config.Ethernet{
			{Name: "eth0", Ip: "192.168.0.101", Port: "8765"},
            {Name: "eth1", Ip: "192.168.0.102", Port: "8765"},
		},
	PacketsLoss: 0,
}

func TestCOM(t *testing.T) {
	testCases := []struct {
		desc	string
		comInfo COMInfo
		expected hw.Status
	}{
		{
			desc: "Рабочие COM порты",
			comInfo: COMInfo{TestPorts: []string{"/dev/ttyS0", "/dev/ttyS1"}, Result: true},
			expected: hw.Pass, 
		},
		{
			desc: "Какой-то не работает из COM портов",
			comInfo: COMInfo{TestPorts: []string{"/dev/ttyS0", "/dev/ttyS1"}, Result: false},
			expected: hw.Fail, 
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testCOM := NewTestCOM(&mockCOMTest{info: tC.comInfo}, confPorts)
			actual := testCOM.Run(ctx, make(chan string))
			assert.Equal(t, tC.expected, actual.Status)
		})
	}
}

func TestCOMError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	testCOM := NewTestCOM(&mockCOMTest{err: errors.New("hardware error")}, confPorts)
	actual := testCOM.Run(ctx, make(chan string))
	assert.Equal(t, hw.Error, actual.Status)
}

// ======================================================

type mockEthernetsInfo struct {
	pi PortsInfo
	err error
}

func (m *mockEthernetsInfo) GetEthernetsInfo(ctx context.Context, eths []config.Ethernet, logCh chan string) (PortsInfo, error) {
	return m.pi, m.err
}



func TestEthernets(t *testing.T) {
	testCases := []struct {
		desc	string
		expected hw.Status
		portsInfo PortsInfo
	}{
		{
			desc: "Параметры ОК",
			expected: hw.Pass,
			portsInfo: PortsInfo{PacketsLoss: 0, Ports: []string{"eth0", "eth1"}},
		},
		{
			desc: "Есть потеряные пакеты",
			expected: hw.Fail,
			portsInfo: PortsInfo{PacketsLoss: 10, Ports: []string{"eth0", "eth1"}},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testEthernets := NewEthernetsTest(&mockEthernetsInfo{pi: tC.portsInfo}, confPorts)
			actual := testEthernets.Run(ctx, make(chan string))
			assert.Equal(t, tC.expected, actual.Status)
		})
	}
}

func TestEthernetsError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	testEthernets := NewEthernetsTest(&mockEthernetsInfo{err: errors.New("hardware error")}, confPorts)
	actual := testEthernets.Run(ctx, make(chan string))
	assert.Equal(t, hw.Error, actual.Status)
}

// ==============================================================

type mockUSBInfo struct {
	ui USBInfo
	err error
}

func (m *mockUSBInfo) GetUSBInfo(ctx context.Context, logCh chan string) (USBInfo, error) {
	return m.ui, m.err
}

var confUSB = config.USBFlash{
	MountPoint: "/mnt/usb",
	Filename: "test.txt",
}

func TestUSB(t *testing.T) {
	testCases := []struct {
		desc	string
		expected hw.Status
		usbInfo USBInfo
	}{
		{
			desc: "Порты исправны",
			expected: hw.Pass,
			usbInfo: USBInfo{Result: true},
		},
		{
			desc: "Один из портов не работает",
			expected: hw.Fail,
			usbInfo: USBInfo{Result: false},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			testUSB := NewUSBTest(&mockUSBInfo{ui: tC.usbInfo}, confUSB)
			actual := testUSB.Run(ctx, make(chan string))
			assert.Equal(t, tC.expected, actual.Status)
		})
	}
}

func TestUSBError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	testUSB := NewUSBTest(&mockUSBInfo{err: errors.New("hardware error")}, confUSB)
	actual := testUSB.Run(ctx, make(chan string))
	assert.Equal(t, hw.Error, actual.Status)
}