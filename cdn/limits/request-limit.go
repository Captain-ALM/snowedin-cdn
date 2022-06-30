package limits

import (
	"snow.mrmelon54.xyz/snowedin/conf"
	"sync"
	"time"
)

func NewRequestLimit(conf conf.LimitRequestsYaml) *RequestLimit {
	return &RequestLimit{
		ExpireTime:        time.Now().Add(conf.RequestRateInterval),
		RequestsRemaining: conf.MaxRequests,
		LimitConf:         conf,
		mu:                &sync.Mutex{},
	}
}

type RequestLimit struct {
	mu                *sync.Mutex
	ExpireTime        time.Time
	RequestsRemaining uint
	LimitConf         conf.LimitRequestsYaml
}

func (rl *RequestLimit) StartRequest() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.ExpireTime.After(time.Now()) {
		if rl.RequestsRemaining == 0 {
			return false
		} else {
			rl.RequestsRemaining--
		}
	} else {
		rl.ExpireTime = time.Now().Add(rl.LimitConf.RequestRateInterval)
		rl.RequestsRemaining = rl.LimitConf.MaxRequests - 1
	}
	return true
}
