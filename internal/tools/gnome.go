package tools

import (
	"fmt"
	"strings"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) GnomeConfigGet(schema, key string) mcp.ToolResult {
	if schema == "" {
		return mcp.ToolErr("ERROR: 'schema' is required (e.g. 'org.gnome.desktop.interface')")
	}
	var out string
	var err error
	if key == "" {
		out, err = t.guard.Runner.Run("gsettings", "list-recursively", schema)
	} else {
		out, err = t.guard.Runner.Run("gsettings", "get", schema, key)
	}
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: gsettings get failed: %s", out))
	}
	return mcp.ToolOK(fmt.Sprintf("%s %s:\n```\n%s\n```", schema, key, out))
}

func (t *Toolbox) GnomeConfigSet(schema, key, value string) mcp.ToolResult {
	if schema == "" || key == "" || value == "" {
		return mcp.ToolErr("ERROR: 'schema', 'key' and 'value' are required")
	}

	// Read current value for logging
	currentOut, _ := t.guard.Runner.Run("gsettings", "get", schema, key)

	out, err := t.guard.Runner.Run("gsettings", "set", schema, key, value)
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf(
			"ERROR: gsettings set failed: %s\nTip: Check value format (strings need quotes: \"'dark'\")", out))
	}

	return mcp.ToolOK(fmt.Sprintf(
		"Set %s %s\nPrevious value: %s\nNew value:      %s",
		schema, key,
		strings.TrimSpace(currentOut),
		value,
	))
}
