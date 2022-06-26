package web

import (
	"github.com/gorilla/mux"
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
	if cdnIn.Config.Listen.Identify {
		router.Use(headerMiddleware)
	}
	if cdnIn.Config.Listen.Web == "" {
		log.Fatalf("[Http] Invalid Listening Address")
	}
	s := &http.Server{
		Addr:         cdnIn.Config.Listen.Web,
		Handler:      router,
		ReadTimeout:  cdnIn.Config.Listen.GetReadTimeout(),
		WriteTimeout: cdnIn.Config.Listen.GetWriteTimeout(),
		IdleTimeout:  cdnIn.Config.Listen.GetIdleTimeout(),
	}
	LogLevel = cdnIn.Config.LogLevel
	go runBackgroundHttp(s)
	return s
}

func zoneHandlerFunc(rw http.ResponseWriter, req *http.Request, cdnIn *cdn.CDN) {
	logRequest(req)
	if req.Method == http.MethodGet || req.Method == http.MethodHead || req.Method == http.MethodDelete {
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
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusNotFound, "Zone Not Found")
			return
		} else if targetZone == nil && otherZone != nil {
			targetZone = otherZone
		}
		targetZone.ZoneHandleRequest(rw, req)
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet+", "+http.MethodHead+", "+http.MethodDelete)
		if req.Method == http.MethodOptions {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "")
		} else {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusMethodNotAllowed, "")
		}
	}
}

func zoneNotProvided(rw http.ResponseWriter, req *http.Request) {
	logRequest(req)
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusNotFound, "Zone Not Provided")
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet+", "+http.MethodHead)
		if req.Method == http.MethodOptions {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "")
		} else {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusMethodNotAllowed, "")
		}
	}
}

func pathNotProvided(rw http.ResponseWriter, req *http.Request) {
	logRequest(req)
	if req.Method == http.MethodGet || req.Method == http.MethodHead {
		writeResponseHeaderCanWriteBody(1, req.Method, rw, http.StatusNotFound, "Path Not Provided")
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet+", "+http.MethodHead)
		if req.Method == http.MethodOptions {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusOK, "")
		} else {
			writeResponseHeaderCanWriteBody(2, req.Method, rw, http.StatusMethodNotAllowed, "")
		}
	}
}

func headerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "Clerie Gilbert")
		w.Header().Set("X-Powered-By", "Love")
		w.Header().Set("X-Friendly", "True")
		next.ServeHTTP(w, r)
	})
}
