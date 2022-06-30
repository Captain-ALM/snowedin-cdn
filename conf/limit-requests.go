package conf

import (
	"strings"
	"time"
)

type LimitRequestsYaml struct {
	MaxRequests         uint          `yaml:"maxRequests"`
	RequestRateInterval time.Duration `yaml:"requestRateInterval"`
	RemoteAddresses     []string      `yaml:"remoteAddresses"`
}

func (lry LimitRequestsYaml) YamlValid() bool {
	return lry.MaxRequests != 0 && lry.RequestRateInterval.Milliseconds() >= 10
}

func (lry LimitRequestsYaml) AddressContained(address string) bool {
	for _, s := range lry.RemoteAddresses {
		if strings.EqualFold(s, address) {
			return true
		}
	}
	return false
}
