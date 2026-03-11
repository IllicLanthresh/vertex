
# Vertex Traffic Generator

**Linux-only** network traffic generator with multi-interface support and TUI control. Single binary with no dependencies.

> **Platform Support**: Linux only — requires MACVLAN kernel support for virtual network interfaces.

## Features

- Auto-discovers all network interfaces
- Full TUI control — start/stop/configure from your terminal
- Per-interface configuration — enable/disable interfaces individually
- Real-time monitoring — traffic stats and system status
- Single binary deployment — no dependencies or config files required
- Headless mode — run as a systemd service with `--headless`
- Creates virtual MACVLAN interfaces for realistic traffic simulation

## Quick Start

### One-liner install

```bash
curl -sSL https://raw.githubusercontent.com/IllicLanthresh/vertex/master/install.sh | sudo bash
```

### Manual install

```bash
# Linux AMD64
wget https://github.com/IllicLanthresh/vertex/releases/latest/download/vertex-linux-amd64
chmod +x vertex-linux-amd64
sudo mv vertex-linux-amd64 /usr/local/bin/vertex
```

### Run (requires root for network interface creation)

```bash
# Interactive TUI
sudo vertex

# Headless (for systemd / background use)
sudo vertex --headless
```

## Usage

```
vertex [options]

Options:
  -headless        Run without TUI, auto-starts traffic with embedded config
  -config string   External config file (optional - uses embedded defaults)
  -devices int     Virtual devices per interface (default: 3)
  -depth int       Max crawl depth (default: 25)
  -min-sleep int   Min seconds between fetches (default: 3)
  -max-sleep int   Max seconds between fetches (default: 6)
  -timeout int     HTTP request timeout in seconds (default: 30)
  -version         Show version information
```

### TUI Controls

| Key | Action |
|-----|--------|
| `s` | Start traffic generation |
| `x` | Stop traffic generation |
| `r` | Restart |
| `q` / `Ctrl+C` | Quit (graceful shutdown) |
| `↑↓` | Scroll logs |

## How It Works

1. **Interface Discovery** — automatically detects physical network interfaces
2. **Virtual Device Creation** — creates MACVLAN virtual devices on each enabled interface
3. **IP Assignment** — attempts DHCP on virtual devices for realistic network simulation
4. **Traffic Generation** — each virtual device runs independent HTTP crawlers with randomized user agents, sleep intervals, and link following
5. **Cleanup** — virtual interfaces are destroyed on stop/exit

## LXC / VM Deployment

Designed for isolated network environments:

```bash
# On your LXC container or VM:
curl -sSL https://raw.githubusercontent.com/IllicLanthresh/vertex/master/install.sh | sudo bash

# Interactive use
sudo vertex

# Or as a background service
sudo systemctl start vertex
sudo systemctl enable vertex  # start on boot
```

## Network Requirements

- **Root privileges** required for network interface manipulation
- **DHCP server** on network for automatic IP assignment to virtual devices
- **Internet access** for HTTP traffic generation
- **Network interfaces** must support MACVLAN mode

## Development

### Prerequisites
- Go 1.26+
- Root privileges for testing network features

### Build from Source
```bash
git clone https://github.com/IllicLanthresh/vertex.git
cd vertex
make build
```

### Cross-compile
```bash
make build-all  # builds linux-amd64 + linux-arm64
```

### Available make targets
```
make build       Build binary for current platform
make build-all   Build for all supported platforms (Linux amd64/arm64)
make run         Run in development mode
make test        Run tests
make check       Run format, vet, and test
make clean       Clean build artifacts
```

## Platform Support

**Supported:** Linux (AMD64/ARM64) — full functionality including MACVLAN virtual interfaces

**Not supported:** Windows, macOS — MACVLAN interfaces require Linux kernel support via netlink

## Troubleshooting

**"Permission denied" errors**
- Run with root: `sudo vertex`

**"No interfaces found"**
- Check interfaces exist: `ip link show`
- Verify interfaces have MAC addresses (physical interfaces)

**Virtual devices not getting IP addresses**
- Ensure DHCP server is available on the network
- Check network supports MACVLAN mode

## Acknowledgments

Vertex was originally inspired by [Noisy](https://github.com/1tayH/noisy) by Itay Hury — a Python-based random HTTP traffic generator. Vertex is a complete rewrite in Go with a different architecture, TUI interface, and multi-interface MACVLAN support.

## License

MIT License — see [LICENSE](LICENSE) file.
