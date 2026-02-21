// Package exec provides an abstraction over os/exec for testability.
package exec

import (
	"os/exec"
	"strings"
)

// Runner abstracts command execution. Use SystemRunner in production
// and MockRunner in tests.
type Runner interface {
	Run(name string, args ...string) (string, error)
}

// SystemRunner executes real system commands via os/exec.
type SystemRunner struct{}

func (r *SystemRunner) Run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// CommandExists checks if a binary is available in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
