package cdn

import (
	"mime/multipart"
	"net/http"
	"net/textproto"
	"snow.mrmelon54.xyz/snowedin/cdn/utils"
	"strconv"
	"strings"
	"time"
)

func processSupportedPreconditionsForNext(rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	return processSupportedPreconditions(0, "", rw, req, modT, etag, noBypassModify, noBypassMatch)
}

func processSupportedPreconditions200(rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	return processSupportedPreconditions(http.StatusOK, "", rw, req, modT, etag, noBypassModify, noBypassMatch)
}

func processSupportedPreconditions429(rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	return processSupportedPreconditions(http.StatusTooManyRequests, "Too Many Requests", rw, req, modT, etag, noBypassModify, noBypassMatch)
}

func processSupportedPreconditions(statusCode int, statusMessage string, rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, noBypassModify bool, noBypassMatch bool) bool {
	theStrippedETag := utils.GetETagValue(etag)
	if noBypassMatch && theStrippedETag != "" && req.Header.Get("If-None-Match") != "" {
		eTagValues := utils.GetETagValues(req.Header.Get("If-None-Match"))
		conditionSuccess := false
		for _, s := range eTagValues {
			if s == theStrippedETag {
				conditionSuccess = true
				break
			}
		}
		if conditionSuccess {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotModified, "")
			utils.LogPrintln(4, "Send Skipped")
			return false
		}
	}

	if noBypassMatch && theStrippedETag != "" && req.Header.Get("If-Match") != "" {
		eTagValues := utils.GetETagValues(req.Header.Get("If-Match"))
		conditionFailed := true
		for _, s := range eTagValues {
			if s == theStrippedETag {
				conditionFailed = false
				break
			}
		}
		if conditionFailed {
			utils.SwitchToNonCachingHeaders(rw.Header())
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusPreconditionFailed, "")
			utils.LogPrintln(4, "Send Condition Not Satisfied")
			return false
		}
	}

	if noBypassModify && !modT.IsZero() && req.Header.Get("If-Modified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Modified-Since"))
		if err == nil && modT.Before(parse) || strings.EqualFold(modT.Format(http.TimeFormat), req.Header.Get("If-Modified-Since")) {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotModified, "")
			utils.LogPrintln(4, "Send Skipped")
			return false
		}
	}

	if noBypassModify && !modT.IsZero() && req.Header.Get("If-Unmodified-Since") != "" {
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Unmodified-Since"))
		if err == nil && modT.After(parse) {
			utils.SwitchToNonCachingHeaders(rw.Header())
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusPreconditionFailed, "")
			utils.LogPrintln(4, "Send Condition Not Satisfied")
			return false
		}
	}

	if statusCode >= 100 {
		return writeResponseHeaderCanWriteBody(2, req.Method, rw, statusCode, statusMessage)
	} else {
		return true
	}
}

func processRangePreconditions(maxLength int64, rw http.ResponseWriter, req *http.Request, modT time.Time, etag string, supported bool) []utils.ContentRangeValue {
	canDoRange := supported
	theStrippedETag := utils.GetETagValue(etag)
	modTStr := modT.Format(http.TimeFormat)

	if canDoRange {
		rw.Header().Set("Accept-Ranges", "bytes")
	}

	if canDoRange && !modT.IsZero() && strings.HasSuffix(req.Header.Get("If-Range"), "GMT") {
		newModT, err := time.Parse(http.TimeFormat, modTStr)
		parse, err := time.Parse(http.TimeFormat, req.Header.Get("If-Range"))
		if err == nil && !newModT.Equal(parse) {
			canDoRange = false
		}
	} else if canDoRange && theStrippedETag != "" && req.Header.Get("If-Range") != "" {
		if utils.GetETagValue(req.Header.Get("If-Range")) != theStrippedETag {
			canDoRange = false
		}
	}

	if canDoRange && strings.HasPrefix(req.Header.Get("Range"), "bytes=") {
		if theRanges := utils.GetRanges(req.Header.Get("Range"), maxLength); len(theRanges) != 0 {
			if len(theRanges) == 1 {
				rw.Header().Set("Content-Length", strconv.FormatInt(theRanges[0].Length, 10))
				rw.Header().Set("Content-Range", theRanges[0].ToField(maxLength))
			} else {
				theSize := getMultipartLength(theRanges, rw.Header().Get("Content-Type"), maxLength)
				rw.Header().Set("Content-Length", strconv.FormatInt(theSize, 10))
			}
			if writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusPartialContent, "") {
				return theRanges
			} else {
				return nil
			}
		} else {
			utils.SwitchToNonCachingHeaders(rw.Header())
			rw.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(maxLength, 10))
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusRequestedRangeNotSatisfiable, "")
			utils.LogPrintln(4, "Requested Range Not Satisfiable")
			return nil
		}
	}
	if writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "") {
		return make([]utils.ContentRangeValue, 0)
	}
	return nil
}

func getMultipartLength(parts []utils.ContentRangeValue, contentType string, maxLength int64) int64 {
	cWriter := &utils.CountingWriter{Length: 0}
	var returnLength int64 = 0
	mWriter := multipart.NewWriter(cWriter)
	for _, currentPart := range parts {
		_, _ = mWriter.CreatePart(textproto.MIMEHeader{
			"Content-Range": {currentPart.ToField(maxLength)},
			"Content-Type":  {contentType},
		})
		returnLength += currentPart.Length
	}
	_ = mWriter.Close()
	returnLength += cWriter.Length
	return returnLength
}

func writeResponseHeaderCanWriteBody(minLevel uint, method string, rw http.ResponseWriter, statusCode int, message string) bool {
	hasBody := method != http.MethodHead && method != http.MethodOptions
	if hasBody && message != "" {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Content-Length", strconv.Itoa(len(message)+2))
	}
	utils.LogHeaders(rw.Header())
	rw.WriteHeader(statusCode)
	if hasBody {
		if message != "" {
			_, _ = rw.Write([]byte(message + "\r\n"))
			utils.LogPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode)+" : "+message)
			return false
		}
		utils.LogPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode))
		return true
	}
	utils.LogPrintln(minLevel, strconv.Itoa(statusCode)+" "+http.StatusText(statusCode))
	return false
}
