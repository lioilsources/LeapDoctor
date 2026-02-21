package tools

import (
	"fmt"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) Systemctl(action, service string) mcp.ToolResult {
	if action == "" || service == "" {
		return mcp.ToolErr("ERROR: 'action' and 'service' are required")
	}

	readOnly := map[string]bool{
		"status": true, "is-active": true, "is-enabled": true,
	}

	var out string
	var err error

	if readOnly[action] {
		out, err = t.guard.Runner.Run("systemctl", action, service)
	} else {
		if t.dryRun {
			return mcp.ToolOK(fmt.Sprintf("DRY-RUN MODE: Would run systemctl %s %s", action, service))
		}
		if e := t.rateLimit.CheckDestructive(); e != nil {
			return mcp.ToolErr(fmt.Sprintf("ERROR: Rate limit exceeded: %v", e))
		}
		out, err = t.guard.Runner.Run("sudo", "systemctl", action, service)
	}

	if err != nil && out == "" {
		return mcp.ToolErr(fmt.Sprintf("ERROR: systemctl %s %s failed: %v", action, service, err))
	}

	status := "OK"
	if err != nil {
		status = "WARN"
	}

	return mcp.ToolOK(fmt.Sprintf("%s: systemctl %s %s:\n```\n%s\n```", status, action, service, out))
}
