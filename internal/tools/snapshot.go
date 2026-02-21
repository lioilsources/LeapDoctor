package tools

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) SnapshotCreate(desc string) mcp.ToolResult {
	if desc == "" {
		desc = fmt.Sprintf("manual snapshot @ %s", time.Now().Format("2006-01-02 15:04"))
	}
	out, err := t.guard.Runner.Run("snapper", "create",
		"--cleanup-algorithm", "timeline",
		"--print-number",
		"--description", desc,
	)
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Snapshot failed: %s", out))
	}
	return mcp.ToolOK(fmt.Sprintf("Snapshot #%s created: %s", strings.TrimSpace(out), desc))
}

func (t *Toolbox) SnapshotList() mcp.ToolResult {
	out, err := t.guard.Runner.Run("snapper", "list", "--columns", "number,type,date,description")
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: snapper list failed: %s", out))
	}
	return mcp.ToolOK("Snapper snapshots:\n\n" + out)
}

func (t *Toolbox) SnapshotRollback(snapshotNum string) mcp.ToolResult {
	if snapshotNum == "" {
		return mcp.ToolErr("ERROR: 'snapshot_number' is required. Use suse_snapshot_list to see available snapshots.")
	}
	num, err := strconv.Atoi(strings.TrimSpace(snapshotNum))
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Invalid snapshot number: %q", snapshotNum))
	}

	if err := t.rateLimit.CheckDestructive(); err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Rate limit exceeded: %v", err))
	}

	out, err := t.guard.Runner.Run("snapper", "undochange", fmt.Sprintf("%d..0", num))
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Rollback to snapshot #%d failed: %s", num, out))
	}
	return mcp.ToolOK(fmt.Sprintf(
		"Rolled back to snapshot #%d.\n%s\n\n"+
			"IMPORTANT: A system restart is required to complete the rollback:\n   sudo systemctl reboot",
		num, out,
	))
}
