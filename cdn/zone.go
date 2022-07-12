package cdn

import (
	"github.com/tomasen/realip"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path"
	"snow.mrmelon54.xyz/snowedin/cdn/limits"
	"snow.mrmelon54.xyz/snowedin/cdn/utils"
	"snow.mrmelon54.xyz/snowedin/conf"
	"strconv"
	"strings"
	"sync"
)

func NewZone(conf conf.ZoneYaml, logLevel uint) *Zone {
	var thePathAttributes map[string]*ZonePathAttributes
	if conf.CacheResponse.RequestLimitedCacheCheck {
		thePathAttributes = make(map[string]*ZonePathAttributes)
	}
	cZone := &Zone{
		Config:           conf,
		Backend:          NewBackendFromName(conf.Backend, conf.BackendSettings),
		mutAccess:        new(sync.RWMutex),
		mutRequest:       new(sync.RWMutex),
		mutConn:          new(sync.RWMutex),
		mutPathAttr:      new(sync.RWMutex),
		AccessLimits:     make(map[string]*limits.AccessLimit),
		RequestLimits:    make(map[string]*limits.RequestLimit),
		ConnectionLimits: make(map[string]*limits.ConnectionLimit),
		PathAttributes:   thePathAttributes,
	}
	if cZone.Backend == nil {
		return nil
	}
	utils.LogLevel = logLevel
	return cZone
}

type Zone struct {
	Config           conf.ZoneYaml
	Backend          Backend
	mutAccess        *sync.RWMutex
	mutRequest       *sync.RWMutex
	mutConn          *sync.RWMutex
	mutPathAttr      *sync.RWMutex
	AccessLimits     map[string]*limits.AccessLimit
	RequestLimits    map[string]*limits.RequestLimit
	ConnectionLimits map[string]*limits.ConnectionLimit
	PathAttributes   map[string]*ZonePathAttributes
}

func (zone *Zone) checkRequestLimits(clientIP string) *limits.RequestLimit {
	zone.mutRequest.Lock()
	a := zone.RequestLimits[clientIP]
	if a == nil {
		reqLimit := zone.Config.Limits.GetLimitRequestsYaml(clientIP)
		a = limits.NewRequestLimit(reqLimit)
		zone.RequestLimits[clientIP] = a
	}
	zone.mutRequest.Unlock()
	return a
}

func (zone *Zone) checkConnectionLimits(clientIP string) *limits.ConnectionLimit {
	zone.mutConn.Lock()
	a := zone.ConnectionLimits[clientIP]
	if a == nil {
		connLimit := zone.Config.Limits.GetLimitConnectionYaml(clientIP)
		a = limits.NewConnectionLimit(connLimit)
		zone.ConnectionLimits[clientIP] = a
	}
	zone.mutConn.Unlock()
	return a
}

func (zone *Zone) checkAccessLimits(lookupPath string) *limits.AccessLimit {
	zone.mutAccess.Lock()
	a := zone.AccessLimits[lookupPath]
	if a == nil {
		a = limits.NewAccessLimit(zone.Config.AccessLimit)
		zone.AccessLimits[lookupPath] = a
	}
	zone.mutAccess.RUnlock()
	return a
}

func (zone *Zone) checkPathAttributes(lookupPath string) *ZonePathAttributes {
	zone.mutPathAttr.RLock()
	defer zone.mutPathAttr.RUnlock()
	return zone.PathAttributes[lookupPath]
}

