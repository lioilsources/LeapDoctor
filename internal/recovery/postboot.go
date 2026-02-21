// Package recovery implements post-boot snapshot recovery logic.
package recovery

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/olindenern/leapdoctor/internal/guard"
)

// PostBootCheck reads the pending snapshot state file and determines
// if an automatic rollback is needed after a reboot.
func PostBootCheck(g *guard.SnapperGuard) {
	data, err := os.ReadFile(guard.StateFile)
	if err != nil {
		// No pending snapshot = all OK
		return
	}

	snapshotNum, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || snapshotNum == 0 {
		os.Remove(guard.StateFile)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[leapdoctor] WARNING: Found pending snapshot #%d from previous session!\n",
		snapshotNum,
	)

	// Wait for GDM (max 60s)
	for i := 0; i < 12; i++ {
		out, err := g.Runner.Run("systemctl", "is-active", "gdm")
		if err == nil && strings.TrimSpace(out) == "active" {
			break
		}
		fmt.Fprintf(os.Stderr, "[leapdoctor] Waiting for GDM (%d/12)...\n", i+1)
		time.Sleep(5 * time.Second)
	}

	health := g.CheckSystemHealth()

	if health.OK {
		fmt.Fprintf(os.Stderr,
			"[leapdoctor] OK: System healthy after restart, snapshot #%d preserved\n",
			snapshotNum,
		)
		os.Remove(guard.StateFile)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[leapdoctor] FAIL: GNOME unhealthy - automatic rollback to snapshot #%d\n",
		snapshotNum,
	)

	if err := g.RollbackToSnapshot(snapshotNum); err != nil {
		fmt.Fprintf(os.Stderr,
			"[leapdoctor] ERROR: Rollback failed: %v\nManual fix: snapper undochange %d..0\n",
			err, snapshotNum,
		)
		// Try GNOME notification
		g.Runner.Run("notify-send", "--urgency=critical",
			"LeapDoctor",
			fmt.Sprintf("Automatic rollback to snapshot #%d failed!\nManual fix: snapper undochange %d..0",
				snapshotNum, snapshotNum),
		)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[leapdoctor] OK: Rollback to snapshot #%d successful.\n"+
			"A reboot is recommended. Run: sudo systemctl reboot\n",
		snapshotNum,
	)
	os.Remove(guard.StateFile)

	// Notify user instead of auto-rebooting
	g.Runner.Run("notify-send", "--urgency=critical",
		"LeapDoctor",
		fmt.Sprintf("System rolled back to snapshot #%d due to health issues.\nPlease reboot: sudo systemctl reboot",
			snapshotNum),
	)
}
