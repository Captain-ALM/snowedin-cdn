package cdn

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func processIfModSince(rw http.ResponseWriter, req *http.Request, modT time.Time, noBypass bool) bool {
	if noBypass && !modT.IsZero() && req.Header.Get("If-Modified-Since") != "" {
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
	if noBypass && !modT.IsZero() && req.Header.Get("If-Unmodified-Since") != "" {
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
