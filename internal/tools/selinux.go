package tools

import (
	"fmt"
	"strings"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) SelinuxStatus(showDenials bool, lines string) mcp.ToolResult {
	var sb strings.Builder
	sb.WriteString("# SELinux Status\n\n")

	// Mode
	out, _ := t.guard.Runner.Run("getenforce")
	sb.WriteString(fmt.Sprintf("Mode: **%s**\n", strings.TrimSpace(out)))

	// Detailed status
	out, _ = t.guard.Runner.Run("sestatus")
	sb.WriteString("```\n" + out + "\n```\n\n")

	if showDenials {
		n := "20"
		if lines != "" {
			n = lines
		}
		out, err := t.guard.Runner.Run("bash", "-c",
			fmt.Sprintf("ausearch -m avc -ts today 2>/dev/null | head -%s", n))
		if err != nil || strings.Contains(out, "no matches") {
			sb.WriteString("## AVC Denials\nOK: No denial records today\n")
		} else {
			sb.WriteString(fmt.Sprintf("## AVC Denials (last %s)\n```\n%s\n```\n\n", n, out))
			sb.WriteString("Tip: Run `audit2allow -a | audit2why` to analyze denials\n")
		}
	}

	return mcp.ToolOK(sb.String())
}
