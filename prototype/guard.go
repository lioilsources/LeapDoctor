// guard.go – Jádro bezpečnostní logiky: Snapper + systémové operace

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	stateFile   = "/var/lib/suse-agent-guard/last-snapshot"
	stateDir    = "/var/lib/suse-agent-guard"
	gnomePkgs   = "gnome-shell,gdm,mutter,glib2,gtk3,gtk4,wayland,pipewire"
)

// SnapperGuard obaluje systémové operace snapshoty a health checky
type SnapperGuard struct{}

func NewSnapperGuard() *SnapperGuard {
	return &SnapperGuard{}
}

// ── Pomocné funkce ─────────────────────────────────────────────────────────

func runCmd(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func toolOK(text string) ToolResult {
	return ToolResult{Content: []ContentItem{{Type: "text", Text: text}}}
}

func toolErr(text string) ToolResult {
	return ToolResult{Content: []ContentItem{{Type: "text", Text: text}}, IsError: true}
}

func snapperAvailable() bool {
	_, err := exec.LookPath("snapper")
	return err == nil
}

func btrfsAvailable() bool {
	out, err := runCmd("stat", "-f", "--format=%T", "/")
	return err == nil && strings.Contains(out, "btrfs")
}

// ── Snapper operace ────────────────────────────────────────────────────────

func (g *SnapperGuard) createPreSnapshot(desc string) (int, error) {
	if !snapperAvailable() {
		return 0, fmt.Errorf("snapper není nainstalován")
	}
	out, err := runCmd("snapper", "create",
		"--type", "pre",
		"--cleanup-algorithm", "timeline",
		"--print-number",
		"--description", "agent: "+desc,
	)
	if err != nil {
		return 0, fmt.Errorf("snapper create selhal: %s (%w)", out, err)
	}
	num, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0, fmt.Errorf("neplatné číslo snapshotu: %q", out)
	}

	// Ulož pro post-boot recovery
	os.MkdirAll(stateDir, 0755)
	os.WriteFile(stateFile, []byte(strconv.Itoa(num)), 0644)

	return num, nil
}

func (g *SnapperGuard) createPostSnapshot(preNum int, desc string) (int, error) {
	out, err := runCmd("snapper", "create",
		"--type", "post",
		"--pre-number", strconv.Itoa(preNum),
		"--print-number",
		"--description", "agent: "+desc+" (post)",
	)
	if err != nil {
		return 0, err
	}
	num, _ := strconv.Atoi(strings.TrimSpace(out))
	return num, nil
}

func (g *SnapperGuard) rollbackToSnapshot(snapshotNum int) error {
	_, err := runCmd("snapper", "undochange",
		fmt.Sprintf("%d..0", snapshotNum),
	)
	return err
}

func (g *SnapperGuard) clearStateFile() {
	os.Remove(stateFile)
}

// ── Health check ───────────────────────────────────────────────────────────

type healthResult struct {
	ok      bool
	details string
}

func (g *SnapperGuard) checkSystemHealth() healthResult {
	var sb strings.Builder
	allOK := true

	// 1. Selhané systemd služby
	out, _ := runCmd("systemctl", "--failed", "--no-legend", "--plain")
	if out == "" || out == "(prázdné)" {
		sb.WriteString("✅ Žádné selhané systemd služby\n")
	} else {
		sb.WriteString("❌ Selhané služby:\n" + out + "\n")
		allOK = false
	}

	// 2. GDM (GNOME Display Manager)
	out, err := runCmd("systemctl", "is-active", "gdm")
	if err != nil || strings.TrimSpace(out) != "active" {
		sb.WriteString(fmt.Sprintf("❌ GDM není aktivní: %s\n", out))
		allOK = false
	} else {
		sb.WriteString("✅ GDM aktivní\n")
	}

	// 3. NetworkManager
	out, err = runCmd("systemctl", "is-active", "NetworkManager")
	if err != nil || strings.TrimSpace(out) != "active" {
		sb.WriteString(fmt.Sprintf("⚠️  NetworkManager: %s\n", out))
	} else {
		sb.WriteString("✅ NetworkManager aktivní\n")
	}

	// 4. Posledních 5 chyb z journalu
	out, _ = runCmd("journalctl", "-p", "err", "-n", "5", "--no-pager", "-q")
	if out != "" {
		sb.WriteString("\n⚠️  Poslední chyby:\n" + out + "\n")
	}

	return healthResult{ok: allOK, details: sb.String()}
}

