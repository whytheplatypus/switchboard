package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/whytheplatypus/flushable"
)

var (
	accessLog       = log.New(os.Stderr, "[access] ", log.LstdFlags)
	registrationLog = log.New(os.Stderr, "[registration] ", log.LstdFlags)
	routingLog      = log.New(os.Stderr, "[routing] ", log.LstdFlags)
)

func configureLog(addr string) {
	go func() {
		r := http.NewServeMux()
		r.Handle("/debug/access", logHandler(accessLog))
		r.Handle("/debug/registration", logHandler(registrationLog))
		r.Handle("/debug/routing", logHandler(routingLog))
		if err := http.ListenAndServe(addr, r); err != nil {
			log.Println(err)
		}
	}()
}

func logHandler(l *log.Logger) http.Handler {
	m := &flushable.MultiFlusher{}
	l.SetOutput(io.MultiWriter(m, l.Writer()))
	return m
}

type lWriter struct {
	*log.Logger
}

func (lw *lWriter) Write(p []byte) (n int, err error) {
	err = lw.Output(2, string(p))
	return len(p), err
}
