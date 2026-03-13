package config

import "time"

type Config struct {
	RAM           RAM
	ROM           ROM
	Ports         Ports
	USBFlash      USBFlash
	Stress        Stress
	OptionalFlags OptionalFlags
}

type RAM struct {
	ValueMB int
}

type ROM struct {
	Nums         int
	ValueMBEach  int
	MinReadVMBs  int
	MinWriteVMBs int
}

type Ports struct {
	Ethernets []string
	COM       []string
}

type USBFlash struct {
	MountPoint string
	Filename   string
}

type Stress struct {
	MaxHeat  int
	Gradient int
	Duration time.Time
}

type OptionalFlags struct {
	Ports    bool
	USBFlash bool
	Stress   bool
}