// ── Nástroje (volané z main.go) ────────────────────────────────────────────

func (g *SnapperGuard) ZypperInstall(pkg string, dryRun bool) ToolResult {
	if pkg == "" {
		return toolErr("❌ Parametr 'package' je povinný")
	}

	if dryRun {
		out, _ := runCmd("zypper", "--non-interactive", "--dry-run", "install", pkg)
		return toolOK(fmt.Sprintf("🔍 DRY RUN – zypper install %s:\n%s", pkg, out))
	}

	if !btrfsAvailable() {
		return toolErr("❌ Btrfs není k dispozici – Snapper snapshoty nejsou možné. Přerušuji.")
	}

	desc := fmt.Sprintf("zypper install %s @ %s", pkg, time.Now().Format("2006-01-02 15:04"))

	// Pre-snapshot
	preNum, err := g.createPreSnapshot(desc)
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Nelze vytvořit pre-snapshot: %v\nInstalace přerušena.", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📸 Pre-snapshot #%d vytvořen\n", preNum))
	sb.WriteString(fmt.Sprintf("⚙️  Instaluji: %s\n\n", pkg))

	// Instalace
	out, installErr := runCmd("zypper", "--non-interactive", "install", pkg)
	sb.WriteString(out + "\n\n")

	// Post-snapshot
	postNum, _ := g.createPostSnapshot(preNum, desc)
	if postNum > 0 {
		sb.WriteString(fmt.Sprintf("📸 Post-snapshot #%d vytvořen\n", postNum))
	}

	if installErr != nil {
		// Rollback
		sb.WriteString(fmt.Sprintf("\n❌ Instalace selhala!\n"))
		if rbErr := g.rollbackToSnapshot(preNum); rbErr != nil {
			sb.WriteString(fmt.Sprintf("❌ Rollback TAKÉ selhal: %v\nManuální rollback: snapper undochange %d..0\n", rbErr, preNum))
		} else {
			sb.WriteString(fmt.Sprintf("🔄 Rollback na snapshot #%d proveden\n", preNum))
			g.clearStateFile()
		}
		return toolErr(sb.String())
	}

	// Health check
	health := g.checkSystemHealth()
	sb.WriteString("\n🏥 Health check:\n" + health.details)

	if !health.ok {
		sb.WriteString(fmt.Sprintf("\n⚠️  Systém nezdravý po instalaci! Rollback na snapshot #%d...\n", preNum))
		if rbErr := g.rollbackToSnapshot(preNum); rbErr != nil {
			sb.WriteString(fmt.Sprintf("❌ Rollback selhal: %v\n", rbErr))
		} else {
			sb.WriteString(fmt.Sprintf("🔄 Rollback proveden. Doporučuji restart: sudo systemctl reboot\n"))
			g.clearStateFile()
		}
		return toolErr(sb.String())
	}

	g.clearStateFile()
	sb.WriteString(fmt.Sprintf("\n✅ %s úspěšně nainstalován (snapshots #%d → #%d)\n", pkg, preNum, postNum))
	return toolOK(sb.String())
}

func (g *SnapperGuard) ZypperRemove(pkg string, dryRun bool) ToolResult {
	if pkg == "" {
		return toolErr("❌ Parametr 'package' je povinný")
	}

	// Varování pro kritické GNOME balíčky
	for _, critical := range strings.Split(gnomePkgs, ",") {
		if strings.Contains(pkg, critical) {
			return toolErr(fmt.Sprintf(
				"⛔ ODMÍTNUTO: '%s' je kritický GNOME balíček!\n"+
					"Odstranění by mohlo znefunkčnit desktop prostředí.\n"+
					"Pokud jsi si jist, proveď to manuálně.",
				pkg,
			))
		}
	}

	if dryRun {
		out, _ := runCmd("zypper", "--non-interactive", "--dry-run", "remove", pkg)
		return toolOK(fmt.Sprintf("🔍 DRY RUN – zypper remove %s:\n%s", pkg, out))
	}

	desc := fmt.Sprintf("zypper remove %s @ %s", pkg, time.Now().Format("2006-01-02 15:04"))

	preNum, err := g.createPreSnapshot(desc)
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Nelze vytvořit pre-snapshot: %v", err))
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📸 Pre-snapshot #%d vytvořen\n⚙️  Odstraňuji: %s\n\n", preNum, pkg))

	out, removeErr := runCmd("zypper", "--non-interactive", "remove", pkg)
	sb.WriteString(out + "\n")

	postNum, _ := g.createPostSnapshot(preNum, desc)
	if postNum > 0 {
		sb.WriteString(fmt.Sprintf("📸 Post-snapshot #%d vytvořen\n", postNum))
	}

	if removeErr != nil {
		g.rollbackToSnapshot(preNum)
		g.clearStateFile()
		sb.WriteString(fmt.Sprintf("❌ Odstranění selhalo, rollback na #%d proveden\n", preNum))
		return toolErr(sb.String())
	}

	health := g.checkSystemHealth()
	sb.WriteString("\n🏥 Health check:\n" + health.details)

	if !health.ok {
		g.rollbackToSnapshot(preNum)
		g.clearStateFile()
		sb.WriteString(fmt.Sprintf("⚠️  GNOME nezdravý, rollback na #%d proveden\n", preNum))
		return toolErr(sb.String())
	}

	g.clearStateFile()
	sb.WriteString(fmt.Sprintf("\n✅ %s odstraněn (snapshots #%d → #%d)\n", pkg, preNum, postNum))
	return toolOK(sb.String())
}

