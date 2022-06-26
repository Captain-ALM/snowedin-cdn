package cdn

import (
	"github.com/tomasen/realip"
	"io"
	"net/http"
	"path"
	"snow.mrmelon54.xyz/snowedin/structure"
	"strconv"
	"strings"
	"time"
)

var LogLevel uint = 0

func NewZone(conf structure.ZoneYaml, logLevel uint) *Zone {
	var thePathAttributes map[string]*ZonePathAttributes
	if conf.CacheResponse.RequestLimitedCacheCheck {
		thePathAttributes = make(map[string]*ZonePathAttributes)
	}
	cZone := &Zone{
		Config:           conf,
		Backend:          NewBackendFromName(conf.Backend, conf.BackendSettings),
		AccessLimits:     make(map[string]*AccessLimit),
		RequestLimits:    make(map[string]*RequestLimit),
		ConnectionLimits: make(map[string]*ConnectionLimit),
		PathAttributes:   thePathAttributes,
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
	PathAttributes   map[string]*ZonePathAttributes
}

func (zone *Zone) ZoneHandleRequest(rw http.ResponseWriter, req *http.Request) {
	if zone.Backend == nil {
		writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusServiceUnavailable, "Zone Backend Unavailable")
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

		lookupPath := strings.TrimPrefix(path.Clean(strings.TrimPrefix(req.URL.Path, "/"+zone.Config.Name+"/")), "/")

		if idx := strings.IndexAny(lookupPath, "?"); idx > -1 {
			lookupPath = lookupPath[:idx]
		}

		if !zone.RequestLimits[clientIP].limitConf.YamlValid() || zone.RequestLimits[clientIP].startRequest() {

			pexists, plistable := zone.Backend.Exists(lookupPath)

			if pexists {

				zLAccessLimts := zone.AccessLimits[lookupPath]
				if zLAccessLimts == nil {
					zLAccessLimts = NewAccessLimit(zone.Config.AccessLimit)
					zone.AccessLimits[lookupPath] = zLAccessLimts
				}

				if req.Method == http.MethodGet || req.Method == http.MethodHead {

					if zLAccessLimts.Gone {
						setNeverCacheHeader(rw.Header())
						writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusGone, "Object Gone")
					} else {
						if zLAccessLimts.accessLimitReached() {
							setNeverCacheHeader(rw.Header())
							writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "Access Limit Reached")
						} else {
							if zLAccessLimts.isExpired() {
								setNeverCacheHeader(rw.Header())
								writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusGone, "Object Expired")
							} else {
								fsSize, fsMod, err := zone.Backend.Stats(lookupPath)
								if err == nil {
									theETag := zone.Backend.ETag(lookupPath)
									if plistable {
										list, err := zone.Backend.List(lookupPath)
										if err == nil {
											setLastModifiedHeader(rw.Header(), fsMod)
											if zLAccessLimts.ExpireTime.IsZero() {
												setCacheHeaderWithAge(rw.Header(), zone.Config.CacheResponse.MaxAge, fsMod, zone.Config.CacheResponse.PrivateCache)
											} else {
												setExpiresHeader(rw.Header(), zLAccessLimts.ExpireTime)
												if zone.Config.CacheResponse.PrivateCache {
													rw.Header().Set("Cache-Control", "private")
												}
											}
											fsSize = int64(lengthOfStringSlice(list))
											if theETag == "" {
												theETag = getValueForETagUsingAttributes(fsMod, fsSize)
											}
											rw.Header().Set("ETag", theETag)
											rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
											setDownloadHeaders(rw.Header(), zone.Config.DownloadResponse, getFilenameFromPath(lookupPath), "text/plain; charset=utf-8")
											if processSupportedPreconditions200(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags) {
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
											if zone.Config.CacheResponse.RequestLimitedCacheCheck {
												if zone.PathAttributes[lookupPath] == nil {
													zone.PathAttributes[lookupPath] = NewZonePathAttributes(fsMod, theETag)
												} else {
													zone.PathAttributes[lookupPath].Update(fsMod, theETag, rw.Header())
												}
											}
										} else {
											setNeverCacheHeader(rw.Header())
											writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "")
										}
									} else {
										if theETag == "" {
											theETag = getValueForETagUsingAttributes(fsMod, fsSize)
										}
										rw.Header().Set("ETag", theETag)
										setLastModifiedHeader(rw.Header(), fsMod)
										if zLAccessLimts.ExpireTime.IsZero() {
											setCacheHeaderWithAge(rw.Header(), zone.Config.CacheResponse.MaxAge, fsMod, zone.Config.CacheResponse.PrivateCache)
										} else {
											setExpiresHeader(rw.Header(), zLAccessLimts.ExpireTime)
											if zone.Config.CacheResponse.PrivateCache {
												rw.Header().Set("Cache-Control", "private")
											}
										}
										if fsSize >= 0 {
											rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
											if fsSize > 0 {
												theMimeType := zone.Backend.MimeType(lookupPath)
												if theMimeType != "" {
													setDownloadHeaders(rw.Header(), zone.Config.DownloadResponse, getFilenameFromPath(lookupPath), theMimeType)
													rw.Header().Set("Content-Type", theMimeType)
												}
												if processSupportedPreconditions200(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags) {
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
												processSupportedPreconditions200(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags)
											}
											if zone.Config.CacheResponse.RequestLimitedCacheCheck {
												if zone.PathAttributes[lookupPath] == nil {
													zone.PathAttributes[lookupPath] = NewZonePathAttributes(fsMod, theETag)
												} else {
													zone.PathAttributes[lookupPath].Update(fsMod, theETag, rw.Header())
												}
											}
										} else {
											switchToNonCachingHeaders(rw.Header())
											writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "")
										}
									}
								} else {
									setNeverCacheHeader(rw.Header())
									writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusInternalServerError, "Stat Failure: "+err.Error())
								}
							}
						}
					}

				} else if req.Method == http.MethodDelete {
					err := zone.Backend.Purge(lookupPath)
					if zone.Config.CacheResponse.RequestLimitedCacheCheck && zone.PathAttributes[lookupPath] != nil {
						zone.PathAttributes[lookupPath].NotExpunged = false
					}
					setNeverCacheHeader(rw.Header())
					if err == nil {
						writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "")
					} else {
						writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusInternalServerError, "Purge Error: "+err.Error())
					}
				} else {
					writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusForbidden, "Forbidden Method")
				}

			} else {
				if zone.Config.CacheResponse.RequestLimitedCacheCheck && zone.PathAttributes[lookupPath] != nil {
					zone.PathAttributes[lookupPath].NotExpunged = false
				}
				if zone.AccessLimits[lookupPath] != nil {
					zone.AccessLimits[lookupPath] = nil
				}
				setNeverCacheHeader(rw.Header())
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotFound, "Object Not Found")
			}
		} else {
			if zone.Config.CacheResponse.RequestLimitedCacheCheck && zone.PathAttributes[lookupPath] != nil && zone.PathAttributes[lookupPath].NotExpunged {
				zone.PathAttributes[lookupPath].UpdateHeader(rw.Header())
				processSupportedPreconditions429(rw, req, zone.PathAttributes[lookupPath].lastModifiedTime, zone.PathAttributes[lookupPath].eTag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags)
			} else {
				setNeverCacheHeader(rw.Header())
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusTooManyRequests, "Too Many Requests")
			}
		}
		if zone.ConnectionLimits[clientIP].limitConf.YamlValid() {
			zone.ConnectionLimits[clientIP].stopConnection()
		}
	} else {
		setNeverCacheHeader(rw.Header())
		writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusTooManyRequests, "Too Many Connections")
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

