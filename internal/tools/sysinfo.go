package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/olindenern/leapdoctor/internal/mcp"
)

func (t *Toolbox) SystemInfo() mcp.ToolResult {
	var sb strings.Builder
	sb.WriteString("# System Information - openSUSE Leap\n\n")

	// OS release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		sb.WriteString("## OS\n```\n")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "NAME=") ||
				strings.HasPrefix(line, "VERSION=") ||
				strings.HasPrefix(line, "VERSION_ID=") ||
				strings.HasPrefix(line, "PRETTY_NAME=") ||
				strings.HasPrefix(line, "ID=") {
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("```\n\n")
	}

	// Kernel
	out, _ := t.guard.Runner.Run("uname", "-r")
	sb.WriteString(fmt.Sprintf("## Kernel\n%s\n\n", out))

	// GNOME version
	out, _ = t.guard.Runner.Run("gnome-shell", "--version")
	sb.WriteString(fmt.Sprintf("## GNOME\n%s\n", out))

	// Session type (Wayland/X11)
	out, _ = t.guard.Runner.Run("loginctl", "show-session", "--property=Type", "--value")
	sb.WriteString(fmt.Sprintf("Session type: %s\n\n", strings.TrimSpace(out)))

	// CPU
	out, _ = t.guard.Runner.Run("lscpu")
	sb.WriteString("## CPU\n```\n")
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Model name:") ||
			strings.HasPrefix(line, "CPU(s):") ||
			strings.HasPrefix(line, "Architecture:") ||
			strings.HasPrefix(line, "Thread(s) per core:") ||
			strings.HasPrefix(line, "Core(s) per socket:") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("```\n\n")

	// RAM/swap
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		sb.WriteString("## Memory\n```\n")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") ||
				strings.HasPrefix(line, "MemAvailable:") ||
				strings.HasPrefix(line, "SwapTotal:") ||
				strings.HasPrefix(line, "SwapFree:") {
				sb.WriteString(line + "\n")
			}
		}
		sb.WriteString("```\n\n")
	}

	// Disk / Btrfs
	out, _ = t.guard.Runner.Run("df", "-h", "/")
	sb.WriteString("## Disk\n```\n" + out + "\n```\n")

	btrfsOut, err := t.guard.Runner.Run("btrfs", "filesystem", "usage", "/")
	if err == nil {
		sb.WriteString("### Btrfs\n```\n" + btrfsOut + "\n```\n")
	}
	sb.WriteString("\n")

	// GPU + driver
	out, _ = t.guard.Runner.Run("lspci")
	sb.WriteString("## GPU\n```\n")
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "vga") || strings.Contains(lower, "3d") || strings.Contains(lower, "display") {
			sb.WriteString(line + "\n")
		}
	}
	sb.WriteString("```\n")

	// Loaded GPU driver
	out, _ = t.guard.Runner.Run("lsmod")
	drivers := []string{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			mod := fields[0]
			if mod == "nvidia" || mod == "amdgpu" || mod == "i915" || mod == "nouveau" || mod == "radeon" {
				drivers = append(drivers, mod)
			}
		}
	}
	if len(drivers) > 0 {
		sb.WriteString(fmt.Sprintf("Loaded GPU drivers: %s\n\n", strings.Join(drivers, ", ")))
	} else {
		sb.WriteString("Loaded GPU drivers: (none detected)\n\n")
	}

	// Key packages
	out, _ = t.guard.Runner.Run("rpm", "-qa", "--qf", "%{NAME}-%{VERSION}\n",
		"gnome-shell", "mutter", "gdm", "pipewire", "flatpak", "snapper")
	if out != "" {
		sb.WriteString("## Key packages\n```\n" + out + "\n```\n")
	}

	return mcp.ToolOK(sb.String())
}
