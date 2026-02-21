// Package guard provides the SnapperGuard core: Btrfs snapshot management,
// system health checks, and state file handling for post-boot recovery.
package guard

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/olindenern/leapdoctor/internal/exec"
)

const (
	StateFile = "/var/lib/leapdoctor/last-snapshot"
	StateDir  = "/var/lib/leapdoctor"
	GnomePkgs = "gnome-shell,gdm,mutter,glib2,gtk3,gtk4,wayland,pipewire"
)

// SnapperGuard wraps system operations with Btrfs snapshots and health checks.
type SnapperGuard struct {
	Runner exec.Runner
}

func New(runner exec.Runner) *SnapperGuard {
	return &SnapperGuard{Runner: runner}
}

// ── Snapper operations ──────────────────────────────────────────────────────

func (g *SnapperGuard) CreatePreSnapshot(desc string) (int, error) {
	if !exec.CommandExists("snapper") {
		return 0, fmt.Errorf("snapper is not installed")
	}
	out, err := g.Runner.Run("snapper", "create",
		"--type", "pre",
		"--cleanup-algorithm", "timeline",
		"--print-number",
		"--description", "agent: "+desc,
	)
	if err != nil {
		return 0, fmt.Errorf("snapper create failed: %s (%w)", out, err)
	}
	num, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("invalid snapshot number: %q", out)
	}

	// Save for post-boot recovery (atomic write)
	if err := os.MkdirAll(StateDir, 0755); err != nil {
		return 0, fmt.Errorf("cannot create state dir: %w", err)
	}
	tmpFile := StateFile + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(strconv.Itoa(num)), 0644); err != nil {
		return 0, fmt.Errorf("cannot write state file: %w", err)
	}
	if err := os.Rename(tmpFile, StateFile); err != nil {
		return 0, fmt.Errorf("cannot rename state file: %w", err)
	}

	return num, nil
}

func (g *SnapperGuard) CreatePostSnapshot(preNum int, desc string) (int, error) {
	out, err := g.Runner.Run("snapper", "create",
		"--type", "post",
		"--pre-number", strconv.Itoa(preNum),
		"--print-number",
		"--description", "agent: "+desc+" (post)",
	)
	if err != nil {
		return 0, err
	}
	num, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("invalid post-snapshot number: %q", out)
	}
	return num, nil
}

func (g *SnapperGuard) RollbackToSnapshot(snapshotNum int) error {
	_, err := g.Runner.Run("snapper", "undochange",
		fmt.Sprintf("%d..0", snapshotNum),
	)
	return err
}

func (g *SnapperGuard) ClearStateFile() {
	os.Remove(StateFile)
}

// ── Health check ────────────────────────────────────────────────────────────

type HealthResult struct {
	OK      bool
	Details string
}

func (g *SnapperGuard) CheckSystemHealth() HealthResult {
	var sb strings.Builder
	allOK := true

	// 1. Failed systemd services
	out, _ := g.Runner.Run("systemctl", "--failed", "--no-legend", "--plain")
	if out == "" {
		sb.WriteString("OK: No failed systemd services\n")
	} else {
		sb.WriteString("FAIL: Failed services:\n" + out + "\n")
		allOK = false
	}

	// 2. GDM (GNOME Display Manager)
	out, err := g.Runner.Run("systemctl", "is-active", "gdm")
	if err != nil || strings.TrimSpace(out) != "active" {
		sb.WriteString(fmt.Sprintf("FAIL: GDM not active: %s\n", out))
		allOK = false
	} else {
		sb.WriteString("OK: GDM active\n")
	}

	// 3. NetworkManager
	out, err = g.Runner.Run("systemctl", "is-active", "NetworkManager")
	if err != nil || strings.TrimSpace(out) != "active" {
		sb.WriteString(fmt.Sprintf("WARN: NetworkManager: %s\n", out))
	} else {
		sb.WriteString("OK: NetworkManager active\n")
	}

	// 4. Recent journal errors
	out, _ = g.Runner.Run("journalctl", "-p", "err", "-n", "5", "--no-pager", "-q")
	if out != "" {
		sb.WriteString("\nWARN: Recent errors:\n" + out + "\n")
	}

	return HealthResult{OK: allOK, Details: sb.String()}
}

// BtrfsAvailable checks if root filesystem is Btrfs.
func (g *SnapperGuard) BtrfsAvailable() bool {
	out, err := g.Runner.Run("stat", "-f", "--format=%T", "/")
	return err == nil && strings.Contains(out, "btrfs")
}

// IsCriticalPackage checks if a package name matches GNOME critical packages.
func IsCriticalPackage(pkg string) bool {
	for _, critical := range strings.Split(GnomePkgs, ",") {
		if strings.Contains(pkg, critical) {
			return true
		}
	}
	return false
}
