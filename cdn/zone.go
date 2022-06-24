package cdn

import (
	"github.com/tomasen/realip"
	"io"
	"net/http"
	"path"
	"snow.mrmelon54.xyz/snowedin/structure"
	"strconv"
	"strings"
	"sync"
	"time"
)

func NewZone(conf structure.ZoneYaml) *Zone {
	cZone := &Zone{
		Config:           conf,
		Backend:          NewBackendFromName(conf.Backend, conf.BackendSettings),
		AccessLimits:     make(map[string]*AccessLimit),
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
	AccessLimits     map[string]*AccessLimit
	RequestLimits    map[string]*RequestLimit
	ConnectionLimits map[string]*ConnectionLimit
}

func (zone *Zone) ZoneHandleRequest(rw http.ResponseWriter, req *http.Request) {
	if zone.Backend == nil {
		http.Error(rw, "Zone Backend Unavailable", http.StatusServiceUnavailable)
	}

	clientIP := realip.FromRequest(req)

	if zone.RequestLimits[clientIP] == nil {
		rqlim := zone.Config.Limits.GetLimitRequestsYaml(clientIP)
		zone.RequestLimits[clientIP] = NewRequestLimit(rqlim)
	}

	if zone.ConnectionLimits[clientIP] == nil {
		cnlim := zone.Config.Limits.GetLimitConnectionYaml(clientIP)
		zone.ConnectionLimits[clientIP] = NewConnectionLimit(cnlim)
	}

	bwlim := zone.Config.Limits.GetBandwidthLimitYaml(clientIP)

	if !zone.ConnectionLimits[clientIP].limitConf.LimitConnectionYamlValid() || zone.ConnectionLimits[clientIP].startConnection() {

		if !zone.RequestLimits[clientIP].limitConf.LimitRequestsYamlValid() || zone.RequestLimits[clientIP].startRequest() {

			lookupPath := strings.TrimPrefix(path.Clean(strings.TrimPrefix(req.URL.Path, "/"+zone.Config.Name+"/")), "/")

			if idx := strings.IndexAny(lookupPath, "?"); idx > -1 {
				lookupPath = lookupPath[:idx]
			}

			if zone.AccessLimits[lookupPath] == nil {
				zone.AccessLimits[lookupPath] = NewAccessLimit(zone.Config.AccessLimit)
			}

			if zone.Backend.Exists(lookupPath) && !zone.AccessLimits[lookupPath].Gone {
				if zone.AccessLimits[lookupPath].accessLimitReached() {
					http.Error(rw, "Access Limit Reached", http.StatusForbidden)
				} else {
					if zone.AccessLimits[lookupPath].isExpired() {
						http.Error(rw, "Object Expired", http.StatusGone)
					} else {
						fsSize := zone.Backend.Size(lookupPath)
						rw.Header().Set("Content-Length", strconv.FormatInt(zone.Backend.Size(lookupPath), 10))
						fsStrm := zone.Backend.OpenReader(lookupPath)
						if bwlim.BandwidthLimitYamlValid() {
							var fsIndex int64 = 0
							for fsIndex < fsSize {
								if fsSize-fsIndex < int64(bwlim.Bytes) {
									n, err := io.CopyN(rw, fsStrm, fsSize-fsIndex)
									if err != nil {
										http.Error(rw, "Error Passing Data", http.StatusInternalServerError)
										break
									}
									fsIndex += n
								} else {
									n, err := io.CopyN(rw, fsStrm, int64(bwlim.Bytes))
									if err != nil {
										break
									}
									fsIndex += n
									time.Sleep(bwlim.Interval)
								}
							}
							if fsIndex >= fsSize {
								rw.WriteHeader(http.StatusOK)
							}
						} else {
							_, err := io.Copy(rw, fsStrm)
							if err == nil {
								rw.WriteHeader(http.StatusOK)
							}
						}
						zone.Backend.CloseReader(fsStrm)
					}
				}
			} else {
				http.Error(rw, "Object Not Found", http.StatusNotFound)
			}

		} else {
			http.Error(rw, "Too Many Requests", http.StatusTooManyRequests)
		}
		if zone.ConnectionLimits[clientIP].limitConf.LimitConnectionYamlValid() {
			zone.ConnectionLimits[clientIP].stopConnection()
		}
	} else {
		http.Error(rw, "Too Many Connections", http.StatusTooManyRequests)
	}
}

func (zone *Zone) ZoneHostAllowed(host string) bool {
	if len(zone.Config.Domains) == 0 {
		return true
	} else {
		for _, s := range zone.Config.Domains {
			if strings.EqualFold(s, host) {
				return true
			}
		}
		return false
	}
}

func NewAccessLimit(conf structure.AccessLimitYaml) *AccessLimit {
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

func (al *AccessLimit) isExpired() bool {
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

func (al *AccessLimit) accessLimitReached() bool {
	if al.AccessLimit {
		if al.AccessesRemaining == 0 {
			return true
		} else {
			al.AccessesRemaining--
		}
	}
	return false
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
		rl.expireTime = time.Now().Add(rl.limitConf.RequestRateInterval)
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
