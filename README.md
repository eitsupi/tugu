# tugu

A minimal, zero-dependency proxy that bridges Windows named pipes and Unix domain sockets. Single binary, small footprint, does one thing.

The name comes from the Japanese word "継ぐ" (tsugu), meaning "to join" or "to inherit."

## Why

Windows IPC is fragmented. Many Windows services expose named pipes (`\\.\pipe\...`), but Unix-oriented tools only speak Unix domain sockets (AF_UNIX). The reverse is also true: tools like Win32-OpenSSH's ssh-agent communicate exclusively via named pipes, leaving Unix socket clients unable to connect.

| Tool / Runtime | AF_UNIX (UDS) | Named Pipe (npipe) |
|----------------|:-:|:-:|
| curl (Windows, 8.13.0+) | ✅ | ❌ |
| Win32-OpenSSH ssh-agent | ❌ | ✅ |
| socat | ✅ | ❌ (no Windows support) |
| Docker CLI | ✅ | ✅ |

Despite this gap, no general-purpose proxy bridges named pipes and Unix domain sockets on native Windows without runtime dependencies. Existing alternatives each have limitations:

- **[npiperelay](https://github.com/jstarks/npiperelay)** ([actively maintained fork](https://github.com/albertony/npiperelay)) relays a single named pipe connection to stdio. It cannot listen on a socket by itself and requires socat on the WSL side, making it unsuitable for native Windows use.
- **[WinSocat](https://github.com/firejox/WinSocat)** is more feature-rich, supporting npipe, UDS, TCP, Hyper-V sockets, serial ports, and WSL2 interop. Its self-contained release is a single .NET binary.

tugu takes a different approach: do one thing in as little code as possible. It is a small static Go binary (~3 MB) with no runtime dependencies, easy to cross-compile, and simple to audit. If all you need is an npipe/UDS bridge on native Windows, tugu is the smallest tool for the job.

## Use cases

**Testing npipe-only services with curl.**
Some Windows runtimes (e.g., Rust's tokio) do not yet support AF_UNIX listeners and only expose named pipes. curl on Windows supports Unix sockets but not named pipes, so you cannot reach these services directly. tugu bridges the gap:

```bash
tugu npipe:////./pipe/myapp unix:///tmp/myapp.sock
curl --unix-socket /tmp/myapp.sock http://localhost/health
```

**ssh-agent forwarding.**
Win32-OpenSSH's ssh-agent listens on a named pipe. SSH clients or tools that expect `SSH_AUTH_SOCK` (a Unix socket path) can reach it through tugu:

```bash
tugu npipe:////./pipe/openssh-ssh-agent unix:///tmp/ssh-agent.sock
export SSH_AUTH_SOCK=/tmp/ssh-agent.sock
```

**Connecting Unix-oriented tools to Windows services.**
Any Windows service that communicates over named pipes can be made accessible to tools that only support AF_UNIX sockets, without modifying either side.

## Alternatives

tugu bridges transports **within the same OS** (primarily native Windows). If you need to cross the WSL2/Windows boundary (e.g., connecting from a WSL2 Linux process to a Windows named pipe), use [npiperelay](https://github.com/jstarks/npiperelay) with socat instead.

| Tool | npipe | UDS | TCP | Runtime deps | Platform |
|------|:-----:|:---:|:---:|:-------------|:---------|
| **tugu** | ✅ listen+connect | ✅ listen+connect | ✅ listen+connect | None (static binary) | Windows, Linux, macOS |
| [npiperelay](https://github.com/jstarks/npiperelay) ([fork](https://github.com/albertony/npiperelay)) | connect only | ❌ (via socat) | ❌ | socat (WSL side) | WSL2 → Windows |
| [WinSocat](https://github.com/firejox/WinSocat) | ✅ listen+connect | ✅ listen+connect | ✅ listen+connect | .NET 6+ or self-contained | Windows (+ WSL2 via HVSOCK) |
| socat | ❌ | ✅ listen+connect | ✅ listen+connect | libc | Linux, macOS |

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
| Named pipe (Windows) | `npipe:////./pipe/name` | `npipe:////./pipe/myapp` |

`unix://localhost/path` is also accepted. Named pipes use the [Docker convention](https://docs.docker.com/reference/cli/dockerd/) (`npipe:////./pipe/name`). TCP support is included as a fallback for environments where neither named pipes nor Unix sockets are available.

### Examples

```bash
# Named pipe → Unix domain socket (the primary use case)
tugu npipe:////./pipe/myapp unix:///tmp/myapp.sock

# Named pipe → TCP (expose a Windows pipe as a TCP port)
tugu npipe:////./pipe/myapp tcp://127.0.0.1:8080

# TCP → Unix domain socket
tugu tcp://127.0.0.1:3000 unix:///var/run/app.sock

# Unix domain socket → TCP
tugu unix:///tmp/proxy.sock tcp://127.0.0.1:9090
```

### Flags

```
--verbose       Enable verbose logging
--version       Print version and exit
--help          Print help and exit
```

## Build

Requires Go 1.25 or later.

```bash
# Native
go build -o tugu .

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o tugu.exe .
```

## License

MIT
