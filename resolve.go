package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
)

// resolveListener parses a URI and returns a net.Listener.
// Supported schemes: tcp, unix, npipe (Windows only).
func resolveListener(addr string) (net.Listener, error) {
	u, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}

	switch u.scheme {
	case "tcp":
		return net.Listen("tcp", u.host)
	case "unix":
		// Remove stale socket file if it exists.
		_ = os.Remove(u.path)
		return net.Listen("unix", u.path)
	case "npipe":
		return listenPipe(u.pipe)
	default:
		return nil, fmt.Errorf("unsupported scheme for listener: %s", u.scheme)
	}
}

// resolveDialer parses a URI and returns a dial function.
func resolveDialer(addr string) (func(ctx context.Context) (net.Conn, error), error) {
	u, err := parseAddr(addr)
	if err != nil {
		return nil, err
	}

	switch u.scheme {
	case "tcp":
		return func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", u.host)
		}, nil
	case "unix":
		return func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", u.path)
		}, nil
	case "npipe":
		return func(ctx context.Context) (net.Conn, error) {
			return dialPipe(ctx, u.pipe)
		}, nil
	default:
		return nil, fmt.Errorf("unsupported scheme for dialer: %s", u.scheme)
	}
}

type parsedAddr struct {
	scheme string
	host   string // for tcp
	path   string // for unix
	pipe   string // for npipe (Windows pipe path like \\.\pipe\name)
}

func parseAddr(raw string) (*parsedAddr, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", raw, err)
	}

	switch u.Scheme {
	case "tcp":
		if u.Host == "" {
			return nil, fmt.Errorf("tcp address requires host:port: %s", raw)
		}
		return &parsedAddr{scheme: "tcp", host: u.Host}, nil

	case "unix":
		if u.Host != "" && u.Host != "localhost" {
			return nil, fmt.Errorf("unix address does not support remote hosts: %s", raw)
		}
		if u.Path == "" || u.Path == "/" {
			return nil, fmt.Errorf("unix address requires a path: %s", raw)
		}
		return &parsedAddr{scheme: "unix", path: cleanUnixPath(u.Path)}, nil

	case "npipe":
		pipe, err := parseNpipePath(u)
		if err != nil {
			return nil, fmt.Errorf("invalid npipe address %q: %w", raw, err)
		}
		return &parsedAddr{scheme: "npipe", pipe: pipe}, nil

	default:
		return nil, fmt.Errorf("unsupported scheme %q in address %s", u.Scheme, raw)
	}
}

// parseNpipePath converts a parsed npipe:// URL to a Windows pipe path.
//
// Supported forms:
//
//	npipe:////./pipe/name  → \\.\pipe\name  (Docker standard, 4 slashes)
//	npipe://./pipe/name    → \\.\pipe\name  (Docker.DotNet style, host=".")
func parseNpipePath(u *url.URL) (string, error) {
	switch {
	// npipe:////./pipe/name → host="", path="//./pipe/name"
	case u.Host == "" && len(u.Path) > 1 && u.Path[:2] == "//":
		return toBackslash(u.Path), nil

	// npipe://./pipe/name → host=".", path="/pipe/name"
	case u.Host == ".":
		return `\\.` + toBackslash(u.Path), nil

	default:
		return "", fmt.Errorf("expected npipe:////./pipe/<name> or npipe://./pipe/<name>")
	}
}

func toBackslash(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == '/' {
			b[i] = '\\'
		}
	}
	return string(b)
}

// cleanUnixPath strips the leading slash from Windows drive-letter paths.
// url.Parse("unix:///C:\tmp\a.sock") yields path="/C:\tmp\a.sock";
// net.Listen("unix", ...) needs "C:\tmp\a.sock" on Windows.
func cleanUnixPath(p string) string {
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		return p[1:]
	}
	return p
}
