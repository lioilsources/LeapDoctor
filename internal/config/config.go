// Package config loads and validates LeapDoctor configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DryRun    bool        `json:"dry_run"`
	RateLimit RateLimit   `json:"rate_limit"`
	Safety    Safety      `json:"safety"`
	Recovery  Recovery    `json:"recovery"`
}

type RateLimit struct {
	MaxDestructivePer30Min int `json:"max_destructive_per_30min"`
	MaxRollbacksBeforeLock int `json:"max_rollbacks_before_lockout"`
}

type Safety struct {
	BlockGnomeCritical bool     `json:"block_gnome_critical"`
	CriticalPackages   []string `json:"critical_packages"`
}

type Recovery struct {
	Enabled              bool   `json:"enabled"`
	GDMWaitTimeout       string `json:"gdm_wait_timeout"`
	AutoRebootOnRollback bool   `json:"auto_reboot_on_rollback"`
}

// Default returns the default configuration.
func Default() Config {
	return Config{
		DryRun: false,
		RateLimit: RateLimit{
			MaxDestructivePer30Min: 10,
			MaxRollbacksBeforeLock: 3,
		},
		Safety: Safety{
			BlockGnomeCritical: true,
			CriticalPackages:   []string{"gnome-shell", "gdm", "mutter", "glib2", "gtk3", "gtk4", "wayland", "pipewire"},
		},
		Recovery: Recovery{
			Enabled:              true,
			GDMWaitTimeout:       "60s",
			AutoRebootOnRollback: false,
		},
	}
}

// Load reads config from /etc/leapdoctor/config.json, falling back to defaults.
func Load() Config {
	cfg := Default()

	paths := []string{
		"/etc/leapdoctor/config.json",
		filepath.Join(os.Getenv("HOME"), ".config", "leapdoctor", "config.json"),
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			fmt.Fprintf(os.Stderr, "[leapdoctor] WARNING: invalid config %s: %v\n", p, err)
		}
		break
	}

	return cfg
}

// McphostSnippet returns a YAML snippet for ~/.mcphost.yml.
func McphostSnippet(binaryPath string) string {
	return fmt.Sprintf(`mcpServers:
  leapdoctor:
    command: %s
`, binaryPath)
}

// ClaudeCodeSnippet returns a JSON snippet for ~/.claude.json.
func ClaudeCodeSnippet(binaryPath string) string {
	return fmt.Sprintf(`{
  "mcpServers": {
    "leapdoctor": {
      "command": "%s",
      "type": "stdio"
    }
  }
}`, binaryPath)
}

// MergeMcphostConfig adds LeapDoctor to existing ~/.mcphost.yml.
func MergeMcphostConfig(binaryPath string) error {
	path := filepath.Join(os.Getenv("HOME"), ".mcphost.yml")
	existing, _ := os.ReadFile(path)

	// Check specifically for leapdoctor as a YAML key (indented under mcpServers)
	for _, line := range strings.Split(string(existing), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "leapdoctor:" || strings.HasPrefix(trimmed, "leapdoctor:") {
			return fmt.Errorf("leapdoctor already configured in %s", path)
		}
	}

	snippet := fmt.Sprintf("\n  leapdoctor:\n    command: %s\n", binaryPath)

	if strings.Contains(string(existing), "mcpServers:") {
		// Append under existing mcpServers
		content := strings.Replace(string(existing), "mcpServers:", "mcpServers:"+snippet, 1)
		return os.WriteFile(path, []byte(content), 0644)
	}

	// Create new or append section
	content := string(existing) + "\nmcpServers:" + snippet
	return os.WriteFile(path, []byte(content), 0644)
}

// MergeClaudeConfig adds LeapDoctor to existing ~/.claude.json.
func MergeClaudeConfig(binaryPath string) error {
	path := filepath.Join(os.Getenv("HOME"), ".claude.json")
	existing, _ := os.ReadFile(path)

	if len(existing) == 0 {
		return os.WriteFile(path, []byte(ClaudeCodeSnippet(binaryPath)), 0644)
	}

	// Parse existing JSON and merge
	var obj map[string]interface{}
	if err := json.Unmarshal(existing, &obj); err != nil {
		return fmt.Errorf("cannot parse %s: %w", path, err)
	}

	servers, _ := obj["mcpServers"].(map[string]interface{})
	if servers == nil {
		servers = make(map[string]interface{})
	}
	if _, exists := servers["leapdoctor"]; exists {
		return fmt.Errorf("leapdoctor already configured in %s", path)
	}
	servers["leapdoctor"] = map[string]interface{}{
		"command": binaryPath,
		"type":    "stdio",
	}
	obj["mcpServers"] = servers

	data, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
