# LeapDoctor - MCP rozšíření Claude Code pro SUSE Leap 16.0

## Kontext

**Problém:** Claude Code nezná konkrétní Linux systém uživatele. Navrhuje řešení, která nejsou kompatibilní se setupem. Nemá safety net — může rozbít desktop a není cesta zpět.

**Řešení:** MCP server (Go binary), který Claude Code připojí přes stdio. Dá mu:
1. **Znalost systému** — detailní info o OS, GNOME, GPU, balíčcích
2. **Safety net** — každá destruktivní operace obalená Btrfs/snapper snapshoty s auto-rollback
3. **Rozcestník** — přístup ke config souborům, logům, systemd službám

**Prototyp:** `prototype/` — funkční MCP server (Go, stdlib only), 12 nástrojů, snapper integrace, post-boot recovery. Vše v češtině.

**Scope v1:**
- Restrukturalizace prototypu do čistých Go packages
- Tool popisy v angličtině pro LLM
- Nový `suse_system_info` tool (Claude musí znát systém)
- Safety hardening (rate limit, loop detekce, atomické zápisy)
- Distribuce přes `make install`

**Odloženo na v2:** RPM/OBS, network/display/audio/firewall diagnostika

---

## Fáze 1: Restrukturalizace kódu

### Adresářová struktura
```
leapdoctor/
  cmd/leapdoctor/main.go           -- entry point, flagy
  internal/
    mcp/server.go, types.go        -- JSON-RPC 2.0 over stdio
    guard/guard.go                  -- SnapperGuard (snapshot, health, rate limit)
    tools/                          -- tool implementace
      zypper.go                     -- suse_zypper_install, suse_zypper_remove
      snapshot.go                   -- suse_snapshot_create/list/rollback
      health.go                     -- suse_system_health
      journal.go                    -- suse_journalctl
      gnome.go                      -- suse_gnome_config_get/set
      flatpak.go                    -- suse_flatpak_install
      selinux.go                    -- suse_selinux_status
      systemctl.go                  -- suse_systemctl
      sysinfo.go                    -- suse_system_info (NEW)
    exec/runner.go, mock.go         -- Runner interface + mock pro testy
    recovery/postboot.go            -- post-boot recovery
    ratelimit/limiter.go            -- rate limiting
    config/config.go                -- YAML konfigurace
  dist/
    leapdoctor-recovery.service     -- systemd unit
    leapdoctor.conf.default         -- default config
  go.mod
  Makefile
```

### Klíčová změna: `exec.Runner` interface

```go
// internal/exec/runner.go — umožní testování bez reálných systémových příkazů
type Runner interface {
    Run(name string, args ...string) (string, error)
}
type SystemRunner struct{}   // produkce: volá os/exec
type MockRunner struct{}     // testy: předpřipravené odpovědi
```

Prototype `guard.go:29-32` má globální `runCmd()` → nahradit injektovaným Runnerem.

### MCP protokol: zachovat hand-rolled

Prototype implementuje přesně to co potřebujeme (initialize, tools/list, tools/call, ping). ~120 řádků, zero deps. Migrace na Go SDK jen při breaking change specifikace.

### Jazyk tool definic

Tool definitions (name, description, inputSchema) → **anglicky** pro LLM.
Tool responses → anglicky (systémová data). Claude Code odpovídá uživateli v jeho jazyce automaticky.

---

## Fáze 2: Safety hardening

### Opravy prototypu
- **Atomické state file zápisy** — `guard.go:74` ignoruje chyby → write-to-temp + `os.Rename`
- **Scanner buffer** — `main.go:118` default 64KB → zvětšit na 1MB
- **Rollback semantika** — `snapper undochange N..0` → `snapper undochange N..M` (pre..post pair)
- **GnomeConfigSet popis** — gsettings/dconf nežije na Btrfs subvolume, snapper rollback nepomůže → opravit popis

### Nové safety mechanismy

**Rate limiter** (`internal/ratelimit/limiter.go`):
- Max 10 destruktivních operací / 30 min
- Max 3 rollbacky v session → read-only režim

**Snapshot loop detekce**:
- In-memory ledger operací
- Stejný tool+args rollbacknutý v posledních 10 min → odmítnutí
- 5+ destruktivních operací / 30 min → vyžadovat `"force": true`

**`--dry-run` flag**:
- Všechny destruktivní operace simulovány
- LLM informován přes `instructions` v initialize response

