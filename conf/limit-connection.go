package conf

import "strings"

type LimitConnectionYaml struct {
	MaxConnections  uint     `yaml:"maxConnections"`
	RemoteAddresses []string `yaml:"remoteAddresses"`
}

func (lcy LimitConnectionYaml) YamlValid() bool {
	return lcy.MaxConnections != 0
}

func (lcy LimitConnectionYaml) AddressContained(address string) bool {
	for _, s := range lcy.RemoteAddresses {
		if strings.EqualFold(s, address) {
			return true
		}
	}
	return false
}
