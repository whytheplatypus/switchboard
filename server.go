package main

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

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
	log.Println("authenticating as", username)
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		BannerCallback:  BannerDisplayStderr(),
	}
	// Dial your ssh server.
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}
	go deferContext(ctx, conn.Close)

	s, err := conn.NewSession()
	go func() {
		r, err := s.StderrPipe()
		if err != nil {
			log.Println("faile", err)
		}
		io.Copy(os.Stdout, r)
	}()
	go func() {
		r, err := s.StdoutPipe()
		if err != nil {
			log.Println("faile", err)
		}
		io.Copy(os.Stdout, r)
	}()
	if err := s.Shell(); err != nil {
		log.Println("oh no!", err)
	}

	log.Println("setting up listening")

	la, p, err := net.SplitHostPort(Laddr)
	if err != nil {
		return nil, err
	}
	pp, err := strconv.ParseInt(p, 10, 32)
	if err != nil {
		return nil, err
	}
	l, err := conn.ListenTCPHostname(la, uint32(pp))
	//l, err := conn.Listen("tcp", Laddr)
	if err != nil {
		return nil, err
	}
	log.Println(l.Addr())
	go deferContext(ctx, l.Close)
	return l, nil
}

func deferContext[T any](c context.Context, f func() T) {
	<-c.Done()
	f()
}
