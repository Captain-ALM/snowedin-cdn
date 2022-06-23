package api

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/cdn"
)

func New(cdn *cdn.CDN) *http.Server {
	router := mux.NewRouter()
	s := &http.Server{
		Addr:         cdn.Config.Listen.Api,
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
