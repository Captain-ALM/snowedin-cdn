package cdn

import (
	"github.com/tomasen/realip"
	"io"
	"net/http"
	"path"
	"snow.mrmelon54.xyz/snowedin/structure"
	"strconv"
	"strings"
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
											if processIfModSince(rw, req, fsMod, zone.Config.NotModifiedResponseUsingLastModified) {
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
												if processIfModSince(rw, req, fsMod, zone.Config.NotModifiedResponseUsingLastModified) {
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
												processIfModSince(rw, req, fsMod, zone.Config.NotModifiedResponseUsingLastModified)
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
