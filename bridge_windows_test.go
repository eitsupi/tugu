//go:build windows

package main

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"

	"github.com/Microsoft/go-winio"
)

var pipeSeq atomic.Uint64

func init() {
	extraTransports = append(extraTransports, transportDef{
		name: "npipe",
		listen: func(t *testing.T) net.Listener {
			t.Helper()
			name := fmt.Sprintf(`\\.\pipe\tugu-test-%d`, pipeSeq.Add(1))
			l, err := winio.ListenPipe(name, nil)
			if err != nil {
				t.Fatal(err)
			}
			return l
		},
		dial: func(t *testing.T, addr string) func(ctx context.Context) (net.Conn, error) {
			t.Helper()
			return func(ctx context.Context) (net.Conn, error) {
				return winio.DialPipeContext(ctx, addr)
			}
		},
	})
}
