package web

import (
	"github.com/tomasen/realip"
	"log"
	"net/http"
)

func runBackgroundHttp(s *http.Server) {
	err := s.ListenAndServe()
	if err != nil {
		if err == http.ErrServerClosed {
			logPrintln(0, "The http server shutdown successfully")
		} else {
			log.Fatalf("[Http] Error trying to host the http server: %s\n", err.Error())
		}
	}
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
