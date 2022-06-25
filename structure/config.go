package structure

import (
	"strings"
	"time"
)

type ConfigYaml struct {
	LogLevel uint       `yaml:"logLevel"`
	Listen   ListenYaml `yaml:"listen"`
	Zones    []ZoneYaml `yaml:"zones"`
}

type ListenYaml struct {
	Web          string        `yaml:"web"`
	Api          string        `yaml:"api"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
}

func (ly ListenYaml) GetReadTimeout() time.Duration {
	if ly.ReadTimeout.Seconds() < 15 {
		return 15 * time.Second
	} else {
		return ly.ReadTimeout
	}
}

func (ly ListenYaml) GetWriteTimeout() time.Duration {
	if ly.WriteTimeout.Hours() < 1 {
		return 1 * time.Hour
	} else {
		return ly.WriteTimeout
	}
}

type ZoneYaml struct {
	Name            string            `yaml:"name"`
	Domains         []string          `yaml:"domains"`
	MaxAge          uint              `yaml:"maxAge"`
	PrivateCache    bool              `yaml:"privateCache"`
	AccessLimit     AccessLimitYaml   `yaml:"accessLimit"`
	Limits          LimitsYaml        `yaml:"limits"`
	Backend         string            `yaml:"backend"`
	BackendSettings map[string]string `yaml:"backendSettings"`
}

type AccessLimitYaml struct {
	PurgeExpired bool          `yaml:"purgeExpired"`
	ExpireTime   time.Duration `yaml:"expireTime"`
	AccessLimit  uint          `yaml:"accessLimit"`
}

type LimitsYaml struct {
	ConnectionLimits []LimitConnectionYaml `yaml:"connectionLimits"`
	RequestLimits    []LimitRequestsYaml   `yaml:"requestLimits"`
	BandwidthLimits  []BandwidthLimitYaml  `yaml:"bandwidthLimits"`
}

func (ly LimitsYaml) GetLimitConnectionYaml(address string) LimitConnectionYaml {
	var other *LimitConnectionYaml
	var lcy *LimitConnectionYaml
	for _, lcyc := range ly.ConnectionLimits {
		if len(lcyc.RemoteAddresses) == 0 {
			other = &lcyc
		}
		if lcyc.AddressContained(address) {
			lcy = &lcyc
			break
		}
	}
	if lcy == nil && other == nil {
		lcy = &LimitConnectionYaml{}
	} else if lcy == nil && other != nil {
		lcy = other
	}
	return *lcy
}

func (ly LimitsYaml) GetLimitRequestsYaml(address string) LimitRequestsYaml {
	var other *LimitRequestsYaml
	var lry *LimitRequestsYaml
	for _, lryc := range ly.RequestLimits {
		if len(lryc.RemoteAddresses) == 0 {
			other = &lryc
		}
		if lryc.AddressContained(address) {
			lry = &lryc
			break
		}
	}
	if lry == nil && other == nil {
		lry = &LimitRequestsYaml{}
	} else if lry == nil && other != nil {
		lry = other
	}
	return *lry
}

func (ly LimitsYaml) GetBandwidthLimitYaml(address string) BandwidthLimitYaml {
	var other *BandwidthLimitYaml
	var bly *BandwidthLimitYaml
	for _, blyc := range ly.BandwidthLimits {
		if len(blyc.RemoteAddresses) == 0 {
			other = &blyc
		}
		if blyc.AddressContained(address) {
			bly = &blyc
			break
		}
	}
	if bly == nil && other == nil {
		bly = &BandwidthLimitYaml{}
	} else if bly == nil && other != nil {
		bly = other
	}
	return *bly
}

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

type LimitRequestsYaml struct {
	MaxRequests         uint          `yaml:"maxRequests"`
	RequestRateInterval time.Duration `yaml:"requestRateInterval"`
	RemoteAddresses     []string      `yaml:"remoteAddresses"`
}

func (lry LimitRequestsYaml) YamlValid() bool {
	return lry.MaxRequests != 0 && lry.RequestRateInterval.Seconds() >= 1
}

func (lry LimitRequestsYaml) AddressContained(address string) bool {
	for _, s := range lry.RemoteAddresses {
		if strings.EqualFold(s, address) {
			return true
		}
	}
	return false
}

type BandwidthLimitYaml struct {
	Bytes           uint          `yaml:"bytes"`
	Interval        time.Duration `yaml:"interval"`
	RemoteAddresses []string      `yaml:"remoteAddresses"`
}

func (bly BandwidthLimitYaml) YamlValid() bool {
	return bly.Bytes != 0 && bly.Interval.Seconds() >= 1
}

func (bly BandwidthLimitYaml) AddressContained(address string) bool {
	for _, s := range bly.RemoteAddresses {
		if strings.EqualFold(s, address) {
			return true
		}
	}
	return false
}
