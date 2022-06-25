package web

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/cdn"
	"strings"
)

func New(cdnIn *cdn.CDN) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/", zoneNotProvided)
	router.HandleFunc("/{zone}", pathNotProvided)
	router.HandleFunc("/{zone}/", pathNotProvided)
	router.PathPrefix("/{zone}/").HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
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
				rw.Header().Set("Cache-Control", "max-age=0, no-cache, no-store, must-revalidate")
				http.Error(rw, "Zone Not Found", http.StatusNotFound)
				return
			} else if targetZone == nil && otherZone != nil {
				targetZone = otherZone
			}
			targetZone.ZoneHandleRequest(rw, req)
		} else {
			rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet+", "+http.MethodDelete)
			if req.Method == http.MethodOptions {
				rw.WriteHeader(http.StatusOK)
			} else {
				rw.WriteHeader(http.StatusMethodNotAllowed)
			}
		}
	})
	s := &http.Server{
		Addr:         cdnIn.Config.Listen.Web,
		Handler:      router,
		ReadTimeout:  cdnIn.Config.Listen.GetReadTimeout(),
		WriteTimeout: cdnIn.Config.Listen.GetWriteTimeout(),
	}
	go runBackgroundHttp(s)
	return s
}

func zoneNotProvided(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		http.Error(rw, "Zone Not Provided", http.StatusNotFound)
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet)
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func pathNotProvided(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodGet {
		http.Error(rw, "Path Not Provided", http.StatusNotFound)
	} else {
		rw.Header().Set("Allow", http.MethodOptions+", "+http.MethodGet)
		if req.Method == http.MethodOptions {
			rw.WriteHeader(http.StatusOK)
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func runBackgroundHttp(s *http.Server) {
	err := s.ListenAndServe()
	if err != nil {
		if err == http.ErrServerClosed {
			log.Println("[Http] The http server shutdown successfully")
		} else {
			log.Printf("[Http] Error trying to host the http server: %s\n", err.Error())
		}
	}
}
