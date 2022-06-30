package web

import (
	"github.com/tomasen/realip"
	"log"
	"net/http"
	"strconv"
)

func writeResponseHeaderCanWriteBody(minLevel uint, method string, rw http.ResponseWriter, statusCode int, message string) bool {
	hasBody := method != http.MethodHead && method != http.MethodOptions
	if hasBody && message != "" {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.Header().Set("X-Content-Type-Options", "nosniff")
		rw.Header().Set("Content-Length", strconv.Itoa(len(message)+2))
	}
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

func logRequest(req *http.Request) {
	if LogLevel < 2 {
		logPrintln(1, req.Method+" "+req.RequestURI)
	} else {
		logPrintln(2, req.Method+" "+req.RequestURI+" "+req.Proto)
		logPrintln(2, "Host: "+req.Host)
		logPrintln(2, "Client Address: "+realip.FromRequest(req))
	}
	logHeaders(req.Header)
}

func logHeaders(headers http.Header) {
	if LogLevel >= 3 {
		for k := range headers {
			log.Println("[Http] [Header] " + k + ": " + headers.Get(k))
		}
	}
}

func logPrintln(minLevel uint, toLog string) {
	if LogLevel >= minLevel {
		log.Println("[Http] " + toLog)
	}
}
