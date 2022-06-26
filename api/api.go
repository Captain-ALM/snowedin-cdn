package api

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"snow.mrmelon54.xyz/snowedin/cdn"
)

//var LogLevel uint = 0

func New(cdnIn *cdn.CDN) *http.Server {
	router := mux.NewRouter()
	if cdnIn.Config.Listen.Api == "" {
		log.Fatalf("[Http] Invalid Listening Address")
	}
	s := &http.Server{
		Addr:         cdnIn.Config.Listen.Api,
		Handler:      router,
		ReadTimeout:  cdnIn.Config.Listen.GetReadTimeout(),
		WriteTimeout: cdnIn.Config.Listen.GetWriteTimeout(),
		IdleTimeout:  cdnIn.Config.Listen.GetIdleTimeout(),
	}
	//LogLevel = cdnIn.Config.LogLevel
	go runBackgroundHttp(s)
	return s
}

func runBackgroundHttp(s *http.Server) {
	err := s.ListenAndServe()
	if err != nil {
		if err == http.ErrServerClosed {
			log.Println("[Http] The http server shutdown successfully")
		} else {
			log.Fatalf("[Http] Error trying to host the http server: %s\n", err.Error())
		}
	}
}
