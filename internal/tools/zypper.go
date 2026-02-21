package tools

import (
	"fmt"
	"strings"
	"time"

	"github.com/olindenern/leapdoctor/internal/guard"
	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) ZypperInstall(pkg string, dryRun bool) mcp.ToolResult {
	if pkg == "" {
		return mcp.ToolErr("ERROR: 'package' parameter is required")
	}

	if dryRun {
		out, _ := t.guard.Runner.Run("zypper", "--non-interactive", "--dry-run", "install", pkg)
		return mcp.ToolOK(fmt.Sprintf("DRY RUN - zypper install %s:\n%s", pkg, out))
	}

	if !t.guard.BtrfsAvailable() {
		return mcp.ToolErr("ERROR: Btrfs not available - Snapper snapshots not possible. Aborting.")
	}

	if err := t.rateLimit.CheckDestructive(); err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Rate limit exceeded: %v", err))
	}

	if t.rateLimit.WasRolledBack("suse_zypper_install", pkg) {
		return mcp.ToolErr(fmt.Sprintf(
			"ERROR: zypper install %s was already attempted and rolled back recently. Manual intervention required.", pkg))
	}

	desc := fmt.Sprintf("zypper install %s @ %s", pkg, time.Now().Format("2006-01-02 15:04"))

	preNum, err := t.guard.CreatePreSnapshot(desc)
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Cannot create pre-snapshot: %v\nInstallation aborted.", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pre-snapshot #%d created\nInstalling: %s\n\n", preNum, pkg))

	out, installErr := t.guard.Runner.Run("zypper", "--non-interactive", "install", pkg)
	sb.WriteString(out + "\n\n")

	postNum, _ := t.guard.CreatePostSnapshot(preNum, desc)
	if postNum > 0 {
		sb.WriteString(fmt.Sprintf("Post-snapshot #%d created\n", postNum))
	}

	if installErr != nil {
		sb.WriteString("\nERROR: Installation failed!\n")
		if rbErr := t.guard.RollbackToSnapshot(preNum); rbErr != nil {
			sb.WriteString(fmt.Sprintf("ERROR: Rollback ALSO failed: %v\nManual rollback: snapper undochange %d..0\n", rbErr, preNum))
		} else {
			sb.WriteString(fmt.Sprintf("Rolled back to snapshot #%d\n", preNum))
			t.guard.ClearStateFile()
			t.rateLimit.RecordRollback("suse_zypper_install", pkg)
		}
		return mcp.ToolErr(sb.String())
	}

	health := t.guard.CheckSystemHealth()
	sb.WriteString("\nHealth check:\n" + health.Details)

	if !health.OK {
		sb.WriteString(fmt.Sprintf("\nWARN: System unhealthy after install! Rolling back to snapshot #%d...\n", preNum))
		if rbErr := t.guard.RollbackToSnapshot(preNum); rbErr != nil {
			sb.WriteString(fmt.Sprintf("ERROR: Rollback failed: %v\n", rbErr))
		} else {
			sb.WriteString("Rollback complete. Recommend restart: sudo systemctl reboot\n")
			t.guard.ClearStateFile()
			t.rateLimit.RecordRollback("suse_zypper_install", pkg)
		}
		return mcp.ToolErr(sb.String())
	}

	t.guard.ClearStateFile()
	sb.WriteString(fmt.Sprintf("\n%s successfully installed (snapshots #%d -> #%d)\n", pkg, preNum, postNum))
	return mcp.ToolOK(sb.String())
}

func (t *Toolbox) ZypperRemove(pkg string, dryRun bool) mcp.ToolResult {
	if pkg == "" {
		return mcp.ToolErr("ERROR: 'package' parameter is required")
	}

	if guard.IsCriticalPackage(pkg) {
		return mcp.ToolErr(fmt.Sprintf(
			"BLOCKED: '%s' is a critical GNOME package!\n"+
				"Removing it could break the desktop environment.\n"+
				"If you are sure, do it manually.", pkg))
	}

	if dryRun {
		out, _ := t.guard.Runner.Run("zypper", "--non-interactive", "--dry-run", "remove", pkg)
		return mcp.ToolOK(fmt.Sprintf("DRY RUN - zypper remove %s:\n%s", pkg, out))
	}

	if err := t.rateLimit.CheckDestructive(); err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Rate limit exceeded: %v", err))
	}

	desc := fmt.Sprintf("zypper remove %s @ %s", pkg, time.Now().Format("2006-01-02 15:04"))

	preNum, err := t.guard.CreatePreSnapshot(desc)
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Cannot create pre-snapshot: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Pre-snapshot #%d created\nRemoving: %s\n\n", preNum, pkg))

	out, removeErr := t.guard.Runner.Run("zypper", "--non-interactive", "remove", pkg)
	sb.WriteString(out + "\n")

	postNum, _ := t.guard.CreatePostSnapshot(preNum, desc)
	if postNum > 0 {
		sb.WriteString(fmt.Sprintf("Post-snapshot #%d created\n", postNum))
	}

	if removeErr != nil {
		t.guard.RollbackToSnapshot(preNum)
		t.guard.ClearStateFile()
		sb.WriteString(fmt.Sprintf("ERROR: Remove failed, rolled back to #%d\n", preNum))
		return mcp.ToolErr(sb.String())
	}

	health := t.guard.CheckSystemHealth()
	sb.WriteString("\nHealth check:\n" + health.Details)

	if !health.OK {
		t.guard.RollbackToSnapshot(preNum)
		t.guard.ClearStateFile()
		sb.WriteString(fmt.Sprintf("WARN: GNOME unhealthy, rolled back to #%d\n", preNum))
		return mcp.ToolErr(sb.String())
	}

	t.guard.ClearStateFile()
	sb.WriteString(fmt.Sprintf("\n%s removed (snapshots #%d -> #%d)\n", pkg, preNum, postNum))
	return mcp.ToolOK(sb.String())
}
