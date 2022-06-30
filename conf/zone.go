package conf

type ZoneYaml struct {
	Name             string               `yaml:"name"`
	Domains          []string             `yaml:"domains"`
	AllowRange       bool                 `yaml:"allowRange"`
	CacheResponse    CacheSettingsYaml    `yaml:"cacheResponse"`
	DownloadResponse DownloadSettingsYaml `yaml:"downloadResponse"`
	AccessLimit      AccessLimitYaml      `yaml:"accessLimit"`
	Limits           LimitsYaml           `yaml:"limits"`
	Backend          string               `yaml:"backend"`
	BackendSettings  map[string]string    `yaml:"backendSettings"`
}
