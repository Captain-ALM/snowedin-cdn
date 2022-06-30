package conf

type ConfigYaml struct {
	LogLevel uint       `yaml:"logLevel"`
	Listen   ListenYaml `yaml:"listen"`
	Zones    []ZoneYaml `yaml:"zones"`
}
