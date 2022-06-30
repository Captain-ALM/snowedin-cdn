package conf

import "time"

type AccessLimitYaml struct {
	PurgeExpired bool          `yaml:"purgeExpired"`
	ExpireTime   time.Duration `yaml:"expireTime"`
	AccessLimit  uint          `yaml:"accessLimit"`
}
