BINARY     := leapdoctor
VERSION    := 1.0.0
INSTALL_DIR := /usr/local/bin
CONFIG_DIR  := /etc/leapdoctor
STATE_DIR   := /var/lib/leapdoctor
SERVICE_DIR := /etc/systemd/system

LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test install uninstall service-install service-uninstall check clean

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/leapdoctor/

test:
	go test ./...

install: build
	@echo "Installing $(BINARY) to $(INSTALL_DIR)..."
	sudo install -Dm755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	sudo mkdir -p $(STATE_DIR)
	sudo mkdir -p $(CONFIG_DIR)
	@if [ ! -f $(CONFIG_DIR)/config.json ]; then \
		sudo install -Dm644 dist/leapdoctor.conf.default $(CONFIG_DIR)/config.json; \
	fi
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"

service-install: install
	@echo "Installing systemd recovery service..."
	sudo install -Dm644 dist/leapdoctor-recovery.service $(SERVICE_DIR)/leapdoctor-recovery.service
	sudo systemctl daemon-reload
	sudo systemctl enable leapdoctor-recovery.service
	@echo "Recovery service enabled"

service-uninstall:
	sudo systemctl disable leapdoctor-recovery.service || true
	sudo rm -f $(SERVICE_DIR)/leapdoctor-recovery.service
	sudo systemctl daemon-reload

uninstall: service-uninstall
	sudo rm -f $(INSTALL_DIR)/$(BINARY)
	sudo rm -rf $(STATE_DIR)
	@echo "Uninstalled"

check: build
	./$(BINARY) --check

clean:
	rm -f $(BINARY)

# Test MCP protocol (send initialize + tools/list)
test-mcp: build
	@echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | \
		./$(BINARY) | python3 -m json.tool
	@echo ""
	@echo "Tools:"
	@echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | \
		./$(BINARY) | python3 -c "import json,sys; data=json.load(sys.stdin); [print(f'  - {t[\"name\"]}') for t in data.get('result',{}).get('tools',[])]"
