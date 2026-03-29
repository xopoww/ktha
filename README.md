# ktha — Serverless Node.js App Host

A prototype backend for a lightweight Node.js hosting platform. Apps run in isolated Linux namespaces, start on first HTTP request (cold start), and stop after an idle timeout.

## Features

- **Path-based routing** — `host:port/app-id/...` routes traffic to the correct app
- **Serverless lifecycle** — apps start on demand, stop after configurable idle timeout
- **OS-level isolation** — Linux namespaces (mount, PID, network, UTS) + cgroups v2
- **Unix domain sockets** — IPC between proxy and guest apps, no port exhaustion
- **Observability** — Prometheus metrics + Grafana dashboard
- **Scalable** — load-tested with 1000 concurrent apps on GCP

## Requirements

- Linux (namespaces + cgroups v2)
- Go 1.24+
- Node.js 22+ (for guest apps)
- Root privileges (namespace/cgroup operations)

## Project Structure

```
ktha/
├── node/       # Go service: ktha-node (host) + ktha-runner (container runtime)
├── apps/       # Node.js guest apps (demo + load test)
├── tools/      # Load test tooling
├── grafana/    # Grafana dashboard
├── deploy/     # Ansible playbook + Terraform (GCP)
└── docs/       # Design document + architecture diagrams
```

## Building

```bash
cd node
make build    # produces build/ktha-node + build/ktha-runner
```

## Running

```bash
sudo ./node/build/ktha-node -config config.yml
```

The host process needs root for namespace creation, cgroup management, and bind-mounts. See the design document for details on the isolation model.

## Architecture

The system is split into two binaries:
- **ktha-node** — host process: reverse proxy, app lifecycle management, admin API, metrics
- **ktha-runner** — container runtime: namespace/cgroup setup, filesystem preparation, exec Node.js

See [docs/design-doc.pdf](docs/design-doc.pdf) for detailed design documentation with architecture diagrams.
