# Plan: Update MCP integration docs + fix --setup-claude bug

## Kontext

CLAUDE.md měl zastaralou sekci "MCP Integration" s referencí na starý název `suse-guard` a bez zmínky o `--setup-claude`. Tvrdil, že `cmd/leapdoctor/main.go` neexistuje (existuje). README.md byl placeholder s 5 odrážkami. `--setup-claude` nefungoval kvůli false-positive detekci "already configured".

---

## Fáze 1: Update CLAUDE.md

### Oprava "Prototype vs Refactored Code"
- Odstraněna poznámka o neexistujícím `cmd/leapdoctor/main.go` — existuje

### Přepsání "MCP Integration" sekce
Tři metody integrace:
```bash
# Method 1: Automatic (recommended)
make install
leapdoctor --setup-claude

# Method 2: Claude Code CLI
claude mcp add leapdoctor -- /usr/local/bin/leapdoctor

# Method 3: Manual (~/.claude.json)
```
Přidán tip pro `--dry-run` a zmínka o `--setup-mcphost`.

---

## Fáze 2: Přepsání README.md

Placeholder nahrazen kompletní dokumentací:
- **What it is** — popis projektu
- **Quick Start** — build, install, setup, verify
- **Available Tools** — všech 13 `suse_*` nástrojů po kategoriích (Package Management, Snapshots, System Monitoring, GNOME Configuration, Service Management)
- **Safety Features** — snapshoty, health checks, rate limiting, loop detekce, rollback lockout, critical package blocking
- **Configuration** — `/etc/leapdoctor/config.json` formát
- **Recovery Service** — `make service-install`
- **Flags** — tabulka: `--check`, `--dry-run`, `--post-boot-check`, `--setup-claude`, `--setup-mcphost`
- **MCP Integration** — 3 metody + `--dry-run` tip
- **Requirements** — openSUSE Leap 16, Btrfs, Snapper, Go 1.24.5+

---

## Fáze 3: Fix --setup-claude bug

### Problém
`MergeClaudeConfig()` v `internal/config/config.go` používal:
```go
strings.Contains(string(existing), "leapdoctor")
```
Hledal řetězec v celém `~/.claude.json`. Našel `"lioilsources/leapdoctor"` v `githubRepoPaths` a mylně hlásil "already configured".

### Oprava
Místo naivního string search se teď parsuje JSON a kontroluje se klíč přímo v `mcpServers` mapě:
```go
servers, _ := obj["mcpServers"].(map[string]interface{})
if _, exists := servers["leapdoctor"]; exists {
    return fmt.Errorf("leapdoctor already configured in %s", path)
}
```

Stejný typ opravy aplikován i na `MergeMcphostConfig()` — místo `strings.Contains` se iteruje přes YAML řádky a hledá se `leapdoctor:` jako klíč.

---

## Změněné soubory

| Soubor | Změna |
|--------|-------|
| `CLAUDE.md` | Fix 2 sekcí (Prototype, MCP Integration) |
| `README.md` | Kompletní přepsání |
| `internal/config/config.go` | Fix false-positive detekce v MergeClaudeConfig + MergeMcphostConfig |

---

## Verifikace

1. `make build` → OK
2. `leapdoctor --setup-claude` → zapíše do `~/.claude.json` mcpServers.leapdoctor
3. Restart Claude Code → `suse_*` nástroje dostupné
