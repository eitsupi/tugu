package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var version = "dev"

func main() {
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: tugu <listen-address> <connect-address>\n\n")
		fmt.Fprintf(os.Stderr, "A cross-platform IPC socket bridge.\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  tugu tcp://127.0.0.1:8080 unix:///tmp/app.sock\n")
		fmt.Fprintf(os.Stderr, "  tugu unix:///tmp/proxy.sock tcp://127.0.0.1:9090\n")
		fmt.Fprintf(os.Stderr, "  tugu npipe:////./pipe/myapp tcp://127.0.0.1:8080\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVersion {
		fmt.Println("tugu", version)
		return
	}

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(args[0], args[1], *verbose); err != nil {
		log.Fatal(err)
	}
}

func run(listenAddr, connectAddr string, verbose bool) error {
	listener, err := resolveListener(listenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", listenAddr, err)
	}
	defer listener.Close()

	// Clean up unix socket file on exit.
	if listener.Addr().Network() == "unix" {
		defer os.Remove(listener.Addr().String())
	}

	dial, err := resolveDialer(connectAddr)
	if err != nil {
		return fmt.Errorf("connect %s: %w", connectAddr, err)
	}

	log.Printf("listening on %s, connecting to %s", listenAddr, connectAddr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return bridge(ctx, listener, dial, verbose)
}