func (g *SnapperGuard) SnapshotCreate(desc string) ToolResult {
	if desc == "" {
		desc = fmt.Sprintf("manuální snapshot @ %s", time.Now().Format("2006-01-02 15:04"))
	}
	out, err := runCmd("snapper", "create",
		"--cleanup-algorithm", "timeline",
		"--print-number",
		"--description", desc,
	)
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Snapshot selhal: %s", out))
	}
	return toolOK(fmt.Sprintf("✅ Snapshot #%s vytvořen: %s", strings.TrimSpace(out), desc))
}

func (g *SnapperGuard) SnapshotList() ToolResult {
	out, err := runCmd("snapper", "list", "--columns", "number,type,date,description")
	if err != nil {
		return toolErr(fmt.Sprintf("❌ snapper list selhal: %s", out))
	}
	return toolOK("📋 Snapper snapshoty:\n\n" + out)
}

func (g *SnapperGuard) SnapshotRollback(snapshotNum string) ToolResult {
	if snapshotNum == "" {
		return toolErr("❌ 'snapshot_number' je povinný. Použij suse_snapshot_list pro zobrazení dostupných snapshotů.")
	}
	num, err := strconv.Atoi(strings.TrimSpace(snapshotNum))
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Neplatné číslo snapshotu: %q", snapshotNum))
	}
	out, err := runCmd("snapper", "undochange", fmt.Sprintf("%d..0", num))
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Rollback na snapshot #%d selhal: %s", num, out))
	}
	return toolOK(fmt.Sprintf(
		"🔄 Rollback na snapshot #%d proveden.\n%s\n\n"+
			"⚠️  Pro dokončení rollbacku je nutný restart:\n   sudo systemctl reboot",
		num, out,
	))
}

func (g *SnapperGuard) SystemHealth() ToolResult {
	var sb strings.Builder
	sb.WriteString("# 🏥 Zdraví systému – openSUSE Leap 16\n\n")

	// Selhané služby
	out, _ := runCmd("systemctl", "--failed", "--no-legend", "--plain")
	if out == "" {
		sb.WriteString("## Systemd\n✅ Žádné selhané služby\n\n")
	} else {
		sb.WriteString("## Systemd\n❌ Selhané služby:\n```\n" + out + "\n```\n\n")
	}

	// GNOME shell
	out, err := runCmd("systemctl", "is-active", "gdm")
	gdmOK := err == nil && strings.TrimSpace(out) == "active"
	if gdmOK {
		sb.WriteString("## GNOME\n✅ GDM aktivní\n")
	} else {
		sb.WriteString("## GNOME\n❌ GDM nezdravý: " + out + "\n")
	}

	// Wayland session
	wayland, _ := runCmd("loginctl", "show-session", "--property=Type", "--value")
	sb.WriteString(fmt.Sprintf("   Session type: %s\n\n", strings.TrimSpace(wayland)))

	// Flatpak
	out, _ = runCmd("flatpak", "list", "--app", "--columns=name", "-q")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) > 0 && lines[0] != "" {
		sb.WriteString(fmt.Sprintf("## Flatpak\n✅ %d aplikací nainstalováno\n\n", len(lines)))
	}

	// SELinux
	seOut, _ := runCmd("getenforce")
	sb.WriteString(fmt.Sprintf("## SELinux\nRežim: %s\n", strings.TrimSpace(seOut)))

	// Denial count
	denials, _ := runCmd("bash", "-c", "ausearch -m avc -ts today 2>/dev/null | grep -c 'type=AVC' || echo 0")
	sb.WriteString(fmt.Sprintf("AVC denial záznamy dnes: %s\n\n", strings.TrimSpace(denials)))

	// Poslední chyby
	out, _ = runCmd("journalctl", "-p", "err", "-n", "10", "--no-pager", "-q",
		"--output=short-monotonic")
	if out != "" {
		sb.WriteString("## Poslední chyby (journalctl)\n```\n" + out + "\n```\n")
	} else {
		sb.WriteString("## Poslední chyby\n✅ Žádné chyby v journalu\n")
	}

	// Snapper stav
	if data, err := os.ReadFile(stateFile); err == nil {
		sb.WriteString(fmt.Sprintf(
			"\n## ⚠️  Nedokončený agent snapshot\nSnapshot #%s nebyl uzavřen – možná nedokončená operace.\n"+
				"Rollback: suse_snapshot_rollback s číslem %s\n",
			strings.TrimSpace(string(data)),
			strings.TrimSpace(string(data)),
		))
	}

	return toolOK(sb.String())
}

