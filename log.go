package main

import (
	"io"
	"log"
	"net/http"

	"github.com/whytheplatypus/flushable"
)

func configureLog(addr string) {
	m := &flushable.MultiFlusher{}
	log.SetOutput(io.MultiWriter(m, log.Writer()))
	go func() {
		if err := http.ListenAndServe(addr, m); err != nil {
			log.Println(err)
		}
	}()
}
