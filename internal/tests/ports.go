package tests

import (
	"context"
	"factorytest/internal/config"
	"factorytest/internal/hw"
	"slices"
	"time"
)

type COMInfo struct {
	Result bool
	TestPorts []string
}

type PortsInfo struct {
	PacketsLoss int
	Ports []string
}

type USBInfo struct {
	Result bool
}

type COMTest struct {
	name string
	ci GetterCOMInfo
	conf config.Ports
}

type GetterCOMInfo interface {
	EchoTest(ctx context.Context) (COMInfo, error) 
}

func NewTestCOM(getterComInfo GetterCOMInfo, conf config.Ports) *COMTest {
	return &COMTest{
		name: "COM-порты тесты",
		ci: getterComInfo,
		conf: conf,
	}
}

func (c *COMTest) Run(ctx context.Context) (result hw.TestResult) {
	start := time.Now()
	defer func(){
		result.Duration = time.Since(start)
	}()

	result.Name = c.Name()
	comInfo, err := c.ci.EchoTest(ctx)

	if err != nil {
		result.Status = hw.Error
		result.Details = err.Error()
		return
	}

	for _, port := range c.conf.COM {
		if !slices.Contains(comInfo.TestPorts, port) {
			result.Status = hw.Fail
			result.Details = "Нет обязательных портов в системе"
			return
		}
	}

	if comInfo.Result {
		result.Status = hw.Pass
	} else {
		result.Status = hw.Fail
	}
	result.Metrics = map[string]any{
		"COM-ports": comInfo.TestPorts,
	}

	return result
}

func (c *COMTest) Name() string {
	return c.name
}

// ===============================

type EthernetsTest struct {
	name string
	ei GetterEthernetsInfo
	conf config.Ports
}

type GetterEthernetsInfo interface {
	GetEthernetsInfo(ctx context.Context) (PortsInfo, error) 
}

func NewEthernetsTest(getterEthernetsInfo GetterEthernetsInfo, conf config.Ports) *EthernetsTest {
	return &EthernetsTest{
		name: "Тесты Ethernet портов",
		ei: getterEthernetsInfo,
		conf: conf,
	}
}

func (e *EthernetsTest) Run(ctx context.Context) (result hw.TestResult) {
	start := time.Now()
	defer func ()  {
		result.Duration = time.Since(start)
	}()
	result.Name = e.Name()

	ethernetsInfo, err := e.ei.GetEthernetsInfo(ctx)
	if err != nil {
		result.Status = hw.Error
		result.Details = err.Error()
		return
	}

	reqPorts := make([]string, len(e.conf.Ethernets))
	for i := range e.conf.Ethernets {
		reqPorts[i] = e.conf.Ethernets[i].Name
	}

	for _, port := range reqPorts {
		if !slices.Contains(ethernetsInfo.Ports, port) {
			result.Status = hw.Fail
			result.Details = "Нет обязательных портов в системе"
			return
		}
	}

	if ethernetsInfo.PacketsLoss > e.conf.PacketsLoss {
		result.Status = hw.Fail
		result.Details = "Превышено допустимое количество потеряных пакетов"
	} else {
		result.Status = hw.Pass
		result.Metrics = map[string]any{
			"Ethernets-ports": ethernetsInfo.Ports,
		}
	}

	return result
}

func (e *EthernetsTest) Name() string {
	return e.name
}

// ========================

type USBTest struct {
	name string
	ui GetterUSBInfo
	conf config.USBFlash
}

type GetterUSBInfo interface {
	GetUSBInfo(ctx context.Context) (USBInfo, error)
}

func NewUSBTest(getterUSBInfo GetterUSBInfo, conf config.USBFlash) *USBTest {
	return &USBTest{
		name: "Тестирование USB-портов",
		ui: getterUSBInfo,
		conf: conf,
	}
}

func (u *USBTest) Run(ctx context.Context) (result hw.TestResult) {
	start := time.Now()
	defer func ()  {
		result.Duration = time.Since(start)
	}()
	result.Name = u.Name()

	usbInfo, err := u.ui.GetUSBInfo(ctx)
	if err != nil {
		result.Status = hw.Error
		result.Details = err.Error()
		return
	}

	if usbInfo.Result {
		result.Status = hw.Pass
	} else {
		result.Status = hw.Fail
		result.Details = "Ошибка при чтении файла с flash-накопителя"
	}

	return result
}

func (u *USBTest) Name() string {
	return u.name
}