func (g *SnapperGuard) Journalctl(service, priority, lines, since string) ToolResult {
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

	out, err := runCmd("journalctl", args...)
	if err != nil && out == "" {
		return toolErr("❌ journalctl selhal: " + err.Error())
	}
	if out == "" {
		return toolOK("✅ Žádné záznamy pro zadaná kritéria")
	}

	header := "📋 Journal"
	if service != "" {
		header += " [" + service + "]"
	}
	if priority != "" {
		header += " (≥" + priority + ")"
	}
	return toolOK(header + ":\n```\n" + out + "\n```")
}

func (g *SnapperGuard) GnomeConfigGet(schema, key string) ToolResult {
	if schema == "" {
		return toolErr("❌ 'schema' je povinný (např. 'org.gnome.desktop.interface')")
	}
	var out string
	var err error
	if key == "" {
		out, err = runCmd("gsettings", "list-recursively", schema)
	} else {
		out, err = runCmd("gsettings", "get", schema, key)
	}
	if err != nil {
		return toolErr(fmt.Sprintf("❌ gsettings get selhal: %s", out))
	}
	return toolOK(fmt.Sprintf("🔧 %s %s:\n```\n%s\n```", schema, key, out))
}

func (g *SnapperGuard) GnomeConfigSet(schema, key, value string) ToolResult {
	if schema == "" || key == "" || value == "" {
		return toolErr("❌ 'schema', 'key' a 'value' jsou povinné")
	}

	// Přečti aktuální hodnotu pro log
	currentOut, _ := runCmd("gsettings", "get", schema, key)

	out, err := runCmd("gsettings", "set", schema, key, value)
	if err != nil {
		return toolErr(fmt.Sprintf("❌ gsettings set selhal: %s\nTip: Zkontroluj formát hodnoty (string musí být v uvozovkách: \"'dark'\")", out))
	}

	return toolOK(fmt.Sprintf(
		"✅ Nastaveno %s %s\nPředchozí hodnota: %s\nNová hodnota:     %s",
		schema, key,
		strings.TrimSpace(currentOut),
		value,
	))
}

func (g *SnapperGuard) FlatpakInstall(appID, remote string, dryRun bool) ToolResult {
	if appID == "" {
		return toolErr("❌ 'app_id' je povinný (např. 'org.gimp.GIMP')")
	}
	if remote == "" {
		remote = "flathub"
	}

	args := []string{"install", "--assumeyes", remote, appID}
	if dryRun {
		out, _ := runCmd("flatpak", "search", appID)
		return toolOK(fmt.Sprintf("🔍 DRY RUN – hledám '%s':\n%s", appID, out))
	}

	out, err := runCmd("flatpak", args...)
	if err != nil {
		return toolErr(fmt.Sprintf("❌ Flatpak install selhal: %s\n\nTip: Zkontroluj App ID na https://flathub.org/apps/%s", out, appID))
	}
	return toolOK(fmt.Sprintf("✅ %s nainstalován z %s\n%s", appID, remote, out))
}

