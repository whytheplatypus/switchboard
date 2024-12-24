package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/crypto/ssh"
)

type server struct {
	Addr    string
	Handler http.Handler
	CertDir string
	Domains []string
}

func TLSConfig(certDir string, domains ...string) (*tls.Config, error) {
	if certDir == "" || domains == nil {
		return nil, nil
	}
	m := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
	}

	m.HostPolicy = autocert.HostWhitelist(domains...)

	if err := os.MkdirAll(certDir, os.ModePerm); err != nil {
		return nil, err
	}
	m.Cache = autocert.DirCache(certDir)
	return m.TLSConfig(), nil
}

func BannerDisplayStderr() ssh.BannerCallback {
	return func(banner string) error {
		log.Println(banner)
		return nil
	}
}

func SSHListener(ctx context.Context, username string, addr string, Laddr string, auth ...ssh.AuthMethod) (net.Listener, error) {
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		BannerCallback:  BannerDisplayStderr(),
	}
	// Dial your ssh server.
	conn, err := ssh.Dial("tcp", addr, config)
	go deferContext(ctx, conn.Close)

	log.Println("setting up listening")

	// Request the remote side to open port 8080 on all interfaces.
	l, err := conn.Listen("tcp", ":80")
	if err != nil {
		return l, err
	}
	log.Println(l.Addr())
	go deferContext(ctx, l.Close)
	return l, nil
}

func deferContext[T any](c context.Context, f func() T) {
	<-c.Done()
	f()
}
