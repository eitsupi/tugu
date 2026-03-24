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
		w := os.Stderr
		fmt.Fprintf(w, `Usage: tugu <listen-address> <connect-address>

A cross-platform IPC socket bridge. Listens on the first address, connects
to the second, and copies data bidirectionally for each incoming connection.

Address formats:

  tcp://host:port          TCP socket (all platforms)
                           e.g. tcp://127.0.0.1:8080
                                tcp://localhost:9090
                                tcp://[::1]:80

  unix:///path             Unix domain socket (all platforms)
                           e.g. unix:///tmp/app.sock
                                unix:///var/run/app.sock
                                unix:///C:/tmp/app.sock  (Windows)
                           unix://localhost/path is also accepted.

  npipe:////./pipe/name    Windows named pipe (Windows only)
                           e.g. npipe:////./pipe/docker_engine
                                npipe:////./pipe/myapp
                           npipe://./pipe/name (Docker.DotNet style) is
                           also accepted.

Examples:

  # Expose a Windows named pipe as a TCP port
  tugu npipe:////./pipe/myapp tcp://127.0.0.1:8080

  # Bridge TCP to a Unix domain socket
  tugu tcp://127.0.0.1:3000 unix:///var/run/app.sock

  # Bridge a Unix domain socket to TCP
  tugu unix:///tmp/proxy.sock tcp://127.0.0.1:9090

  # Chain two transports via a Unix domain socket on Windows
  tugu npipe:////./pipe/myapp unix:///C:/tmp/bridge.sock
  tugu unix:///C:/tmp/bridge.sock tcp://127.0.0.1:8080

Flags:

`)
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
