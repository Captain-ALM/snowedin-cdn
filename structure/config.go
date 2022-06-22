package structure

import "time"

type ConfigYaml struct {
	Listen ListenYaml `yaml:"listen"`
	Zones  []ZoneYaml `yaml:"zones"`
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
	Cache           CacheYaml         `yaml:"cacheSettings"`
	Limits          LimitsYaml        `yaml:"limits"`
	Backend         string            `yaml:"backend"`
	BackendSettings map[string]string `yaml:"BackendSettings"`
}

type CacheYaml struct {
	PurgeExpired bool          `yaml:"purgeExpired"`
	ExpireTime   time.Duration `yaml:"expireTime"`
	AccessLimit  uint          `yaml:"accessLimit"`
}

type LimitsYaml struct {
	ConnectionLimits []LimitConnectionYaml `yaml:"connectionLimits"`
	RequestLimits    []LimitRequestsYaml   `yaml:"requestLimits"`
	BandwidthLimits  []BandwidthLimitYaml  `yaml:"bandwidthLimits"`
}

type LimitConnectionYaml struct {
	MaxConnections  uint     `yaml:"maxConnections"`
	RemoteAddresses []string `yaml:"remoteAddresses"`
}

type LimitRequestsYaml struct {
	MaxRequests         uint          `yaml:"maxRequests"`
	RequestRateInterval time.Duration `yaml:"requestRateInterval"`
	RemoteAddresses     []string      `yaml:"remoteAddresses"`
}

type BandwidthLimitYaml struct {
	Bytes           []byte        `yaml:"bytes"`
	Interval        time.Duration `yaml:"interval"`
	RemoteAddresses []string      `yaml:"remoteAddresses"`
}
