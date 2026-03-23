# ktha — Serverless Node.js App Host

A prototype backend for a lightweight Node.js hosting platform. Apps run in isolated Linux namespaces, start on first HTTP request (cold start), and stop after an idle timeout.

## Features

- **Path-based routing** — `host:port/app-id/...` routes traffic to the correct app
- **Serverless lifecycle** — apps start on demand, stop after configurable idle timeout
- **OS-level isolation** — Linux namespaces (mount, PID) + cgroups v2 for resource limits
- **Unix domain sockets** — IPC between proxy and app processes, no port exhaustion
- **Scalable** — designed to support up to 1000 concurrent apps

## Requirements

- Linux (namespaces + cgroups v2)
- Go 1.22+
- Node.js (for guest apps)
- Root or appropriate capabilities (`CAP_SYS_ADMIN`, `CAP_SYS_CHROOT`)

## Project Structure

```
ktha/
├── node/       # Go service: reverse proxy + process manager
├── apps/       # Sample Node.js guest apps for testing and demo
└── docs/       # Design documentation and diagrams
```

## Building

```bash
cd node
go build -o ktha ./cmd/ktha
```

## Architecture

See [docs/DESIGN.md](docs/DESIGN.md) for detailed design documentation with diagrams.
