package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/whytheplatypus/switchboard/client"
	"github.com/whytheplatypus/switchboard/operator"
)

var cmds = map[string]func(args []string){
	"hookup": hookup,
	"route":  route,
}

func main() {
	flag.Parse()
	args := flag.Args()
	cmd, ok := cmds[args[0]]
	if !ok {
		flag.Usage()
		for key, _ := range cmds {
			fmt.Fprintln(flag.CommandLine.Output(), key)
		}
		os.Exit(2)
	}
	cmd(args[1:])
}

type StringArray []string

func (av *StringArray) String() string {
	return ""
}

func (av *StringArray) Set(s string) error {
	*av = append(*av, s)
	return nil
}

func route(args []string) {
	flags := flag.NewFlagSet("route", flag.ExitOnError)
	port := flags.Int("port", 80, "the port this should run on")
	cdir := flags.String("cert-directory", "/var/cache/switchboard/autocert", "the directory to store the acme cert")
	var domains StringArray
	flags.Var(&domains, "domain", "a domain to register a tls cert for")
	flags.Parse(args)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go operator.Listen(ctx)

	notFound := func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Host, r.URL.Path)
		http.NotFound(w, r)
	}
	operator.Phonebook.HandleFunc("/", notFound)

	srv := &server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: operator.Phonebook,
		CertDir: *cdir,
		Domains: domains,
	}
	if err := srv.serve(ctx); err != nil {
		log.Fatal(err)
	}

	waitFor(syscall.SIGINT, syscall.SIGTERM)
	fmt.Println("exiting")
}

func hookup(args []string) {
	flags := flag.NewFlagSet("hookup", flag.ExitOnError)
	pattern := flags.String("pattern", "", "the url pattern that should forward to this service")
	port := flags.Int("port", 80, "the port the service runs on")
	flags.Parse(args)

	server := client.Hookup(*pattern, *port)
	defer server.Shutdown()
	waitFor(syscall.SIGINT, syscall.SIGTERM)
}

func waitFor(calls ...os.Signal) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, calls...)
	<-sigs
}
