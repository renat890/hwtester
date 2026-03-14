package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

var (
	ErrConfigNotFound = errors.New("не удалось обнаружить конфигурацию")
	ErrInvalidYaml    = errors.New("невалидный конфигурационный файл")
	ErrValidateConfig = errors.New("данные заданы некорректно")
)

func Load(path string) (*Config, error) {
	var conf Config
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	defer file.Close()

	if err := yaml.NewDecoder(file).Decode(&conf); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidYaml, err)
	}

	if err := validateConfig(conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func validateConfig(c Config) error {
	if c.RAM.ValueMB <= 0 {
		return fmt.Errorf("%w - RAM, значение ОЗУ в Мб должно быть больше 0", ErrValidateConfig)
	}
	if c.ROM.Nums <= 0 {
		return fmt.Errorf("%w - ROM, количество дисков должно быть больше 0", ErrValidateConfig)
	}
	if c.ROM.ValueMBEach <= 0 {
		return fmt.Errorf("%w - ROM, значение размера каждого в Мб должно быть больше 0", ErrValidateConfig)
	}
	if c.ROM.MinReadVMBs <= 0 {
		return fmt.Errorf("%w - ROM, скорость чтения для диска должна быть больше 0", ErrValidateConfig)
	}
	if c.ROM.MinWriteVMBs <= 0 {
		return fmt.Errorf("%w - ROM, скорость записи для диска должна быть больше 0", ErrValidateConfig)
	}
	return nil
}
