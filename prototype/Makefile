BINARY     := suse-agent-guard
INSTALL_DIR := /usr/local/bin
SERVICE_DIR := /etc/systemd/system
STATE_DIR   := /var/lib/suse-agent-guard

.PHONY: build install uninstall service-install service-uninstall

build:
	go build -o $(BINARY) .

install: build
	@echo "→ Instaluji $(BINARY) do $(INSTALL_DIR)..."
	sudo install -Dm755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	sudo mkdir -p $(STATE_DIR)
	@echo "✅ Binárka nainstalována: $(INSTALL_DIR)/$(BINARY)"

service-install: install
	@echo "→ Instaluji systemd recovery service..."
	sudo install -Dm644 suse-agent-guard-recovery.service $(SERVICE_DIR)/
	sudo systemctl daemon-reload
	sudo systemctl enable suse-agent-guard-recovery.service
	@echo "✅ Recovery service aktivována"

service-uninstall:
	sudo systemctl disable suse-agent-guard-recovery.service || true
	sudo rm -f $(SERVICE_DIR)/suse-agent-guard-recovery.service
	sudo systemctl daemon-reload

uninstall: service-uninstall
	sudo rm -f $(INSTALL_DIR)/$(BINARY)
	sudo rm -rf $(STATE_DIR)
	@echo "✅ Odinstalováno"

# Test MCP protokolu (pošle initialize request)
test-mcp: build
	@echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | \
		./$(BINARY) | python3 -m json.tool
	@echo ""
	@echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | \
		./$(BINARY) | python3 -c "import json,sys; data=json.load(sys.stdin); [print(f'  • {t[\"name\"]}') for t in data.get('result',{}).get('tools',[])]"