func (zone *Zone) ZoneHandleRequest(rw http.ResponseWriter, req *http.Request) {
	if zone.Backend == nil {
		writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusServiceUnavailable, "Zone Backend Unavailable")
	}

	clientIP := realip.FromRequest(req)

	reqLimit := zone.checkRequestLimits(clientIP)
	connLimit := zone.checkConnectionLimits(clientIP)

	bwLim := zone.Config.Limits.GetBandwidthLimitYaml(clientIP)

	if !connLimit.LimitConf.YamlValid() || connLimit.StartConnection() {
		lookupPath := strings.TrimPrefix(path.Clean(strings.TrimPrefix(req.URL.Path, "/"+zone.Config.Name+"/")), "/")

		if idx := strings.IndexAny(lookupPath, "?"); idx > -1 {
			lookupPath = lookupPath[:idx]
		}

		if !reqLimit.LimitConf.YamlValid() || reqLimit.StartRequest() {
			pExists, pListTable := zone.Backend.Exists(lookupPath)

			if pExists {
				assLimit := zone.checkAccessLimits(lookupPath)

				switch req.Method {
				case http.MethodGet, http.MethodHead:
					zone.handleZoneGetAndHead(rw, req, assLimit, lookupPath, pListTable, bwLim)
				case http.MethodDelete:
					err := zone.Backend.Purge(lookupPath)
					pAttr := zone.checkPathAttributes(lookupPath)
					if zone.Config.CacheResponse.RequestLimitedCacheCheck && pAttr != nil {
						pAttr.NotExpunged = false
					}

					utils.SetNeverCacheHeader(rw.Header())
					if err == nil {
						writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "")
					} else {
						writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusInternalServerError, "Purge Error: "+err.Error())
					}
				default:
					writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusForbidden, "Forbidden Method")
				}

			} else {
				pAttr := zone.checkPathAttributes(lookupPath)
				if zone.Config.CacheResponse.RequestLimitedCacheCheck && pAttr != nil {
					pAttr.NotExpunged = false
				}
				zone.mutAccess.Lock()
				if zone.AccessLimits[lookupPath] != nil {
					zone.AccessLimits[lookupPath] = nil
				}
				zone.mutAccess.Unlock()
				utils.SetNeverCacheHeader(rw.Header())
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotFound, "Object Not Found")
			}
		} else {
			pAttr := zone.checkPathAttributes(lookupPath)
			if zone.Config.CacheResponse.RequestLimitedCacheCheck && pAttr != nil && pAttr.NotExpunged {
				pAttr.UpdateHeader(rw.Header())
				processSupportedPreconditions429(rw, req, pAttr.lastModifiedTime, pAttr.eTag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags)
			} else {
				utils.SetNeverCacheHeader(rw.Header())
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusTooManyRequests, "Too Many Requests")
			}
		}
		if connLimit.LimitConf.YamlValid() {
			connLimit.StopConnection()
		}
	} else {
		utils.SetNeverCacheHeader(rw.Header())
		writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusTooManyRequests, "Too Many Connections")
	}
}