func (g *SnapperGuard) SelinuxStatus(showDenials bool, lines string) ToolResult {
	var sb strings.Builder
	sb.WriteString("# 🔒 SELinux status\n\n")

	// Režim
	out, _ := runCmd("getenforce")
	sb.WriteString(fmt.Sprintf("Režim: **%s**\n", strings.TrimSpace(out)))

	// Detailed status
	out, _ = runCmd("sestatus")
	sb.WriteString("```\n" + out + "\n```\n\n")

	if showDenials {
		n := "20"
		if lines != "" {
			n = lines
		}
		out, err := runCmd("bash", "-c",
			fmt.Sprintf("ausearch -m avc -ts today 2>/dev/null | head -%s", n))
		if err != nil || strings.Contains(out, "no matches") {
			sb.WriteString("## AVC Denials\n✅ Žádné denial záznamy dnes\n")
		} else {
			sb.WriteString(fmt.Sprintf("## AVC Denials (posledních %s)\n```\n%s\n```\n\n", n, out))
			sb.WriteString("💡 Pro povolení pravidla: `audit2allow -a | audit2why`\n")
		}
	}

	return toolOK(sb.String())
}

func (g *SnapperGuard) Systemctl(action, service string) ToolResult {
	if action == "" || service == "" {
		return toolErr("❌ 'action' a 'service' jsou povinné")
	}

	// Read-only akce nevyžadují sudo
	readOnly := map[string]bool{
		"status": true, "is-active": true, "is-enabled": true,
	}

	var out string
	var err error

	if readOnly[action] {
		out, err = runCmd("systemctl", action, service)
	} else {
		// Destruktivní akce – potřebuje sudo/polkit
		out, err = runCmd("sudo", "systemctl", action, service)
	}

	if err != nil && out == "" {
		return toolErr(fmt.Sprintf("❌ systemctl %s %s selhal: %v", action, service, err))
	}

	icon := "✅"
	if err != nil {
		icon = "⚠️ "
	}

	return toolOK(fmt.Sprintf("%s systemctl %s %s:\n```\n%s\n```", icon, action, service, out))
}

// ── Post-boot recovery ─────────────────────────────────────────────────────

func (g *SnapperGuard) PostBootRecovery() {
	data, err := os.ReadFile(stateFile)
	if err != nil {
		// Žádný čekající snapshot = vše OK
		return
	}

	snapshotNum, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || snapshotNum == 0 {
		os.Remove(stateFile)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[suse-agent-guard] ⚠️  Nalezen nedokončený snapshot #%d z předchozí session!\n",
		snapshotNum,
	)

	// Počkej na GDM (max 30s)
	for i := 0; i < 6; i++ {
		out, err := runCmd("systemctl", "is-active", "gdm")
		if err == nil && strings.TrimSpace(out) == "active" {
			break
		}
		fmt.Fprintf(os.Stderr, "[suse-agent-guard] Čekám na GDM (%d/6)...\n", i+1)
		time.Sleep(5 * time.Second)
	}

	health := g.checkSystemHealth()

	if health.ok {
		fmt.Fprintf(os.Stderr,
			"[suse-agent-guard] ✅ Systém zdravý po restartu, snapshot #%d zůstane\n",
			snapshotNum,
		)
		os.Remove(stateFile)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[suse-agent-guard] ❌ GNOME nezdravý – automatický rollback na snapshot #%d\n",
		snapshotNum,
	)

	if err := g.rollbackToSnapshot(snapshotNum); err != nil {
		fmt.Fprintf(os.Stderr,
			"[suse-agent-guard] ❌ Rollback selhal: %v\nManuální oprava: snapper undochange %d..0\n",
			err, snapshotNum,
		)
		// Pošli GNOME notifikaci pokud GNOME alespoň trochu běží
		runCmd("notify-send", "--urgency=critical",
			"SUSE Agent Guard",
			fmt.Sprintf("❌ Automatický rollback na snapshot #%d selhal!\nManuální oprava: snapper undochange %d..0",
				snapshotNum, snapshotNum),
		)
		return
	}

	fmt.Fprintf(os.Stderr,
		"[suse-agent-guard] ✅ Rollback na snapshot #%d úspěšný. Restartuji...\n",
		snapshotNum,
	)
	os.Remove(stateFile)

	// Dej uživateli 10s na přečtení
	time.Sleep(10 * time.Second)
	runCmd("systemctl", "reboot")
}
