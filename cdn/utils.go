package cdn

import (
	"crypto"
	"encoding/hex"
	"log"
	"mime"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/structure"
	"strconv"
	"strings"
	"time"
)

func processSupportedPreconditions200(rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	return processSupportedPreconditions(http.StatusOK, "", rw, req, modT, etag, noBypassModify, noBypassMatch)
}

func processSupportedPreconditions429(rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	return processSupportedPreconditions(http.StatusTooManyRequests, "Too Many Requests", rw, req, modT, etag, noBypassModify, noBypassMatch)
}

func processSupportedPreconditions(statusCode int, statusMessage string, rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	theStrippedETag := getETagValue(etag)
	if noBypassMatch && theStrippedETag != "" && req.Header.Get("If-None-Match") != "" {
		etagVals := getETagValues(req.Header.Get("If-None-Match"))
		conditionSuccess := false
		for _, s := range etagVals {
			if s == theStrippedETag {
				conditionSuccess = true
				break
			}
		}
		if conditionSuccess {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotModified, "")
			logPrintln(4, "Send Skipped")
			return false
		}
	}

	if noBypassMatch && theStrippedETag != "" && req.Header.Get("If-Match") != "" {
		etagVals := getETagValues(req.Header.Get("If-Match"))
		conditionFailed := true
		for _, s := range etagVals {
			if s == theStrippedETag {
				conditionFailed = false
				break
			}
		}
		if conditionFailed {
			switchToNonCachingHeaders(rw.Header())
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusPreconditionFailed, "")
			logPrintln(4, "Send Condition Not Satisfied")
			return false
		}
	}

	if noBypassModify && !modT.IsZero() && req.Header.Get("If-Modified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Modified-Since"))
		if err == nil {
			if modT.Before(parse) || strings.EqualFold(modT.Format(http.TimeFormat), req.Header.Get("If-Modified-Since")) {
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotModified, "")
				logPrintln(4, "Send Skipped")
				return false
			}
		}
	}

	if noBypassModify && !modT.IsZero() && req.Header.Get("If-Unmodified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Unmodified-Since"))
		if err == nil {
			if modT.After(parse) {
				switchToNonCachingHeaders(rw.Header())
				writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusPreconditionFailed, "")
				logPrintln(4, "Send Condition Not Satisfied")
				return false
			}
		}
	}

	return writeResponseHeaderCanWriteBody(2, req.Method, rw, statusCode, statusMessage)
}

func getValueForETagUsingAttributes(timeIn time.Time, sizeIn int64) string {
	theHash := crypto.SHA1.New()
	theValue := timeIn.Format(http.TimeFormat) + ":" + strconv.FormatInt(sizeIn, 10)
	theSum := theHash.Sum([]byte(theValue))
	theHash.Reset()
	if len(theSum) > 0 {
		return "\"" + hex.EncodeToString(theSum) + "\""
	} else {
		return "\"" + hex.EncodeToString([]byte(theValue)) + "\""
	}
}

func getETagValues(stringIn string) []string {
	if strings.ContainsAny(stringIn, ",") {
		seperated := strings.Split(stringIn, ",")
		toReturn := make([]string, len(seperated))
		pos := 0
		for _, s := range seperated {
			cETag := getETagValue(s)
			if cETag != "" {
				toReturn[pos] = cETag
				pos += 1
			}
		}
		if pos == 0 {
			return nil
		}
		return toReturn[:pos]
	}
	toReturn := []string{getETagValue(stringIn)}
	if toReturn[0] == "" {
		return nil
	}
	return toReturn
}

func getETagValue(stringIn string) string {
	startIndex := strings.IndexAny(stringIn, "\"") + 1
	endIndex := strings.LastIndexAny(stringIn, "\"")
	if endIndex > startIndex {
		return stringIn[startIndex:endIndex]
	}
	return ""
}

func writeResponseHeaderCanWriteBody(minLevel uint, method string, rw http.ResponseWriter, statusCode int, message string) bool {
	hasBody := method != http.MethodHead && method != http.MethodOptions
	if hasBody && message != "" {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Content-Length", strconv.Itoa(len(message)+2))
	}
	logHeaders(rw.Header())
	rw.WriteHeader(statusCode)
	if hasBody {
		if message != "" {
			_, _ = rw.Write([]byte(message + "\r\n"))
			logPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode)+" : "+message)
			return false
		}
		logPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode))
		return true
	}
	logPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode))
	return false
}

func setNeverCacheHeader(header http.Header) {
	header.Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
	header.Set("Pragma", "no-cache")
}

func setExpiresHeader(header http.Header, expireTime time.Time) {
	header.Set("Expires", expireTime.UTC().Format(http.TimeFormat))
}

func setLastModifiedHeader(header http.Header, modTime time.Time) {
	if !modTime.IsZero() {
		header.Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	}
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

func switchToNonCachingHeaders(header http.Header) {
	setNeverCacheHeader(header)
	if header.Get("Last-Modified") != "" {
		header.Del("Last-Modified")
	}
	if header.Get("Age") != "" {
		header.Del("Age")
	}
	if header.Get("Expires") != "" {
		header.Del("Expires")
	}
	if header.Get("ETag") != "" {
		header.Del("ETag")
	}
}

func getFilenameFromPath(pathIn string) string {
	lastSlashIndex := strings.LastIndexAny(pathIn, "/")
	if lastSlashIndex < 0 {
		return pathIn
	} else {
		return pathIn[lastSlashIndex+1:]
	}
}

func setDownloadHeaders(header http.Header, config structure.DownloadSettingsYaml, filename string, mimeType string) {
	if config.OutputFilename {
		theFilename := filename
		if theFilename == "" {
			theFilename = "download"
		}
		if exts, err := mime.ExtensionsByType(mimeType); config.SetExtensionIfMissing && !strings.Contains(theFilename, ".") && err == nil && len(exts) > 0 {
			theFilename += exts[0]
		}
		header.Set("Content-Disposition", "attachment; filename=\""+theFilename+"\"")
	} else {
		header.Set("Content-Disposition", "attachment")
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
