// Package tools implements all LeapDoctor MCP tool handlers.
package tools

import (
	"fmt"

	"github.com/olindenern/leapdoctor/internal/guard"
	"github.com/olindenern/leapdoctor/internal/mcp"
	"github.com/olindenern/leapdoctor/internal/ratelimit"
)

// Toolbox holds shared dependencies for all tool implementations.
type Toolbox struct {
	guard     *guard.SnapperGuard
	rateLimit *ratelimit.Limiter
	dryRun    bool // global dry-run mode from --dry-run flag
}

func New(g *guard.SnapperGuard, rl *ratelimit.Limiter, dryRun bool) *Toolbox {
	return &Toolbox{guard: g, rateLimit: rl, dryRun: dryRun}
}

// Dispatch routes a tool call to the appropriate handler.
func (t *Toolbox) Dispatch(name string, args map[string]interface{}) mcp.ToolResult {
	arg := func(key string) string {
		if v, ok := args[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	argBool := func(key string) bool {
		return arg(key) == "true"
	}

	// In global dry-run mode, force dry_run=true for destructive ops
	forceDryRun := t.dryRun

	switch name {
	case "suse_zypper_install":
		return t.ZypperInstall(arg("package"), argBool("dry_run") || forceDryRun)
	case "suse_zypper_remove":
		return t.ZypperRemove(arg("package"), argBool("dry_run") || forceDryRun)
	case "suse_snapshot_create":
		return t.SnapshotCreate(arg("description"))
	case "suse_snapshot_list":
		return t.SnapshotList()
	case "suse_snapshot_rollback":
		if forceDryRun {
			return mcp.ToolErr("DRY-RUN MODE: Rollback simulated, no changes made.")
		}
		return t.SnapshotRollback(arg("snapshot_number"))
	case "suse_system_health":
		return t.SystemHealth()
	case "suse_journalctl":
		return t.Journalctl(arg("service"), arg("priority"), arg("lines"), arg("since"))
	case "suse_gnome_config_get":
		return t.GnomeConfigGet(arg("schema"), arg("key"))
	case "suse_gnome_config_set":
		if forceDryRun {
			return mcp.ToolOK(fmt.Sprintf("DRY-RUN MODE: Would set %s %s = %s", arg("schema"), arg("key"), arg("value")))
		}
		return t.GnomeConfigSet(arg("schema"), arg("key"), arg("value"))
	case "suse_flatpak_install":
		return t.FlatpakInstall(arg("app_id"), arg("remote"), argBool("dry_run") || forceDryRun)
	case "suse_selinux_status":
		return t.SelinuxStatus(argBool("show_denials"), arg("lines"))
	case "suse_systemctl":
		return t.Systemctl(arg("action"), arg("service"))
	case "suse_system_info":
		return t.SystemInfo()
	default:
		return mcp.ToolErr(fmt.Sprintf("Unknown tool: %s", name))
	}
}
