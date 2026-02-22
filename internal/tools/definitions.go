package tools

import "github.com/olindenern/leapdoctor/internal/mcp"

// Tools returns the MCP tool definitions for all LeapDoctor tools.
func Tools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name: "suse_zypper_install",
			Description: `Safely installs a package via zypper with automatic Snapper pre/post snapshots.
Creates a pre-snapshot before installation, verifies system health (GNOME/systemd) afterwards.
Automatically rolls back on failure. Only for openSUSE Leap 16 with Btrfs+Snapper.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"package": {
						Type:        "string",
						Description: "Package name to install (e.g. 'git', 'vim', 'htop')",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "If true, only simulates — no changes are made",
					},
				},
				Required: []string{"package"},
			},
			Annotations: mcp.Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_zypper_remove",
			Description: `Safely removes a package via zypper with Snapper snapshots and rollback capability.
Warns if the package being removed may affect GNOME or critical system components.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"package": {
						Type:        "string",
						Description: "Package name to remove",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "If true, only simulates",
					},
				},
				Required: []string{"package"},
			},
			Annotations: mcp.Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_snapshot_create",
			Description: `Creates a Snapper snapshot of the current system state.
Useful before manual changes or as a safety restore point.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"description": {
						Type:        "string",
						Description: "Snapshot description (what you are about to change)",
					},
				},
				Required: []string{"description"},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: false, DestructiveHint: false},
		},
		{
			Name: "suse_snapshot_list",
			Description: `Lists existing Snapper snapshots with their descriptions and timestamps.
Read-only operation.`,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]mcp.Property{},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true},
		},
		{
			Name: "suse_snapshot_rollback",
			Description: `Rolls back to a specific snapshot number.
WARNING: This is a destructive operation — it reverts system files to a previous state.
A system restart is required after rollback. Show the user the snapshot list (suse_snapshot_list) before calling this.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"snapshot_number": {
						Type:        "string",
						Description: "Snapshot number (use suse_snapshot_list to find it)",
					},
				},
				Required: []string{"snapshot_number"},
			},
			Annotations: mcp.Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_system_health",
			Description: `Checks system health: failed systemd services, GNOME shell status,
SELinux denial logs, Flatpak status, and the last 20 system errors from journalctl.
Read-only operation, safe to call repeatedly.`,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]mcp.Property{},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_journalctl",
			Description: `Reads systemd journal logs. Can filter by priority, service, or time range.
Read-only operation.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"service": {
						Type:        "string",
						Description: "Systemd service name (e.g. 'gdm', 'NetworkManager', 'flatpak'). Empty = all.",
					},
					"priority": {
						Type:        "string",
						Description: "Minimum log priority",
						Enum:        []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"},
					},
					"lines": {
						Type:        "string",
						Description: "Number of last lines (default: 50)",
					},
					"since": {
						Type:        "string",
						Description: "Since when (e.g. '1 hour ago', '2025-01-01', 'today')",
					},
				},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_gnome_config_get",
			Description: `Reads a GNOME/dconf setting via gsettings. Read-only.
Example schemas: org.gnome.desktop.interface, org.gnome.shell`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"schema": {
						Type:        "string",
						Description: "dconf schema (e.g. 'org.gnome.desktop.interface')",
					},
					"key": {
						Type:        "string",
						Description: "Key in schema (empty = list all keys in schema)",
					},
				},
				Required: []string{"schema"},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_gnome_config_set",
			Description: `Sets a GNOME/dconf value via gsettings with Snapper snapshots.
Creates a snapshot before the change for rollback capability.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"schema": {
						Type:        "string",
						Description: "dconf schema (e.g. 'org.gnome.desktop.interface')",
					},
					"key": {
						Type:        "string",
						Description: "Key in schema",
					},
					"value": {
						Type:        "string",
						Description: "New value (in gsettings format, e.g. 'true', \"'dark'\", \"['ext1']\")",
					},
				},
				Required: []string{"schema", "key", "value"},
			},
			Annotations: mcp.Annotations{DestructiveHint: false},
		},
		{
			Name: "suse_flatpak_install",
			Description: `Installs a Flatpak application from Flathub or another configured remote.
This is the preferred method for installing desktop applications on GNOME/Wayland.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"app_id": {
						Type:        "string",
						Description: "Flatpak Application ID (e.g. 'org.gimp.GIMP', 'com.spotify.Client')",
					},
					"remote": {
						Type:        "string",
						Description: "Remote name (default: 'flathub')",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "Simulate only, do not install",
					},
				},
				Required: []string{"app_id"},
			},
			Annotations: mcp.Annotations{DestructiveHint: false},
		},
		{
			Name: "suse_selinux_status",
			Description: `Shows SELinux status, recent denial records, and resolution recommendations.
openSUSE Leap 16 uses SELinux as the default LSM (not AppArmor).`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"show_denials": {
						Type:        "boolean",
						Description: "Show SELinux denial records from audit (default: true)",
					},
					"lines": {
						Type:        "string",
						Description: "Number of denial records (default: 20)",
					},
				},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_systemctl",
			Description: `Manages systemd services: start, stop, restart, enable, disable, status.
Destructive actions (start/stop/restart) are logged before execution.`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]mcp.Property{
					"action": {
						Type:        "string",
						Description: "Action for systemctl",
						Enum:        []string{"status", "start", "stop", "restart", "enable", "disable", "is-active", "is-enabled"},
					},
					"service": {
						Type:        "string",
						Description: "Systemd unit name (e.g. 'gdm.service', 'NetworkManager')",
					},
				},
				Required: []string{"action", "service"},
			},
			Annotations: mcp.Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_system_info",
			Description: `Shows system information: OS version, kernel, desktop environment, Btrfs/Snapper status,
Flatpak availability, and SELinux mode. Read-only operation.`,
			InputSchema: mcp.InputSchema{
				Type:       "object",
				Properties: map[string]mcp.Property{},
			},
			Annotations: mcp.Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
	}
}
