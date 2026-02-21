package tools

import (
	"fmt"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) FlatpakInstall(appID, remote string, dryRun bool) mcp.ToolResult {
	if appID == "" {
		return mcp.ToolErr("ERROR: 'app_id' is required (e.g. 'org.gimp.GIMP')")
	}
	if remote == "" {
		remote = "flathub"
	}

	if dryRun {
		out, _ := t.guard.Runner.Run("flatpak", "search", appID)
		return mcp.ToolOK(fmt.Sprintf("DRY RUN - searching '%s':\n%s", appID, out))
	}

	out, err := t.guard.Runner.Run("flatpak", "install", "--assumeyes", remote, appID)
	if err != nil {
		return mcp.ToolErr(fmt.Sprintf("ERROR: Flatpak install failed: %s", out))
	}
	return mcp.ToolOK(fmt.Sprintf("%s installed from %s\n%s", appID, remote, out))
}
