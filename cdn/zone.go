package cdn

import (
	"snow.mrmelon54.xyz/snowedin/structure"
	"sync"
	"time"
)

func NewZone(conf structure.ZoneYaml) *Zone {
	cZone := &Zone{
		Config:           conf,
		Backend:          NewBackendFromName(conf.Backend, conf.BackendSettings),
		Cache:            make(map[string]*Cached),
		RequestLimits:    make(map[string]*RequestLimit),
		ConnectionLimits: make(map[string]*ConnectionLimit),
	}
	if cZone.Backend == nil {
		return nil
	}
	return cZone
}

type Zone struct {
	Config           structure.ZoneYaml
	Backend          Backend
	Cache            map[string]*Cached
	RequestLimits    map[string]*RequestLimit
	ConnectionLimits map[string]*ConnectionLimit
}

func NewCached(conf structure.CacheYaml, cache []byte) *Cached {
	expr := time.Now().Add(conf.ExpireTime)
	if conf.ExpireTime.Seconds() < 1 {
		expr = time.Time{}
	}
	return &Cached{
		Cache:           cache,
		ExpireTime:      expr,
		UpdateRequested: false,
		Gone:            false,
	}
}

type Cached struct {
	Cache           []byte
	ExpireTime      time.Time
	UpdateRequested bool
	Gone            bool
}

func (ch *Cached) isExpired() bool {
	if ch.ExpireTime.IsZero() {
		return false
	} else {
		if ch.ExpireTime.After(time.Now()) {
			return false
		} else {
			return true
		}
	}
}

func NewRequestLimit(conf structure.LimitRequestsYaml) *RequestLimit {
	return &RequestLimit{
		expireTime:        time.Now().Add(conf.RequestRateInterval),
		requestsRemaining: conf.MaxRequests,
		limitConf:         conf,
	}
}

type RequestLimit struct {
	mu                sync.Mutex
	expireTime        time.Time
	requestsRemaining uint
	limitConf         structure.LimitRequestsYaml
}

func (rl *RequestLimit) startRequest() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.expireTime.After(time.Now()) {
		if rl.requestsRemaining == 0 {
			return false
		} else {
			rl.requestsRemaining--
		}
	} else {
		rl.expireTime = time.Now()
		rl.requestsRemaining = rl.limitConf.MaxRequests - 1
	}
	return true
}

func NewConnectionLimit(conf structure.LimitConnectionYaml) *ConnectionLimit {
	return &ConnectionLimit{
		connectionsRemaining: conf.MaxConnections,
		limitConf:            conf,
	}
}

type ConnectionLimit struct {
	mu                   sync.Mutex
	connectionsRemaining uint
	limitConf            structure.LimitConnectionYaml
}

func (cl *ConnectionLimit) startConnection() bool {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	if cl.connectionsRemaining == 0 {
		return false
	} else {
		cl.connectionsRemaining--
		return true
	}
}

func (cl *ConnectionLimit) stopConnection() {
	cl.mu.Lock()
	cl.connectionsRemaining++
	cl.mu.Unlock()
}
