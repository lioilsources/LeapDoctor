// suse-agent-guard – MCP server pro bezpečné systémové operace na openSUSE Leap 16
//
// Implementuje MCP stdio transport (JSON-RPC 2.0 přes stdin/stdout).
// Všechny destruktivní operace jsou obaleny Snapper pre/post snapshoty.
// Při selhání nebo nezdravém GNOME provede automatický rollback.
//
// Použití s ClaudeCode:
//   claude mcp add suse-guard -- /usr/local/bin/suse-agent-guard
//
// Použití s mcphost (~/.mcphost.yml):
//   mcpServers:
//     suse-guard:
//       command: /usr/local/bin/suse-agent-guard

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// ── JSON-RPC 2.0 typy ─────────────────────────────────────────────────────

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ── MCP Protocol typy ─────────────────────────────────────────────────────

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    Capabilities   `json:"capabilities"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
	Instructions    string         `json:"instructions"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
	Annotations Annotations `json:"annotations,omitempty"`
}

type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"`
}

type Annotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint,omitempty"`
	DestructiveHint bool `json:"destructiveHint,omitempty"`
	IdempotentHint  bool `json:"idempotentHint,omitempty"`
}

type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ── Server ─────────────────────────────────────────────────────────────────

type Server struct {
	guard   *SnapperGuard
	scanner *bufio.Scanner
	writer  *bufio.Writer
}

func NewServer() *Server {
	return &Server{
		guard:   NewSnapperGuard(),
		scanner: bufio.NewScanner(os.Stdin),
		writer:  bufio.NewWriter(os.Stdout),
	}
}

func (s *Server) send(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		return
	}
	fmt.Fprintf(s.writer, "%s\n", data)
	s.writer.Flush()
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	s.send(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id interface{}, code int, msg string) {
	s.send(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: msg},
	})
}

func (s *Server) toolResult(text string, isError bool) ToolResult {
	return ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
		IsError: isError,
	}
}

// ── Tool definice ──────────────────────────────────────────────────────────

