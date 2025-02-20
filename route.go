package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"

	"github.com/whytheplatypus/switchboard/operator"
	"golang.org/x/crypto/ssh"
)

func route(args []string, ctx context.Context) {
	flags := flag.NewFlagSet("route", flag.ExitOnError)
	addr := flags.String("addr", ":80", "the port this should run on")
	cdir := flags.String("cert-directory", "/var/cache/switchboard/autocert", "the directory to store the acme cert")
	sshAddr := flags.String("ssh", "", "the address of a remote server to use as an ssh reverse proxy")
	sshUsername := flags.String("ssh-username", "", "the username for the remote server")
	sshPassword := flags.String("ssh-password", "", "the password for the remote server")
	var domains StringArray
	flags.Var(&domains, "domain", "a domain to register a tls cert for")
	httpLog := flags.String("log-http", "", "The address to serve logs over, no logs are served if empty")
	flags.Parse(args)

	if *httpLog != "" {
		configureLog(*httpLog)
	}

	go func() {
		entries := operator.Listen(ctx)
		for entry := range entries {
			if err := operator.Connect(entry); err != nil {
				registrationLog.Println(err)
				continue
			}
			// register
			registrationLog.Printf(`{"send":"%s","to":"http://%s:%d"}`,
				entry.InfoFields[0],
				entry.AddrV4,
				entry.Port,
			)
		}
	}()

	router := operator.Handler()

	/*
		http.ListenAndServe(":8080", http.HandlerFunc(
			func(rw http.ResponseWriter, r *http.Request) {
				rw.(http.Hijacker).Hijack()
				log.Println("DID IT!!")
				return
			}))
	*/
	/*
		router.ModifyResponse = func(r *http.Response) error {
			info := struct {
				Host   string `json:"host"`
				Target string `json:"target"`
				Path   string `json:"path"`
				Query  string `json:"query"`
			}{
				r.Request.Host,
				r.Request.URL.Host,
				r.Request.URL.Path,
				r.Request.URL.RawQuery,
			}

			b, _ := json.Marshal(info)
			routingLog.Println(r.Request.URL.Query())
			routingLog.Println(string(b))
			return nil
		}
	*/
	h := http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		rw.(http.Hijacker).Hijack()
		log.Println("DID IT!!")
		return
		defer func() {
			if err := recover(); err != nil {
				http.NotFound(rw, r)
			}
		}()
		router.ServeHTTP(rw, r)
	})
	var l net.Listener
	if *sshAddr != "" {
		var err error
		auth := []ssh.AuthMethod{}
		if *sshPassword != "" {
			auth = append(auth, ssh.Password(*sshPassword))
		}
		l, err = SSHListener(ctx, *sshUsername, *sshAddr, *addr, auth...)
		if err != nil {
			routingLog.Fatal(err)
		}
		log.Println("SSH Listener created")
	}

	if l == nil {
		var err error
		l, err = net.Listen("tcp", *addr)
		if err != nil {
			routingLog.Fatal(err)
		}
		log.Println("Standard Listener created")
	}

	TLSConfig, err := TLSConfig(*cdir, domains...)
	if err != nil {
		routingLog.Fatal(err)
	}
	if TLSConfig != nil {
		//l = tls.NewListener(l, TLSConfig)
		log.Println("TLS Listener created")
	}

	srv := &http.Server{
		//Addr:    ":8080",
		//Handler: handlers.LoggingHandler(&lWriter{accessLog}, h),
		Handler: h,
	}

	shutdownCtx, cancel := context.WithCancel(context.Background())
	go deferContext(ctx, func() error {
		srv.Shutdown(context.Background())
		cancel()
		return nil
	})

	log.Println("Switchboard server starting")
	if err := srv.Serve(l); err != nil {
		routingLog.Fatal(err)
	}
	<-shutdownCtx.Done()
}
