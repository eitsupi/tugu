package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type transportDef struct {
	name   string
	listen func(t *testing.T) net.Listener
	dial   func(t *testing.T, addr string) func(ctx context.Context) (net.Conn, error)
}

// extraTransports is populated by platform-specific init() functions (e.g., bridge_test_windows.go).
var extraTransports []transportDef

func TestBridge(t *testing.T) {
	transports := []transportDef{
		{
			name: "tcp",
			listen: func(t *testing.T) net.Listener {
				t.Helper()
				l, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				return l
			},
			dial: func(t *testing.T, addr string) func(ctx context.Context) (net.Conn, error) {
				t.Helper()
				return func(ctx context.Context) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "tcp", addr)
				}
			},
		},
		{
			name: "unix",
			listen: func(t *testing.T) net.Listener {
				t.Helper()
				sock := filepath.Join(t.TempDir(), "test.sock")
				l, err := net.Listen("unix", sock)
				if err != nil {
					t.Fatal(err)
				}
				return l
			},
			dial: func(t *testing.T, addr string) func(ctx context.Context) (net.Conn, error) {
				t.Helper()
				return func(ctx context.Context) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", addr)
				}
			},
		},
	}
	transports = append(transports, extraTransports...)

	for _, listen := range transports {
		for _, connect := range transports {
			t.Run(fmt.Sprintf("%s-to-%s", listen.name, connect.name), func(t *testing.T) {
				t.Run("echo", func(t *testing.T) {
					testBridgeEcho(t, listen.listen, connect.listen, connect.dial)
				})
				t.Run("concurrent", func(t *testing.T) {
					testBridgeConcurrent(t, listen.listen, connect.listen, connect.dial)
				})
				t.Run("half-close", func(t *testing.T) {
					testBridgeHalfClose(t, listen.listen, connect.listen, connect.dial)
				})
			})
		}
	}
}

// echoServer accepts one connection and echoes data back.
func echoServer(t *testing.T, l net.Listener) {
	t.Helper()
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				io.Copy(conn, conn)
			}()
		}
	}()
}

func testBridgeEcho(
	t *testing.T,
	makeListen func(*testing.T) net.Listener,
	makeBackend func(*testing.T) net.Listener,
	makeDial func(*testing.T, string) func(context.Context) (net.Conn, error),
) {
	t.Helper()

	// Start echo backend.
	backend := makeBackend(t)
	defer backend.Close()
	echoServer(t, backend)

	// Start bridge.
	front := makeListen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridge(ctx, front, makeDial(t, backend.Addr().String()), false)

	// Connect through the bridge.
	conn, err := net.Dial(front.Addr().Network(), front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	msg := "hello tugu"
	if _, err := conn.Write([]byte(msg)); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q, want %q", buf, msg)
	}
}

func testBridgeConcurrent(
	t *testing.T,
	makeListen func(*testing.T) net.Listener,
	makeBackend func(*testing.T) net.Listener,
	makeDial func(*testing.T, string) func(context.Context) (net.Conn, error),
) {
	t.Helper()

	backend := makeBackend(t)
	defer backend.Close()
	echoServer(t, backend)

	front := makeListen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridge(ctx, front, makeDial(t, backend.Addr().String()), false)

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	for i := range n {
		go func() {
			defer wg.Done()
			conn, err := net.Dial(front.Addr().Network(), front.Addr().String())
			if err != nil {
				t.Error(err)
				return
			}
			defer conn.Close()

			msg := fmt.Sprintf("conn-%d", i)
			conn.Write([]byte(msg))
			buf := make([]byte, len(msg))
			io.ReadFull(conn, buf)
			if string(buf) != msg {
				t.Errorf("conn %d: got %q, want %q", i, buf, msg)
			}
		}()
	}

	wg.Wait()
}

func testBridgeHalfClose(
	t *testing.T,
	makeListen func(*testing.T) net.Listener,
	makeBackend func(*testing.T) net.Listener,
	makeDial func(*testing.T, string) func(context.Context) (net.Conn, error),
) {
	t.Helper()

	// Backend that sends data then closes.
	backend := makeBackend(t)
	defer backend.Close()

	go func() {
		conn, err := backend.Accept()
		if err != nil {
			return
		}
		conn.Write([]byte("goodbye"))
		conn.Close()
	}()

	front := makeListen(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go bridge(ctx, front, makeDial(t, backend.Addr().String()), false)

	conn, err := net.Dial(front.Addr().Network(), front.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	data, err := io.ReadAll(conn)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "goodbye" {
		t.Fatalf("got %q, want %q", data, "goodbye")
	}
}

func TestResolveAddr(t *testing.T) {
	tests := []struct {
		input    string
		scheme   string
		host     string
		path     string
		pipe     string
		wantErr  bool
	}{
		// TCP
		{input: "tcp://127.0.0.1:8080", scheme: "tcp", host: "127.0.0.1:8080"},
		{input: "tcp://localhost:9090", scheme: "tcp", host: "localhost:9090"},
		{input: "tcp://[::1]:80", scheme: "tcp", host: "[::1]:80"},
		{input: "tcp://noport", scheme: "tcp", host: "noport"},

		// Unix
		{input: "unix:///tmp/app.sock", scheme: "unix", path: "/tmp/app.sock"},
		{input: "unix://localhost/tmp/app.sock", scheme: "unix", path: "/tmp/app.sock"},

		// npipe (Docker standard)
		{input: "npipe:////./pipe/docker_engine", scheme: "npipe", pipe: `\\.\pipe\docker_engine`},
		// npipe (Docker.DotNet style)
		{input: "npipe://./pipe/myapp", scheme: "npipe", pipe: `\\.\pipe\myapp`},

		// Errors
		{input: "http://example.com", wantErr: true},
		{input: "unix://remotehost/path", wantErr: true},
		{input: "tcp://", wantErr: true},
		{input: "unix:///", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseAddr(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.scheme != tt.scheme {
				t.Errorf("scheme: got %q, want %q", got.scheme, tt.scheme)
			}
			if tt.host != "" && got.host != tt.host {
				t.Errorf("host: got %q, want %q", got.host, tt.host)
			}
			if tt.path != "" && got.path != tt.path {
				t.Errorf("path: got %q, want %q", got.path, tt.path)
			}
			if tt.pipe != "" && got.pipe != tt.pipe {
				t.Errorf("pipe: got %q, want %q", got.pipe, tt.pipe)
			}
		})
	}
}

func TestResolveListenerUnixCleanup(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "stale.sock")
	// Create a stale socket file.
	os.WriteFile(sock, []byte{}, 0o600)

	l, err := resolveListener("unix://" + sock)
	if err != nil {
		t.Fatal(err)
	}
	l.Close()
}
