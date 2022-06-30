package conf

import (
	"strings"
	"time"
)

type BandwidthLimitYaml struct {
	Bytes           uint          `yaml:"bytes"`
	Interval        time.Duration `yaml:"interval"`
	RemoteAddresses []string      `yaml:"remoteAddresses"`
}

func (bly BandwidthLimitYaml) YamlValid() bool {
	return bly.Bytes != 0 && bly.Interval.Milliseconds() >= 1
}

func (bly BandwidthLimitYaml) AddressContained(address string) bool {
	for _, s := range bly.RemoteAddresses {
		if strings.EqualFold(s, address) {
			return true
		}
	}
	return false
}