func (s *Server) tools() []Tool {
	return []Tool{
		{
			Name: "suse_zypper_install",
			Description: `Bezpečně nainstaluje balíček přes zypper s automatickým Snapper pre/post snapshoty.
Před instalací vytvoří pre-snapshot, po instalaci ověří zdraví systému (GNOME/systemd).
Při selhání automaticky provede rollback. Pouze pro openSUSE Leap 16 s Btrfs+Snapper.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"package": {
						Type:        "string",
						Description: "Název balíčku k instalaci (např. 'git', 'vim', 'htop')",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "Pokud true, pouze simuluje – neprovede žádné změny",
					},
				},
				Required: []string{"package"},
			},
			Annotations: Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_zypper_remove",
			Description: `Bezpečně odstraní balíček přes zypper se Snapper snapshoty a možností rollbacku.
Varuje pokud odstraňovaný balíček může ovlivnit GNOME nebo kritické systémové komponenty.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"package": {
						Type:        "string",
						Description: "Název balíčku k odstranění",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "Pokud true, pouze simuluje",
					},
				},
				Required: []string{"package"},
			},
			Annotations: Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_snapshot_create",
			Description: `Vytvoří Snapper snapshot aktuálního stavu systému.
Užitečné před manuálními úpravami nebo jako záchranný bod.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"description": {
						Type:        "string",
						Description: "Popis snapshotu (co se chystáš změnit)",
					},
				},
				Required: []string{"description"},
			},
			Annotations: Annotations{ReadOnlyHint: false, DestructiveHint: false},
		},
		{
			Name: "suse_snapshot_list",
			Description: `Zobrazí seznam existujících Snapper snapshotů s jejich popisem a časovým razítkem.
Read-only operace.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
			Annotations: Annotations{ReadOnlyHint: true},
		},
		{
			Name: "suse_snapshot_rollback",
			Description: `Provede rollback na konkrétní snapshot číslo.
POZOR: Tato operace je destruktivní – vrátí systémové soubory do předchozího stavu.
Po rollbacku je nutný restart systému. Zobraz uživateli seznam snapshotů (suse_snapshot_list) před voláním.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"snapshot_number": {
						Type:        "string",
						Description: "Číslo snapshotu (zobrazíš ho přes suse_snapshot_list)",
					},
				},
				Required: []string{"snapshot_number"},
			},
			Annotations: Annotations{DestructiveHint: true},
		},
		{
			Name: "suse_system_health",
			Description: `Zkontroluje zdraví systému: selhané systemd služby, GNOME shell status,
SELinux denial logy, stav Flatpak a posledních 20 systémových chyb z journalctl.
Read-only operace, bezpečná k opakovanému volání.`,
			InputSchema: InputSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
			Annotations: Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_journalctl",
			Description: `Čte systemd journal logy. Umí filtrovat dle priority, služby nebo časového rozsahu.
Read-only operace.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"service": {
						Type:        "string",
						Description: "Název systemd služby (např. 'gdm', 'NetworkManager', 'flatpak'). Prázdné = všechny.",
					},
					"priority": {
						Type:        "string",
						Description: "Minimální priorita logů",
						Enum:        []string{"emerg", "alert", "crit", "err", "warning", "notice", "info", "debug"},
					},
					"lines": {
						Type:        "string",
						Description: "Počet posledních řádků (výchozí: 50)",
					},
					"since": {
						Type:        "string",
						Description: "Od kdy (např. '1 hour ago', '2025-01-01', 'today')",
					},
				},
			},
			Annotations: Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_gnome_config_get",
			Description: `Přečte GNOME/dconf nastavení přes gsettings. Read-only.
Příklady schémat: org.gnome.desktop.interface, org.gnome.shell`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"schema": {
						Type:        "string",
						Description: "dconf schéma (např. 'org.gnome.desktop.interface')",
					},
					"key": {
						Type:        "string",
						Description: "Klíč v schématu (prázdné = všechny klíče v schématu)",
					},
				},
				Required: []string{"schema"},
			},
			Annotations: Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_gnome_config_set",
			Description: `Nastaví GNOME/dconf hodnotu přes gsettings se Snapper snapshoty.
Před změnou vytvoří snapshot pro případ rollbacku.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"schema": {
						Type:        "string",
						Description: "dconf schéma (např. 'org.gnome.desktop.interface')",
					},
					"key": {
						Type:        "string",
						Description: "Klíč v schématu",
					},
					"value": {
						Type:        "string",
						Description: "Nová hodnota (ve formátu gsettings, např. 'true', \"'dark'\", \"['ext1']\")",
					},
				},
				Required: []string{"schema", "key", "value"},
			},
			Annotations: Annotations{DestructiveHint: false},
		},
		{
			Name: "suse_flatpak_install",
			Description: `Nainstaluje Flatpak aplikaci z Flathub nebo jiného nakonfigurovaného remote.
Toto je preferovaná metoda pro instalaci desktopových aplikací na GNOME/Wayland.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"app_id": {
						Type:        "string",
						Description: "Flatpak Application ID (např. 'org.gimp.GIMP', 'com.spotify.Client')",
					},
					"remote": {
						Type:        "string",
						Description: "Remote název (výchozí: 'flathub')",
					},
					"dry_run": {
						Type:        "boolean",
						Description: "Pouze simulace bez instalace",
					},
				},
				Required: []string{"app_id"},
			},
			Annotations: Annotations{DestructiveHint: false},
		},
		{
			Name: "suse_selinux_status",
			Description: `Zobrazí SELinux status, posledních N denial záznamů a doporučení pro řešení.
openSUSE Leap 16 používá SELinux jako výchozí LSM (ne AppArmor).`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"show_denials": {
						Type:        "boolean",
						Description: "Zobrazit SELinux denial záznamy z auditu (výchozí: true)",
					},
					"lines": {
						Type:        "string",
						Description: "Počet denial záznamů (výchozí: 20)",
					},
				},
			},
			Annotations: Annotations{ReadOnlyHint: true, IdempotentHint: true},
		},
		{
			Name: "suse_systemctl",
			Description: `Správa systemd služeb: start, stop, restart, enable, disable, status.
Destruktivní akce (start/stop/restart) jsou logovány před provedením.`,
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"action": {
						Type:        "string",
						Description: "Akce pro systemctl",
						Enum:        []string{"status", "start", "stop", "restart", "enable", "disable", "is-active", "is-enabled"},
					},
					"service": {
						Type:        "string",
						Description: "Název systemd jednotky (např. 'gdm.service', 'NetworkManager')",
					},
				},
				Required: []string{"action", "service"},
			},
			Annotations: Annotations{DestructiveHint: true},
		},
	}
}

