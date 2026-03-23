# ktha — Agent Instructions

Serverless Node.js app host. Kineto (JetBrains) take-home assignment.

---

## Project Overview

A Go service that acts as a reverse proxy + process manager for isolated Node.js guest apps.

**Core flow:** HTTP request → path-based routing → cold-start if needed → reverse proxy to app via unix socket → idle timeout → shutdown.

**Key components:**
- Reverse proxy with path-based routing (`/app-id/...`)
- Process manager: spawn Node.js in Linux namespaces (mount + PID), communicate via unix domain sockets
- Resource limits via cgroups v2
- Serverless lifecycle: start-on-request, stop-on-idle

## Tech Stack

- **Language:** Go
- **Isolation:** Linux namespaces + cgroups v2 (not Docker)
- **IPC:** Unix domain sockets
- **Guest runtime:** Node.js

## Build & Run

```bash
go build -o ktha ./cmd/ktha
sudo ./ktha  # needs root for namespace/cgroup operations
```

## Code Conventions

- Standard Go project layout: `cmd/`, `internal/`
- Use `slog` for structured logging
- Error wrapping with `fmt.Errorf("context: %w", err)`
- No unnecessary abstractions — this is a prototype, keep it direct
- Tests where they add confidence, not for coverage metrics

## What Gets Committed

This is a deliverable artifact. Everything in the repo should be something we're comfortable showing to the Kineto review panel. That includes this file — it demonstrates agentic coding workflow.

**Commit:** code, design docs, agent instructions, useful scripts
**Don't commit:** local env config, scratch files, vault-specific references

## App Artifact Format

Guest apps are pre-bundled archives (tar.gz) containing:
- Application source files
- `node_modules/` (pre-installed, no npm on host)
- Common entrypoint convention (e.g., `index.js` or defined in manifest)

## Design Decisions Log

Key decisions and their rationale — useful context for the agent and for the review panel.

| Decision | Choice | Why |
|----------|--------|-----|
| Language | Go | Systems-level work, good concurrency, low per-process overhead |
| Routing | Path-based | Simpler for demo (no DNS/wildcard certs) |
| Proxy | Built-in (`httputil.ReverseProxy`) | Cold-start hold is natural (check → start → wait → forward) |
| Isolation | Linux namespaces | Real OS-level isolation without Docker overhead |
| IPC | Unix domain sockets | Avoids port exhaustion, better perf than TCP loopback |
| Resource limits | cgroups v2 | Standard Linux mechanism, fine-grained control |
| Idle timeout | Global config param | Per-app timeout discussed verbally, not implemented |
