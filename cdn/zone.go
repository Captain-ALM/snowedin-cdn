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

	if !zone.ConnectionLimits[clientIP].limitConf.YamlValid() || zone.ConnectionLimits[clientIP].startConnection() {

		if !zone.RequestLimits[clientIP].limitConf.YamlValid() || zone.RequestLimits[clientIP].startRequest() {

			lookupPath := strings.TrimPrefix(path.Clean(strings.TrimPrefix(req.URL.Path, "/"+zone.Config.Name+"/")), "/")

			if idx := strings.IndexAny(lookupPath, "?"); idx > -1 {
				lookupPath = lookupPath[:idx]
			}

			pexists, plistable := zone.Backend.Exists(lookupPath)

			if pexists {

				if zone.AccessLimits[lookupPath] == nil {
					zone.AccessLimits[lookupPath] = NewAccessLimit(zone.Config.AccessLimit)
				}

				if req.Method == http.MethodGet {

					if zone.AccessLimits[lookupPath].Gone {
						setNeverCacheHeader(rw.Header())
						http.Error(rw, "Object Gone", http.StatusGone)
					} else {
						if zone.AccessLimits[lookupPath].accessLimitReached() {
							setNeverCacheHeader(rw.Header())
							http.Error(rw, "Access Limit Reached", http.StatusForbidden)
						} else {
							if zone.AccessLimits[lookupPath].isExpired() {
								setNeverCacheHeader(rw.Header())
								http.Error(rw, "Object Expired", http.StatusGone)
							} else {
								fsSize, fsMod, err := zone.Backend.Stats(lookupPath)
								if err == nil {
									if plistable {
										list, err := zone.Backend.List(lookupPath)
										if err == nil {
											setCacheHeaderWithAge(rw.Header(), zone.Config.MaxAge, fsMod, zone.Config.PrivateCache)
											fsSize = int64(lengthOfStringSlice(list))
											rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
											rw.WriteHeader(http.StatusOK)
											for i, cs := range list {
												_, err = rw.Write([]byte(cs))
												if err != nil {
													break
												}
												if i < len(list)-1 {
													_, err = rw.Write([]byte("\r\n"))
													if err != nil {
														break
													}
												}
											}
										} else {
											setNeverCacheHeader(rw.Header())
											http.Error(rw, "Object Expired", http.StatusGone)
										}
									} else {
										if zone.AccessLimits[lookupPath].ExpireTime.IsZero() {
											setCacheHeaderWithAge(rw.Header(), zone.Config.MaxAge, fsMod, zone.Config.PrivateCache)
										} else {
											setExpiresHeader(rw.Header(), zone.AccessLimits[lookupPath].ExpireTime)
											if zone.Config.PrivateCache {
												rw.Header().Set("Cache-Control", "private")
											}
										}
										if fsSize >= 0 {
											rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
											if fsSize > 0 {
												theMimeType := zone.Backend.MimeType(lookupPath)
												if theMimeType != "" {
													rw.Header().Set("Content-Type", theMimeType)
												}
												rw.WriteHeader(http.StatusOK)
												var theWriter io.Writer
												if bwlim.YamlValid() {
													theWriter = GetLimitedBandwidthWriter(bwlim, rw)
												} else {
													theWriter = rw
												}
												_ = zone.Backend.WriteData(lookupPath, theWriter)
											} else {
												rw.WriteHeader(http.StatusOK)
											}
										} else {
											rw.WriteHeader(http.StatusForbidden)
										}
									}
								} else {
									setNeverCacheHeader(rw.Header())
									http.Error(rw, "Stat Failure: "+err.Error(), http.StatusInternalServerError)
								}
							}
						}
					}

				} else if req.Method == http.MethodDelete {
					err := zone.Backend.Purge(lookupPath)
					setNeverCacheHeader(rw.Header())
					if err == nil {
						rw.WriteHeader(http.StatusOK)
					} else {
						http.Error(rw, "Purge Error: "+err.Error(), http.StatusInternalServerError)
					}
				} else {
					http.Error(rw, "Forbidden Method", http.StatusForbidden)
				}

			} else {
				if zone.AccessLimits[lookupPath] != nil {
					zone.AccessLimits[lookupPath] = nil
				}
				setNeverCacheHeader(rw.Header())
				http.Error(rw, "Object Not Found", http.StatusNotFound)
			}
		} else {
			setNeverCacheHeader(rw.Header())
			http.Error(rw, "Too Many Requests", http.StatusTooManyRequests)
		}
		if zone.ConnectionLimits[clientIP].limitConf.YamlValid() {
			zone.ConnectionLimits[clientIP].stopConnection()
		}
	} else {
		setNeverCacheHeader(rw.Header())
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

func setNeverCacheHeader(header http.Header) {
	header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
}

func setExpiresHeader(header http.Header, expireTime time.Time) {
	header.Set("Expires", expireTime.UTC().Format(http.TimeFormat))
}

func setCacheHeaderWithAge(header http.Header, maxAge uint, modifiedTime time.Time, isPrivate bool) {
	header.Set("Cache-Control", "max-age="+strconv.Itoa(int(maxAge))+", must-revalidate")
	if isPrivate {
		header.Set("Cache-Control", header.Get("Cache-Control")+", private")
	}
	if maxAge > 0 {
		checkerSecondsBetween := time.Now().UTC().Second() - modifiedTime.UTC().Second()
		if checkerSecondsBetween < 0 {
			checkerSecondsBetween *= -1
		}
		header.Set("Age", strconv.FormatUint(uint64(checkerSecondsBetween)%uint64(maxAge), 10))
	}
}

func lengthOfStringSlice(theSlice []string) int {
	theLength := 0
	for _, cstr := range theSlice {
		theLength += len(cstr)
	}
	theLength += (len(theSlice) - 1) * 2
	return theLength
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

func GetLimitedBandwidthWriter(bly structure.BandwidthLimitYaml, targetWriter io.Writer) io.Writer {
	return &LimitedBandwidthWriter{
		passedWriter:      targetWriter,
		passedWriterIndex: 0,
		limiterSettings:   bly,
	}
}

type LimitedBandwidthWriter struct {
	passedWriter      io.Writer
	passedWriterIndex uint
	limiterSettings   structure.BandwidthLimitYaml
}

func (lbw *LimitedBandwidthWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	var currentArrayIndex uint = 0
	for currentArrayIndex < uint(len(p)) {
		if currentArrayIndex+(lbw.limiterSettings.Bytes-lbw.passedWriterIndex) < uint(len(p)) {
			written, err := lbw.passedWriter.Write(p[currentArrayIndex : currentArrayIndex+(lbw.limiterSettings.Bytes-lbw.passedWriterIndex)])
			currentArrayIndex += uint(written)
			lbw.passedWriterIndex += uint(written)
			if err != nil {
				return int(currentArrayIndex), err
			}
		} else {
			written, err := lbw.passedWriter.Write(p[currentArrayIndex:])
			currentArrayIndex += uint(written)
			lbw.passedWriterIndex += uint(written)
			if err != nil {
				return int(currentArrayIndex), err
			}
		}

		if lbw.passedWriterIndex >= lbw.limiterSettings.Bytes {
			lbw.passedWriterIndex = lbw.passedWriterIndex - lbw.limiterSettings.Bytes
			time.Sleep(lbw.limiterSettings.Interval)
		}
	}
	return len(p), nil
}