// ── Dispatcher ─────────────────────────────────────────────────────────────

func (s *Server) handleCallTool(id interface{}, params json.RawMessage) {
	var p CallToolParams
	if err := json.Unmarshal(params, &p); err != nil {
		s.sendError(id, -32602, "invalid params: "+err.Error())
		return
	}

	arg := func(key string) string {
		if v, ok := p.Arguments[key]; ok {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	argBool := func(key string) bool {
		return arg(key) == "true"
	}

	var result ToolResult

	switch p.Name {
	case "suse_zypper_install":
		result = s.guard.ZypperInstall(arg("package"), argBool("dry_run"))
	case "suse_zypper_remove":
		result = s.guard.ZypperRemove(arg("package"), argBool("dry_run"))
	case "suse_snapshot_create":
		result = s.guard.SnapshotCreate(arg("description"))
	case "suse_snapshot_list":
		result = s.guard.SnapshotList()
	case "suse_snapshot_rollback":
		result = s.guard.SnapshotRollback(arg("snapshot_number"))
	case "suse_system_health":
		result = s.guard.SystemHealth()
	case "suse_journalctl":
		result = s.guard.Journalctl(arg("service"), arg("priority"), arg("lines"), arg("since"))
	case "suse_gnome_config_get":
		result = s.guard.GnomeConfigGet(arg("schema"), arg("key"))
	case "suse_gnome_config_set":
		result = s.guard.GnomeConfigSet(arg("schema"), arg("key"), arg("value"))
	case "suse_flatpak_install":
		result = s.guard.FlatpakInstall(arg("app_id"), arg("remote"), argBool("dry_run"))
	case "suse_selinux_status":
		result = s.guard.SelinuxStatus(argBool("show_denials"), arg("lines"))
	case "suse_systemctl":
		result = s.guard.Systemctl(arg("action"), arg("service"))
	default:
		result = s.toolResult(fmt.Sprintf("Neznámý tool: %s", p.Name), true)
	}

	s.sendResult(id, result)
}

// ── Hlavní smyčka ──────────────────────────────────────────────────────────

func (s *Server) Run() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[suse-agent-guard] ")

	// Post-boot recovery check
	if len(os.Args) > 1 && os.Args[1] == "--post-boot-check" {
		s.guard.PostBootRecovery()
		return
	}

	log.Println("MCP server spuštěn (stdio transport)")

	for s.scanner.Scan() {
		line := s.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			log.Printf("parse error: %v", err)
			continue
		}

		// Notifikace nemají ID a nevyžadují odpověď
		if req.ID == nil && req.Method != "" {
			if req.Method == "notifications/initialized" {
				log.Println("Klient inicializován")
			}
			continue
		}

		switch req.Method {
		case "initialize":
			s.sendResult(req.ID, InitializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: Capabilities{
					Tools: &ToolsCapability{ListChanged: false},
				},
				ServerInfo: ServerInfo{
					Name:    "suse-agent-guard",
					Version: "1.0.0",
				},
				Instructions: `MCP server pro bezpečné systémové operace na openSUSE Leap 16 s GNOME/Wayland.
DŮLEŽITÉ PRAVIDLA:
- Vždy používej suse_system_health PŘED a PO destruktivních operacích
- Preference: Flatpak pro desktop aplikace, zypper pro systémové balíčky
- NIKDY nenavrhuj: snap, AppImage, yast2, AppArmor příkazy, X11/Xorg řešení
- Při problémech s GNOME: nejdříve zkontroluj journalctl a SELinux denial záznamy
- Snapper snapshoty jsou automatické u suse_zypper_* nástrojů`,
			})

		case "notifications/initialized":
			// Ignore

		case "tools/list":
			s.sendResult(req.ID, map[string]interface{}{
				"tools": s.tools(),
			})

		case "tools/call":
			s.handleCallTool(req.ID, req.Params)

		case "ping":
			s.sendResult(req.ID, map[string]interface{}{})

		default:
			s.sendError(req.ID, -32601, "Method not found: "+req.Method)
		}
	}

	if err := s.scanner.Err(); err != nil {
		log.Printf("stdin error: %v", err)
	}
}

func main() {
	NewServer().Run()
}
