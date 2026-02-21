package tools

import (
	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) Journalctl(service, priority, lines, since string) mcp.ToolResult {
	args := []string{"--no-pager", "-q"}

	if service != "" {
		args = append(args, "-u", service)
	}
	if priority != "" {
		args = append(args, "-p", priority)
	}
	if since != "" {
		args = append(args, "-S", since)
	}

	n := "50"
	if lines != "" {
		n = lines
	}
	args = append(args, "-n", n)

	out, err := t.guard.Runner.Run("journalctl", args...)
	if err != nil && out == "" {
		return mcp.ToolErr("ERROR: journalctl failed: " + err.Error())
	}
	if out == "" {
		return mcp.ToolOK("OK: No entries matching the given criteria")
	}

	header := "Journal"
	if service != "" {
		header += " [" + service + "]"
	}
	if priority != "" {
		header += " (>=" + priority + ")"
	}
	return mcp.ToolOK(header + ":\n```\n" + out + "\n```")
}
