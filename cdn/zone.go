package cdn

import (
	"github.com/tomasen/realip"
	"io"
	"log"
	"net/http"
	"path"
	"snow.mrmelon54.xyz/snowedin/structure"
	"strconv"
	"strings"
	"sync"
	"time"
)

var LogLevel uint = 0

func NewZone(conf structure.ZoneYaml, logLevel uint) *Zone {
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
	LogLevel = logLevel
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
		logPrintln(1, "503 Service Unavailable\nZone Backend Unavailable")
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
						logPrintln(2, "410 Gone\nObject Gone")
					} else {
						if zone.AccessLimits[lookupPath].accessLimitReached() {
							setNeverCacheHeader(rw.Header())
							http.Error(rw, "Access Limit Reached", http.StatusForbidden)
							logPrintln(2, "403 Forbidden\nAccess Limit Reached")
						} else {
							if zone.AccessLimits[lookupPath].isExpired() {
								setNeverCacheHeader(rw.Header())
								http.Error(rw, "Object Expired", http.StatusGone)
								logPrintln(2, "410 Gone\nObject Expired")
							} else {
								fsSize, fsMod, err := zone.Backend.Stats(lookupPath)
								if err == nil {
									if plistable {
										list, err := zone.Backend.List(lookupPath)
										if err == nil {
											setCacheHeaderWithAge(rw.Header(), zone.Config.MaxAge, fsMod, zone.Config.PrivateCache)
											setLastModifiedHeader(rw.Header(), fsMod)
											fsSize = int64(lengthOfStringSlice(list))
											rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
											if processIfModSince(rw, req, fsMod) {
												logPrintln(4, "Send Start")
												var theWriter io.Writer
												if bwlim.YamlValid() {
													theWriter = GetLimitedBandwidthWriter(bwlim, rw)
												} else {
													theWriter = rw
												}
												for i, cs := range list {
													_, err = theWriter.Write([]byte(cs))
													if err != nil {
														logPrintln(1, "Internal Error: "+err.Error())
														break
													}
													if i < len(list)-1 {
														_, err = theWriter.Write([]byte("\r\n"))
														if err != nil {
															logPrintln(1, "Internal Error: "+err.Error())
															break
														}
													}
												}
												if err == nil {
													logPrintln(4, "Send Complete")
												}
											}
										} else {
											setNeverCacheHeader(rw.Header())
											rw.WriteHeader(http.StatusForbidden)
											logPrintln(2, "403 Forbidden")
										}
									} else {
										setLastModifiedHeader(rw.Header(), fsMod)
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
												if processIfModSince(rw, req, fsMod) {
													logPrintln(4, "Send Start")
													var theWriter io.Writer
													if bwlim.YamlValid() {
														theWriter = GetLimitedBandwidthWriter(bwlim, rw)
													} else {
														theWriter = rw
													}
													err = zone.Backend.WriteData(lookupPath, theWriter)
													if err != nil {
														logPrintln(1, "Internal Error: "+err.Error())
													} else {
														logPrintln(4, "Send Complete")
													}
												}
											} else {
												processIfModSince(rw, req, fsMod)
											}
										} else {
											logHeaders(rw.Header())
											rw.WriteHeader(http.StatusForbidden)
											logPrintln(2, "403 Forbidden")
										}
									}
								} else {
									setNeverCacheHeader(rw.Header())
									http.Error(rw, "Stat Failure: "+err.Error(), http.StatusInternalServerError)
									logPrintln(1, "500 Internal Server Error\nStat Failure: "+err.Error())
								}
							}
						}
					}

				} else if req.Method == http.MethodDelete {
					err := zone.Backend.Purge(lookupPath)
					setNeverCacheHeader(rw.Header())
					if err == nil {
						rw.WriteHeader(http.StatusOK)
						logPrintln(2, "200 OK")
					} else {
						http.Error(rw, "Purge Error: "+err.Error(), http.StatusInternalServerError)
						logPrintln(1, "500 Internal Server Error\nPurge Error: "+err.Error())
					}
				} else {
					http.Error(rw, "Forbidden Method", http.StatusForbidden)
					logPrintln(2, "403 Forbidden\nForbidden Method")
				}

			} else {
				if zone.AccessLimits[lookupPath] != nil {
					zone.AccessLimits[lookupPath] = nil
				}
				setNeverCacheHeader(rw.Header())
				http.Error(rw, "Object Not Found", http.StatusNotFound)
				logPrintln(2, "404 Not Found\nObject Not Found")
			}
		} else {
			setNeverCacheHeader(rw.Header())
			http.Error(rw, "Too Many Requests", http.StatusTooManyRequests)
			logPrintln(2, "429 Too Many Requests\nToo Many Requests")
		}
		if zone.ConnectionLimits[clientIP].limitConf.YamlValid() {
			zone.ConnectionLimits[clientIP].stopConnection()
		}
	} else {
		setNeverCacheHeader(rw.Header())
		http.Error(rw, "Too Many Connections", http.StatusTooManyRequests)
		logPrintln(2, "429 Too Many Requests\nToo Many Connections")
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

func processIfModSince(rw http.ResponseWriter, req *http.Request, modT time.Time) bool {
	if !modT.IsZero() && req.Header.Get("If-Modified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Modified-Since"))
		if err == nil {
			if modT.Before(parse) || strings.EqualFold(modT.Format(http.TimeFormat), req.Header.Get("If-Modified-Since")) {
				logHeaders(rw.Header())
				rw.WriteHeader(http.StatusNotModified)
				logPrintln(2, "304 Not Modified")
				logPrintln(4, "Send Skipped")
				return false
			}
		}
	}
	if !modT.IsZero() && req.Header.Get("If-Unmodified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Unmodified-Since"))
		if err == nil {
			if modT.After(parse) {
				setNeverCacheHeader(rw.Header())
				if rw.Header().Get("Last-Modified") != "" {
					rw.Header().Del("Last-Modified")
				}
				if rw.Header().Get("Age") != "" {
					rw.Header().Del("Age")
				}
				if rw.Header().Get("Expires") != "" {
					rw.Header().Del("Expires")
				}
				logHeaders(rw.Header())
				rw.WriteHeader(http.StatusPreconditionFailed)
				logPrintln(2, "412 Precondition Failed")
				logPrintln(4, "Send Condition Not Satisfied")
				return false
			}
		}
	}
	logHeaders(rw.Header())
	rw.WriteHeader(http.StatusOK)
	logPrintln(2, "200 OK")
	return true
}

func setNeverCacheHeader(header http.Header) {
	header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
}

func setExpiresHeader(header http.Header, expireTime time.Time) {
	header.Set("Expires", expireTime.UTC().Format(http.TimeFormat))
}

func setLastModifiedHeader(header http.Header, modTime time.Time) {
	header.Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
}

func setCacheHeaderWithAge(header http.Header, maxAge uint, modifiedTime time.Time, isPrivate bool) {
	header.Set("Cache-Control", "max-age="+strconv.Itoa(int(maxAge))+", must-revalidate")
	if isPrivate {
		header.Set("Cache-Control", header.Get("Cache-Control")+", private")
	}
	if maxAge > 0 {
		checkerSecondsBetween := int64(time.Now().UTC().Sub(modifiedTime.UTC()).Seconds())
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

func logPrintln(minLevel uint, toLog string) {
	if LogLevel >= minLevel {
		log.Println("[Http] [Zone] " + toLog)
	}
}

func logHeaders(headers http.Header) {
	if LogLevel >= 3 {
		for k := range headers {
			log.Println("[Http] [Zone] [Header] " + k + ": " + headers.Get(k))
		}
	}
}
