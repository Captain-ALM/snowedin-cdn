package web

import (
	"github.com/gorilla/mux"
	"github.com/tomasen/realip"
	"log"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/cdn"
	"strings"
)

var LogLevel uint = 0

func New(cdnIn *cdn.CDN) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/", zoneNotProvided)
	router.HandleFunc("/{zone}", pathNotProvided)
	router.HandleFunc("/{zone}/", func(rw http.ResponseWriter, req *http.Request) {
		zoneHandlerFunc(rw, req, cdnIn)
	})
	router.PathPrefix("/{zone}/").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		zoneHandlerFunc(rw, req, cdnIn)
	})
	if cdnIn.Config.Listen.Web == "" {
		log.Fatalf("[Http] Invalid Listening Address")
	}
	s := &http.Server{
		Addr:         cdnIn.Config.Listen.Web,
		Handler:      router,
		ReadTimeout:  cdnIn.Config.Listen.GetReadTimeout(),
		WriteTimeout: cdnIn.Config.Listen.GetWriteTimeout(),
	}
	LogLevel = cdnIn.Config.LogLevel
	go runBackgroundHttp(s)
	return s
}

func zoneHandlerFunc(rw http.ResponseWriter, req *http.Request, cdnIn *cdn.CDN) {
	logRequest(req)
	if req.Method == http.MethodGet || req.Method == http.MethodDelete {
		vars := mux.Vars(req)
		var otherZone *cdn.Zone
		var targetZone *cdn.Zone
		for _, z := range cdnIn.Zones {
			if z == nil {
				continue
			}
			if z.Config.Name == "" && z.ZoneHostAllowed(req.Host) {
				otherZone = z
				continue
			}
			if strings.EqualFold(vars["zone"], z.Config.Name) && z.ZoneHostAllowed(req.Host) {
				targetZone = z
				break
			}
		}
		if targetZone == nil && otherZone == nil {
			http.Error(rw, "Zone Not Found", http.StatusNotFound)
			logPrintln(2, "404 Not Found\nZone Not Found")
			return
		} else if targetZone == nil && otherZone != nil {
			targetZone = otherZone
		}
		targetZone.ZoneHandleRequest(rw, req)
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet+", "+http.MethodDelete)
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			logPrintln(2, "200 OK")
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			logPrintln(2, "405 Method Not Allowed")
		}
	}
}

func zoneNotProvided(rw http.ResponseWriter, req *http.Request) {
	logRequest(req)
	if req.Method == http.MethodGet {
		http.Error(rw, "Zone Not Provided", http.StatusNotFound)
		logPrintln(1, "404 Not Found\nZone Not Provided")
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet)
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			logPrintln(2, "200 OK")
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			logPrintln(2, "405 Method Not Allowed")
		}
	}
}

func pathNotProvided(rw http.ResponseWriter, req *http.Request) {
	logRequest(req)
	if req.Method == http.MethodGet {
		http.Error(rw, "Path Not Provided", http.StatusNotFound)
		logPrintln(1, "404 Not Found\nPath Not Provided")
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet)
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
			logPrintln(2, "200 OK")
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			logPrintln(2, "405 Method Not Allowed")
		}
	}
}

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
}

func logPrintln(minLevel uint, toLog string) {
	if LogLevel >= minLevel {
		log.Println("[Http] " + toLog)
	}
}