func (zone *Zone) handleZoneGetAndHead(rw http.ResponseWriter, req *http.Request, zLAccessLimts *limits.AccessLimit, lookupPath string, plistable bool, bwlim conf.BandwidthLimitYaml) {
	if zLAccessLimts.Gone {
		utils.SetNeverCacheHeader(rw.Header())
		writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusGone, "Object Gone")
	} else {
		if zLAccessLimts.AccessLimitReached() {
			utils.SetNeverCacheHeader(rw.Header())
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "Access Limit Reached")
		} else {
			if zLAccessLimts.Expired() {
				utils.SetNeverCacheHeader(rw.Header())
				if zone.Config.AccessLimit.PurgeExpired {
					err := zone.Backend.Purge(lookupPath)
					if err == nil {
						writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusGone, "Object Expired")
					} else {
						writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusInternalServerError, "Purge Error: "+err.Error())
					}
				} else {
					writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusGone, "Object Expired")
				}
			} else {
				fsSize, fsMod, err := zone.Backend.Stats(lookupPath)
				if err == nil {
					theETag := zone.Backend.ETag(lookupPath)
					if plistable {
						list, err := zone.Backend.List(lookupPath)
						if err == nil {
							utils.SetLastModifiedHeader(rw.Header(), fsMod)
							if zLAccessLimts.ExpireTime.IsZero() {
								utils.SetCacheHeaderWithAge(rw.Header(), zone.Config.CacheResponse.MaxAge, fsMod, zone.Config.CacheResponse.PrivateCache)
							} else {
								utils.SetExpiresHeader(rw.Header(), zLAccessLimts.ExpireTime)
								if zone.Config.CacheResponse.PrivateCache {
									rw.Header().Set("Cache-Control", "private")
								}
							}
							fsSize = int64(utils.LengthOfStringSlice(list))
							if theETag == "" {
								theETag = utils.GetValueForETagUsingAttributes(fsMod, fsSize)
							}
							rw.Header().Set("ETag", theETag)
							rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
							rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
							if zone.Config.DownloadResponse.OutputDisposition {
								utils.SetDownloadHeaders(rw.Header(), zone.Config.DownloadResponse, utils.GetFilenameFromPath(lookupPath), rw.Header().Get("Content-Type"))
							}
							if processSupportedPreconditionsForNext(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags) {
								httpRangeParts := processRangePreconditions(fsSize, rw, req, fsMod, theETag, zone.Config.AllowRange)
								if httpRangeParts != nil {
									if len(httpRangeParts) <= 1 {
										utils.LogPrintln(4, "Send Start")
										var theWriter io.Writer
										if bwlim.YamlValid() {
											theWriter = limits.GetLimitedBandwidthWriter(bwlim, rw)
										} else {
											theWriter = rw
										}
										if len(httpRangeParts) == 1 {
											theWriter = limits.NewPartialRangeWriter(theWriter, httpRangeParts[0])
										}
										for i, cs := range list {
											_, err = theWriter.Write([]byte(cs))
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
												break
											}
											if i < len(list)-1 {
												_, err = theWriter.Write([]byte("\r\n"))
												if err != nil {
													utils.LogPrintln(1, "Internal Error: "+err.Error())
													break
												}
											}
										}
										if err == nil {
											utils.LogPrintln(4, "Send Complete")
										}
									} else {
										utils.LogPrintln(4, "Send Start")
										theListingString := ""
										for i, cs := range list {
											theListingString += cs
											if i < len(list)-1 {
												theListingString += "\r\n"
											}
										}
										var theWriter io.Writer
										if bwlim.YamlValid() {
											theWriter = limits.GetLimitedBandwidthWriter(bwlim, rw)
										} else {
											theWriter = rw
										}
										multWriter := multipart.NewWriter(theWriter)
										rw.Header().Set("Content-Type", "multipart/byteranges; boundary="+multWriter.Boundary())
										utils.LogPrintln(3, "Content-Type: multipart/byteranges; boundary="+multWriter.Boundary())
										for _, currentPart := range httpRangeParts {
											mimePart, err := multWriter.CreatePart(textproto.MIMEHeader{
												"Content-Range": {currentPart.ToField(fsSize)},
												"Content-Type":  {"text/plain; charset=utf-8"},
											})
											utils.LogPrintln(3, "Content-Range: "+currentPart.ToField(fsSize))
											utils.LogPrintln(3, "Content-Type: text/plain; charset=utf-8")
											utils.LogPrintln(4, "Part Start")
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
												break
											}
											_, err = mimePart.Write([]byte(theListingString[currentPart.Start : currentPart.Start+currentPart.Length]))
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
												break
											}
											utils.LogPrintln(4, "Part End")
										}
										err := multWriter.Close()
										if err != nil {
											utils.LogPrintln(1, "Internal Error: "+err.Error())
										} else {
											utils.LogPrintln(4, "Send Complete")
										}
									}
								}
							}
							if zone.Config.CacheResponse.RequestLimitedCacheCheck {
								zone.mutPathAttr.Lock()
								if zone.PathAttributes[lookupPath] == nil {
									zone.PathAttributes[lookupPath] = NewZonePathAttributes(fsMod, theETag)
								} else {
									zone.PathAttributes[lookupPath].Update(fsMod, theETag, rw.Header())
								}
								zone.mutPathAttr.Unlock()
							}
						} else {
							utils.SetNeverCacheHeader(rw.Header())
							writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "")
						}
					} else {
						if theETag == "" {
							theETag = utils.GetValueForETagUsingAttributes(fsMod, fsSize)
						}
						rw.Header().Set("ETag", theETag)
						utils.SetLastModifiedHeader(rw.Header(), fsMod)
						if zLAccessLimts.ExpireTime.IsZero() {
							utils.SetCacheHeaderWithAge(rw.Header(), zone.Config.CacheResponse.MaxAge, fsMod, zone.Config.CacheResponse.PrivateCache)
						} else {
							utils.SetExpiresHeader(rw.Header(), zLAccessLimts.ExpireTime)
							if zone.Config.CacheResponse.PrivateCache {
								rw.Header().Set("Cache-Control", "private")
							}
						}
						if fsSize >= 0 {
							rw.Header().Set("Content-Length", strconv.FormatInt(fsSize, 10))
							if fsSize > 0 {
								theMimeType := zone.Backend.MimeType(lookupPath)
								if theMimeType != "" {
									if zone.Config.DownloadResponse.OutputDisposition {
										utils.SetDownloadHeaders(rw.Header(), zone.Config.DownloadResponse, utils.GetFilenameFromPath(lookupPath), theMimeType)
									}
									rw.Header().Set("Content-Type", theMimeType)
								}
								if processSupportedPreconditionsForNext(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags) {
									httpRangeParts := processRangePreconditions(fsSize, rw, req, fsMod, theETag, zone.Config.AllowRange)
									if httpRangeParts != nil {
										if len(httpRangeParts) == 0 {
											utils.LogPrintln(4, "Send Start")
											var theWriter io.Writer
											if bwlim.YamlValid() {
												theWriter = limits.GetLimitedBandwidthWriter(bwlim, rw)
											} else {
												theWriter = rw
											}
											err = zone.Backend.WriteData(lookupPath, theWriter)
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
											} else {
												utils.LogPrintln(4, "Send Complete")
											}
										} else if len(httpRangeParts) == 1 {
											utils.LogPrintln(4, "Send Start")
											var theWriter io.Writer
											if bwlim.YamlValid() {
												theWriter = limits.GetLimitedBandwidthWriter(bwlim, rw)
											} else {
												theWriter = rw
											}
											err = zone.Backend.WriteDataRange(lookupPath, theWriter, httpRangeParts[0].Start, httpRangeParts[0].Length)
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
											} else {
												utils.LogPrintln(4, "Send Complete")
											}
										} else {
											utils.LogPrintln(4, "Send Start")
											var theWriter io.Writer
											if bwlim.YamlValid() {
												theWriter = limits.GetLimitedBandwidthWriter(bwlim, rw)
											} else {
												theWriter = rw
											}
											mWriter := multipart.NewWriter(theWriter)
											rw.Header().Set("Content-Type", "multipart/byteranges; boundary="+mWriter.Boundary())
											utils.LogPrintln(3, "Content-Type: multipart/byteranges; boundary="+mWriter.Boundary())
											for _, currentPart := range httpRangeParts {
												mimePart, err := mWriter.CreatePart(textproto.MIMEHeader{
													"Content-Range": {currentPart.ToField(fsSize)},
													"Content-Type":  {theMimeType},
												})
												utils.LogPrintln(3, "Content-Range: "+currentPart.ToField(fsSize))
												utils.LogPrintln(3, "Content-Type: "+theMimeType)
												utils.LogPrintln(4, "Part Start")
												if err != nil {
													utils.LogPrintln(1, "Internal Error: "+err.Error())
													break
												}
												err = zone.Backend.WriteDataRange(lookupPath, mimePart, currentPart.Start, currentPart.Length)
												if err != nil {
													utils.LogPrintln(1, "Internal Error: "+err.Error())
													break
												}
												utils.LogPrintln(4, "Part End")
											}
											err := mWriter.Close()
											if err != nil {
												utils.LogPrintln(1, "Internal Error: "+err.Error())
											} else {
												utils.LogPrintln(4, "Send Complete")
											}
										}
									}
								}
							} else {
								processSupportedPreconditions200(rw, req, fsMod, theETag, zone.Config.CacheResponse.NotModifiedResponseUsingLastModified, zone.Config.CacheResponse.NotModifiedResponseUsingETags)
							}
							if zone.Config.CacheResponse.RequestLimitedCacheCheck {
								zone.mutPathAttr.Lock()
								if zone.PathAttributes[lookupPath] == nil {
									zone.PathAttributes[lookupPath] = NewZonePathAttributes(fsMod, theETag)
								} else {
									zone.PathAttributes[lookupPath].Update(fsMod, theETag, rw.Header())
								}
								zone.mutPathAttr.Unlock()
							}
						} else {
							utils.SwitchToNonCachingHeaders(rw.Header())
							writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusForbidden, "")
						}
					}
				} else {
					utils.SetNeverCacheHeader(rw.Header())
					writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusInternalServerError, "Stat Failure: "+err.Error())
				}
			}
		}
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
