package cdn

import (
	"snow.mrmelon54.xyz/snowedin/structure"
)

func New(config structure.ConfigYaml) *CDN {
	toReturn := &CDN{Config: config}
	toReturn.Zones = make([]*Zone, len(toReturn.Config.Zones))
	for i, z := range toReturn.Config.Zones {
		toReturn.Zones[i] = NewZone(z)
	}
	return toReturn
}

type CDN struct {
	Config structure.ConfigYaml
	Zones  []*Zone
}
