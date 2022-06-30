package utils

import (
	"log"
	"net/http"
)

var LogLevel uint = 0

func LogPrintln(minLevel uint, message string) {
	if LogLevel >= minLevel {
		log.Println("[Http] [Zone] " + message)
	}
}

func LogHeaders(headers http.Header) {
	if LogLevel >= 3 {
		for k := range headers {
			log.Println("[Http] [Zone] [Header] " + k + ": " + headers.Get(k))
		}
	}
}
