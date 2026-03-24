//go:build !windows

package main

import (
	"context"
	"fmt"
	"net"
)

func listenPipe(_ string) (net.Listener, error) {
	return nil, fmt.Errorf("named pipes are only supported on Windows")
}

func dialPipe(_ context.Context, _ string) (net.Conn, error) {
	return nil, fmt.Errorf("named pipes are only supported on Windows")
}
