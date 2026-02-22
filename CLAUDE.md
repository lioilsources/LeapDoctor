# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

LeapDoctor is an MCP (Model Context Protocol) server written in Go for safe system administration on openSUSE Leap 16. It provides Claude with safety-wrapped tools for system operations — all destructive actions are wrapped in Btrfs/Snapper snapshots with automatic rollback on health check failures.

## Build & Test Commands

```bash
make build          # Build binary (requires cmd/leapdoctor/ entry point)
make test           # Run all unit tests: go test ./...
make test-mcp       # Integration test: pipes JSON-RPC requests to binary, validates output
make check          # Build + run binary with --check flag
make install        # Build + install to /usr/local/bin with config dirs
make service-install # Install + register systemd recovery service
make clean          # Remove built binary
```

Run a single package's tests:
```bash
go test ./internal/mcp/
go test ./internal/ratelimit/
```

Zero external dependencies — only Go stdlib (Go 1.24.5).

## Architecture

### MCP Protocol Flow
Stdin → JSON-RPC 2.0 request → `Server.Run()` dispatches to handler → tool method executes → JSON-RPC response → stdout

### Destructive Operation Flow
Rate limit check → pre-snapshot (Snapper) → execute command → post-snapshot → health check (GDM, systemd, SELinux, journal) → rollback if unhealthy

### Key Packages (`internal/`)

- **mcp/** — Hand-rolled JSON-RPC 2.0 transport (types + server). Not using an MCP SDK.
- **guard/** — SnapperGuard: snapshot creation, health checks, rollback, atomic state file (`/var/lib/leapdoctor/last-snapshot`)
- **tools/** — 13 tool implementations, all prefixed `suse_`. Toolbox dispatches calls and holds shared Guard/RateLimit/config.
- **exec/** — Runner interface (`SystemRunner`, `MockRunner`) for dependency injection in tests
- **ratelimit/** — Sliding window rate limiting (10 destructive ops/30min) + loop detection (same tool+args rolled back within 10min) + rollback lockout (3 rollbacks → read-only)
- **config/** — JSON config loading from `/etc/leapdoctor/config.json` or `~/.config/leapdoctor/config.json`, plus Claude Code / mcphost integration helpers
- **recovery/** — Post-boot health check and auto-rollback logic (systemd service)

### Prototype vs Refactored Code
`prototype/main.go` is the original single-file implementation (~530 lines, Czech comments). The refactored code lives in `internal/` with English tool descriptions. The entry point is `cmd/leapdoctor/main.go`.

## Key Design Patterns

- **Atomic state files**: Write to `.tmp` then `os.Rename()` to prevent partial writes
- **Dependency injection**: Tools receive a Toolbox with shared Guard, RateLimit, and config; exec.Runner interface enables MockRunner for testing
- **Tool results**: Use `mcp.ToolOK(text)` / `mcp.ToolErr(text)` helpers for structured MCP responses
- **Critical package blocking**: GNOME-critical packages (gnome-shell, gdm, mutter, glib2, gtk3, gtk4, wayland, pipewire) are blocked from removal
- **Scanner buffer**: 1MB (increased from 64KB default) for large tool outputs
- **Tool naming**: All tools prefixed `suse_`, descriptions in English for LLM compatibility

## MCP Integration

```bash
# Method 1: Automatic (recommended)
make install
leapdoctor --setup-claude

# Method 2: Claude Code CLI
claude mcp add leapdoctor -- /usr/local/bin/leapdoctor

# Method 3: Manual — add to ~/.claude.json under mcpServers:
#   "leapdoctor": { "command": "/usr/local/bin/leapdoctor" }
```

For mcphost, use `leapdoctor --setup-mcphost` or add to `~/.mcphost.yml` manually.

Tip: use `leapdoctor --dry-run` as the command for first-time testing — all destructive operations will simulate instead of executing.