func NewZonePathAttributes(lModTime time.Time, eTag string) *ZonePathAttributes {
	return &ZonePathAttributes{
		lastModifiedTime: lModTime,
		eTag:             eTag,
		NotExpunged:      true,
	}
}

type ZonePathAttributes struct {
	lastModifiedTime time.Time
	eTag             string
	contentLength    string
	contentType      string
	cacheControl     string
	age              string
	expire           string
	NotExpunged      bool
}

func (zpa *ZonePathAttributes) Update(lModTime time.Time, eTag string, header http.Header) {
	zpa.NotExpunged = true
	zpa.lastModifiedTime = lModTime
	zpa.eTag = eTag
	zpa.contentLength = header.Get("Content-Length")
	zpa.contentType = header.Get("Content-Type")
	zpa.cacheControl = header.Get("Cache-Control")
	zpa.age = header.Get("Age")
	zpa.expire = header.Get("Expires")
}

func (zpa *ZonePathAttributes) UpdateHeader(header http.Header) {
	if zpa.NotExpunged {
		if zpa.contentLength != "" {
			header.Set("Content-Length", zpa.contentLength)
		}
		if zpa.contentType != "" {
			header.Set("Content-Type", zpa.contentType)
		}
		if zpa.cacheControl != "" {
			header.Set("Cache-Control", zpa.cacheControl)
		}
		if zpa.age != "" {
			header.Set("Age", zpa.age)
		}
		if zpa.expire != "" {
			header.Set("Expires", zpa.expire)
		}
	}
}
