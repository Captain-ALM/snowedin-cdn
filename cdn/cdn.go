package cdn

import (
	"snow.mrmelon54.xyz/snowedin/conf"
)

func New(config conf.ConfigYaml) *CDN {
	toReturn := &CDN{Config: config}
	toReturn.Zones = make([]*Zone, len(toReturn.Config.Zones))
	for i, z := range toReturn.Config.Zones {
		toReturn.Zones[i] = NewZone(z, config.LogLevel)
	}
	return toReturn
}

type CDN struct {
	Config conf.ConfigYaml
	Zones  []*Zone
}
