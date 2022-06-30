package limits

import (
	"snow.mrmelon54.xyz/snowedin/conf"
	"time"
)

func NewAccessLimit(conf conf.AccessLimitYaml) *AccessLimit {
	expr := time.Now().Add(conf.ExpireTime)
	if conf.ExpireTime.Seconds() < 1 {
		expr = time.Time{}
	}
	return &AccessLimit{
		ExpireTime:        expr,
		Gone:              false,
		AccessLimit:       conf.AccessLimit != 0,
		AccessesRemaining: conf.AccessLimit,
	}
}

type AccessLimit struct {
	ExpireTime        time.Time
	Gone              bool
	AccessLimit       bool
	AccessesRemaining uint
}

func (al *AccessLimit) Expired() bool {
	if al.ExpireTime.IsZero() {
		return false
	} else {
		if al.ExpireTime.After(time.Now()) {
			return false
		} else {
			return true
		}
	}
}

func (al *AccessLimit) AccessLimitReached() bool {
	if al.AccessLimit {
		if al.AccessesRemaining == 0 {
			return true
		} else {
			al.AccessesRemaining--
		}
	}
	return false
}