---

## Fáze 3: `suse_system_info` — nový tool

Claude potřebuje znát systém **předtím** než cokoliv navrhne. Tento tool je základ celé hodnoty LeapDoctor.

```
Name: "suse_system_info"
Description: "Gather comprehensive system information for SUSE Leap: OS version, kernel,
              GNOME version, session type (Wayland/X11), CPU, RAM, disk/Btrfs usage,
              GPU with loaded driver, and key installed packages."
Annotations: ReadOnlyHint: true, IdempotentHint: true
```

**Zdroje:**
| Data | Příkaz |
|------|--------|
| OS verze | `/etc/os-release` |
| Kernel | `uname -r` |
| GNOME | `gnome-shell --version` |
| Session | `loginctl show-session --property=Type` |
| CPU | `lscpu` |
| RAM/swap | `/proc/meminfo` |
| Disk | `df -h /` + `btrfs filesystem usage /` |
| GPU + driver | `lspci \| grep VGA` + `lsmod \| grep nvidia\|amdgpu\|i915` |
| Klíčové balíčky | `rpm -qa gnome-shell mutter gdm pipewire` |

### Stávajících 12 nástrojů: zachovat, popisy do EN

---

## Fáze 4: Distribuce + integrace s Claude Code

### Build & install
```bash
make build      # → ./leapdoctor binary
make install    # → /usr/local/bin/leapdoctor + /etc/leapdoctor/config.yml + /var/lib/leapdoctor/
make test       # → unit testy
```

### Integrace s Claude Code
```bash
leapdoctor --setup-claude    # Mergne do ~/.claude.json:
```
```json
{
  "mcpServers": {
    "leapdoctor": {
      "command": "/usr/local/bin/leapdoctor",
      "type": "stdio"
    }
  }
}
```

### Integrace s mcphost (alternativní klient)
```bash
leapdoctor --setup-mcphost   # Mergne do ~/.mcphost.yml
```

### System check
```bash
leapdoctor --check    # Ověří: Btrfs?, snapper?, GNOME?, Flatpak?
```

### Auto-detekce při MCP startu
- Bez Btrfs → snapshoty vypnuty, warning v `instructions`
- Bez snapper → varování
- Bez Flatpak → `suse_flatpak_install` skryt z tools/list

---

## Fáze 5: Konfigurace

### `/etc/leapdoctor/config.yml`
```yaml
dry_run: false
rate_limit:
  max_destructive_per_30min: 10
  max_rollbacks_before_lockout: 3
safety:
  block_gnome_critical: true
  critical_packages: [gnome-shell, gdm, mutter, glib2, gtk3, gtk4, wayland, pipewire]
recovery:
  enabled: true
  gdm_wait_timeout: 60s
  auto_reboot_on_rollback: false
```

---

## Fáze 6: Testování

### Unit testy (přes MockRunner)
- Každý tool: happy path, dry-run, selhání + rollback, health check failure
- Rate limiter: překročení limitů, lockout
- Snapshot loop detekce

### MCP protocol testy
- Initialize, tools/list, tools/call dispatch
- Neznámá metoda → -32601, malformed JSON → parse error

### Smoke test
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | ./leapdoctor
```

---

## Kritické soubory

| Zdroj (prototype/) | Cíl | Změny |
|---------------------|-----|-------|
| `main.go` (535 řádků) | `cmd/leapdoctor/main.go` + `internal/mcp/` | Rozdělit, flagy, scanner buffer 1MB |
| `guard.go` (614 řádků) | `internal/tools/*.go` + `internal/guard/` | Rozdělit, Runner injection, EN popisy |
| `suse-agent-guard-recovery.service` | `dist/leapdoctor-recovery.service` | Ponechat as-is (hardening v2) |
| `Makefile` | `Makefile` | build, test, install targety |
| `config-examples.yml` | `internal/config/` | Základ pro --setup-claude a --setup-mcphost |

---

## Verifikace

1. `make build` → single binary, zero external deps
2. `make test` → unit testy pass
3. MCP smoke: `echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' | ./leapdoctor`
4. `./leapdoctor --check` → systém capabilities výpis
5. `./leapdoctor --setup-claude` → konfig mergnut do `~/.claude.json`
6. Claude Code session: `suse_system_info` → vrátí detaily systému
7. Live na Leap 16 VM: zypper install + snapshot + rollback
