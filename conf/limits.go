package conf

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
