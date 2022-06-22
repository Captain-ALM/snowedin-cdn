package api

import (
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"snowedin/structure"
	"time"
)

func New(conf structure.ConfigYaml) *http.Server {
	router := mux.NewRouter()
	s := &http.Server{
		Addr:              conf.Listen.Api,
		Handler:           router,
		ReadTimeout:       1 * time.Minute,
		ReadHeaderTimeout: 1 * time.Minute,
		WriteTimeout:      1 * time.Minute,
		IdleTimeout:       1 * time.Minute,
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
