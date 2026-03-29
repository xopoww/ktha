# ktha — Agent Instructions

Serverless Node.js app host. Kineto (JetBrains) take-home assignment.

---

## Project Overview

A Go service that acts as a reverse proxy + process manager for isolated Node.js guest apps.

**Core flow:** HTTP request → path-based routing → cold-start if needed → reverse proxy to app via unix socket → idle timeout → shutdown.

**Key components:**
- Reverse proxy with path-based routing (`/app-id/...`)
- Process manager: spawn Node.js in Linux namespaces (mount, PID, network, UTS), communicate via unix domain sockets
- Resource limits via cgroups v2
- Serverless lifecycle: start-on-request, stop-on-idle
- Admin API for app management (add, delete, upgrade)
- Prometheus metrics + Grafana dashboard

## Tech Stack

- **Language:** Go
- **Isolation:** Linux namespaces + cgroups v2 (not Docker)
- **IPC:** Unix domain sockets
- **Guest runtime:** Node.js
- **Observability:** Prometheus + Grafana
- **Deployment:** Ansible + Terraform (GCP)

## Repo Structure

```
ktha/
├── node/       # Go module: ktha-node (host) + ktha-runner (container runtime)
├── apps/       # Node.js guest apps (demo + load test)
├── tools/      # Load test tooling (TypeScript)
├── grafana/    # Grafana dashboard JSON
├── deploy/     # Ansible playbook + Terraform (GCP)
└── docs/       # Design doc (Typst) + architecture diagrams
```

## Build & Run

```bash
cd node && make build     # produces build/ktha-node + build/ktha-runner
sudo ./build/ktha-node -config /path/to/config.yml
```

## Code Conventions

- Standard Go project layout under `node/`: `cmd/`, `internal/`
- `zap` (sugared) for structured logging
- Error wrapping with `fmt.Errorf("context: %w", err)`
- No unnecessary abstractions — this is a prototype, keep it direct
- Tests where they add confidence, not for coverage metrics

## AI-Assisted Development

The Go code (proxy, manager, controller, container, runner) was written by hand — the systems work was the interesting part and I wanted full accountability for every design choice. The agent was used for review, discussion, and as a sounding board for architecture decisions.

Infrastructure-as-config (Ansible, Terraform, Grafana dashboards), demo apps, and load test tooling were AI-assisted — work where the value is in the specification, not the keystrokes.

## What Gets Committed

This is a deliverable artifact. Everything in the repo should be something we're comfortable showing to the Kineto review panel. That includes this file — it demonstrates agentic coding workflow.

**Commit:** code, design docs, agent instructions, deployment config, useful scripts
**Don't commit:** local env config, secrets, terraform state, scratch files

## App Image Format

An app image is a directory containing:
- `index.js` — single esbuild bundle (all dependencies included, no `node_modules/`)
- `public/` (optional) — static assets

The guest app listens on a Unix socket (path provided via `KTHA_SOCK` env var) and must respond to `GET /healthcheck` with `{"ok": true}` when ready.

## Design Decisions Log

Key decisions and their rationale — useful context for the agent and for the review panel.

| Decision | Choice | Why |
|----------|--------|-----|
| Language | Go | Systems-level work, good concurrency, low per-process overhead |
| Routing | Path-based | Simpler for demo (no DNS/wildcard certs) |
| Proxy | Built-in (`httputil.ReverseProxy`) | Cold-start hold is natural (check → start → wait → forward) |
| Isolation | Linux namespaces (mount, PID, net, UTS) | Real OS-level isolation without Docker overhead |
| IPC | Unix domain sockets | Avoids port exhaustion, better perf than TCP loopback |
| Resource limits | cgroups v2 | Standard Linux mechanism, fine-grained control |
| Idle timeout | Global config param | Per-app timeout is a straightforward extension |
| Logging | zap (sugared) | Structured logging with good performance |
