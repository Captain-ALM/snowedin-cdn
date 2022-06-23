package web

import (
	"github.com/gorilla/mux"
	"github.com/tomasen/realip"
	"log"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/cdn"
)

func New(cdn *cdn.CDN) *http.Server {
	router := mux.NewRouter()
	router.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		clientIP := realip.FromRequest(req)
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte(clientIP))
	})
	s := &http.Server{
		Addr:         cdn.Config.Listen.Web,
		Handler:      router,
		ReadTimeout:  cdn.Config.Listen.GetReadTimeout(),
		WriteTimeout: cdn.Config.Listen.GetWriteTimeout(),
	}
	go runBackgroundHttp(s)
	return s
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
