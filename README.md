# tugu

A lightweight, cross-platform IPC socket bridge that proxies bidirectional traffic between Windows named pipes, Unix domain sockets, and TCP.

The name comes from the Japanese word "継ぐ" (tsugu), meaning "to join" or "to inherit."

## Why

Many tools on Windows use named pipes for IPC, but Unix-oriented tools (curl, socat, SSH forwarding, etc.) don't support them. Windows' built-in OpenSSH still cannot forward Unix domain sockets ([Win32-OpenSSH#1564](https://github.com/PowerShell/Win32-OpenSSH/issues/1564)), and AF_UNIX support on Windows remains limited in many runtimes.

Despite this, no general-purpose proxy exists to bridge named pipes and Unix domain sockets. tugu fills that gap as a single binary with no runtime dependencies.

## Install

```bash
go install github.com/eitsupi/tugu@latest
```

## Usage

```
tugu <listen-address> <connect-address>
```

tugu listens on the first address and connects to the second, then copies data bidirectionally for each incoming connection.

### Address formats

| Transport | Format | Example |
|-----------|--------|---------|
| TCP | `tcp://host:port` | `tcp://127.0.0.1:8080` |
| Unix domain socket | `unix:///path` | `unix:///tmp/app.sock` |
| Named pipe (Windows only) | `npipe:////./pipe/name` | `npipe:////./pipe/myapp` |

`unix://localhost/path` is also accepted. Named pipes use the [Docker convention](https://docs.docker.com/reference/cli/dockerd/) (`npipe:////./pipe/name`).

### Examples

```bash
# Named pipe → TCP (expose a Windows pipe as a TCP port)
tugu npipe:////./pipe/myapp tcp://127.0.0.1:8080

# TCP → Unix domain socket
tugu tcp://127.0.0.1:3000 unix:///var/run/app.sock

# Unix domain socket → TCP
tugu unix:///tmp/proxy.sock tcp://127.0.0.1:9090
```

### Flags

```
--keep-alive    Reconnect to the connect-side if it becomes unavailable
--verbose       Enable verbose logging
--version       Print version and exit
--help          Print help and exit
```

## Build

Requires Go 1.25 or later.

```bash
# Native
go build -o tugu .

# Cross-compile
GOOS=windows GOARCH=amd64 go build -o tugu.exe .
GOOS=linux   GOARCH=amd64 go build -o tugu .
GOOS=darwin  GOARCH=arm64 go build -o tugu .
```

## License

MIT
