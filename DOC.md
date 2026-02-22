# LeapDoctor

MCP server for safe system administration on openSUSE Leap 16 with GNOME/Wayland. Gives Claude (or any MCP client) tools for package management, GNOME configuration, service control, and system diagnostics — all destructive operations are wrapped in Btrfs/Snapper snapshots with automatic rollback on health check failure.

## Quick Start

```bash
make build
make install            # installs to /usr/local/bin
leapdoctor --setup-claude   # registers with Claude Code (~/.claude.json)
leapdoctor --check          # verify system capabilities
```

Then start a Claude Code session — the `suse_*` tools will be available.

## Available Tools

**Package Management**
- `suse_zypper_install` — install a package with automatic snapshots and rollback
- `suse_zypper_remove` — remove a package (blocks GNOME-critical packages)
- `suse_flatpak_install` — install a Flatpak app from Flathub

**Snapshots**
- `suse_snapshot_create` — create a manual Snapper snapshot
- `suse_snapshot_list` — list existing snapshots
- `suse_snapshot_rollback` — rollback to a specific snapshot

**System Monitoring**
- `suse_system_health` — check failed services, GNOME status, SELinux denials, journal errors
- `suse_system_info` — OS version, kernel, desktop environment, Btrfs/Snapper/Flatpak status
- `suse_journalctl` — read systemd journal logs with filters
- `suse_selinux_status` — SELinux mode and recent denials

**GNOME Configuration**
- `suse_gnome_config_get` — read a dconf/gsettings value
- `suse_gnome_config_set` — set a dconf/gsettings value with snapshot

**Service Management**
- `suse_systemctl` — start, stop, restart, enable, disable, or query systemd services

## Safety Features

- **Automatic snapshots**: every destructive operation creates a Snapper pre/post snapshot pair
- **Health checks**: after each destructive operation, GDM, systemd, SELinux, and journalctl are verified
- **Automatic rollback**: if health checks fail after an operation, the snapshot is rolled back
- **Rate limiting**: max 10 destructive operations per 30 minutes (configurable)
- **Loop detection**: blocks repeated calls to the same tool+args that were rolled back within 10 minutes
- **Rollback lockout**: 3 rollbacks in a window locks the server to read-only mode
- **Critical package blocking**: refuses to remove gnome-shell, gdm, mutter, glib2, gtk3, gtk4, wayland, pipewire

## Configuration

LeapDoctor loads config from `/etc/leapdoctor/config.json` or `~/.config/leapdoctor/config.json`:

```json
{
  "dry_run": false,
  "rate_limit": {
    "max_destructive_per_30min": 10,
    "max_rollbacks_before_lock": 3
  }
}
```

## Recovery Service

```bash
make service-install    # installs + registers systemd service
```

This installs a systemd service that runs `leapdoctor --post-boot-check` on boot. If the last operation before shutdown/crash was a destructive action that left the system unhealthy, it automatically rolls back.

## Flags

| Flag | Description |
|---|---|
| `--check` | Print system capabilities and exit |
| `--dry-run` | Force all destructive operations to simulate |
| `--post-boot-check` | Run post-boot health check and exit |
| `--setup-claude` | Add LeapDoctor to `~/.claude.json` and exit |
| `--setup-mcphost` | Add LeapDoctor to `~/.mcphost.yml` and exit |

## MCP Integration

```bash
# Automatic (recommended)
make install
leapdoctor --setup-claude

# Claude Code CLI
claude mcp add leapdoctor -- /usr/local/bin/leapdoctor

# mcphost
leapdoctor --setup-mcphost
```

Use `--dry-run` as the command for first-time testing:
```bash
claude mcp add leapdoctor -- /usr/local/bin/leapdoctor --dry-run
```

## Requirements

- openSUSE Leap 16 with Btrfs root filesystem
- Snapper installed and configured
- Go 1.24.5+ (build only — zero external dependencies)
