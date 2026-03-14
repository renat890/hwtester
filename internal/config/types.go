package config

import "time"

type Config struct {
	RAM           RAM           `yaml:"ram"`
	ROM           ROM           `yaml:"rom"`
	Ports         Ports         `yaml:"ports"`
	USBFlash      USBFlash      `yaml:"usb_flash"`
	Stress        Stress        `yaml:"stress"`
	OptionalFlags OptionalFlags `yaml:"optional_flags"`
}

type RAM struct {
	ValueMB int `yaml:"value_mb"`
}

type ROM struct {
	Nums         int `yaml:"nums"`
	ValueMBEach  int `yaml:"value_mb_each"`
	MinReadVMBs  int `yaml:"min_read_v_mbs"`
	MinWriteVMBs int `yaml:"min_write_v_mbs"`
}

// TODO: добавить значения по умолчанию
type Ports struct {
	Ethernets []string `yaml:"ethernets"`
	COM       []string `yaml:"com"`
}

type USBFlash struct {
	MountPoint string `yaml:"mount_point"`
	Filename   string `yaml:"filename"`
}

type Stress struct {
	MaxHeat  int           `yaml:"max_heat"`
	Gradient int           `yaml:"gradient"`
	Duration time.Duration `yaml:"duration"`
}

type OptionalFlags struct {
	Ports    bool `yaml:"ports"`
	USBFlash bool `yaml:"usb_flash"`
	Stress   bool `yaml:"stress"`
}
