//go:build windows

package main

import (
	"context"
	"net"

	"github.com/Microsoft/go-winio"
)

func listenPipe(path string) (net.Listener, error) {
	return winio.ListenPipe(path, nil)
}

func dialPipe(ctx context.Context, path string) (net.Conn, error) {
	// TODO: respect ctx cancellation
	return winio.DialPipeContext(ctx, path)
}
