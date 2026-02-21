package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/olindenern/leapdoctor/internal/guard"
	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) SystemHealth() mcp.ToolResult {
	var sb strings.Builder
	sb.WriteString("# System Health - openSUSE Leap 16\n\n")

	// Failed services
	out, _ := t.guard.Runner.Run("systemctl", "--failed", "--no-legend", "--plain")
	if out == "" {
		sb.WriteString("## Systemd\nOK: No failed services\n\n")
	} else {
		sb.WriteString("## Systemd\nFAIL: Failed services:\n```\n" + out + "\n```\n\n")
	}

	// GNOME shell
	out, err := t.guard.Runner.Run("systemctl", "is-active", "gdm")
	gdmOK := err == nil && strings.TrimSpace(out) == "active"
	if gdmOK {
		sb.WriteString("## GNOME\nOK: GDM active\n")
	} else {
		sb.WriteString("## GNOME\nFAIL: GDM unhealthy: " + out + "\n")
	}

	// Wayland session
	wayland, _ := t.guard.Runner.Run("loginctl", "show-session", "--property=Type", "--value")
	sb.WriteString(fmt.Sprintf("   Session type: %s\n\n", strings.TrimSpace(wayland)))

	// Flatpak
	out, _ = t.guard.Runner.Run("flatpak", "list", "--app", "--columns=name", "-q")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 0 && lines[0] != "" {
		sb.WriteString(fmt.Sprintf("## Flatpak\nOK: %d applications installed\n\n", len(lines)))
	}

	// SELinux
	seOut, _ := t.guard.Runner.Run("getenforce")
	sb.WriteString(fmt.Sprintf("## SELinux\nMode: %s\n", strings.TrimSpace(seOut)))

	denials, _ := t.guard.Runner.Run("bash", "-c", "ausearch -m avc -ts today 2>/dev/null | grep -c 'type=AVC' || echo 0")
	sb.WriteString(fmt.Sprintf("AVC denials today: %s\n\n", strings.TrimSpace(denials)))

	// Recent errors
	out, _ = t.guard.Runner.Run("journalctl", "-p", "err", "-n", "10", "--no-pager", "-q",
		"--output=short-monotonic")
	if out != "" {
		sb.WriteString("## Recent errors (journalctl)\n```\n" + out + "\n```\n")
	} else {
		sb.WriteString("## Recent errors\nOK: No errors in journal\n")
	}

	// Pending snapshot state
	if data, err := os.ReadFile(guard.StateFile); err == nil {
		snap := strings.TrimSpace(string(data))
		sb.WriteString(fmt.Sprintf(
			"\n## WARNING: Pending agent snapshot\nSnapshot #%s was not closed - possibly incomplete operation.\n"+
				"Rollback: suse_snapshot_rollback with number %s\n", snap, snap))
	}

	return mcp.ToolOK(sb.String())
}
