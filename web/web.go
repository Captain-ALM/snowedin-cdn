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
			rw.WriteHeader(http.StatusNotFound)
			_, _ = rw.Write([]byte("Zone Not Found"))
			return
		} else if targetZone == nil && otherZone != nil {
			targetZone = otherZone
		}
		targetZone.ZoneHandleRequest(rw, req)
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
	rw.WriteHeader(http.StatusNotFound)
	_, _ = rw.Write([]byte("Zone Not Provided"))
}

func pathNotProvided(rw http.ResponseWriter, req *http.Request) {
	rw.WriteHeader(http.StatusNotFound)
	_, _ = rw.Write([]byte("Path Not Provided"))
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
