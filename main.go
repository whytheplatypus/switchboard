package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var cmds = map[string]func(args []string, ctx context.Context){
	"hookup": hookup,
	"route":  route,
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		for key := range cmds {
			fmt.Fprintln(flag.CommandLine.Output(), key)
		}
		os.Exit(2)
	}
	cmd, ok := cmds[args[0]]
	if !ok {
		flag.Usage()
		for key := range cmds {
			fmt.Fprintln(flag.CommandLine.Output(), key)
		}
		os.Exit(2)
	}
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		waitFor(syscall.SIGINT, syscall.SIGTERM)
		cancel()
	}()

	cmd(args[1:], ctx)
}

func waitFor(calls ...os.Signal) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, calls...)
	<-sigs
}
