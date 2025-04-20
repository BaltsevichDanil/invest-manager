.PHONY: build clean run test install uninstall

APP_NAME = invest-manager
BUILD_DIR = build
CMD_DIR = cmd/bot

# Default Go environment
GO = go
GOFMT = gofmt
GOFLAGS = -v
LDFLAGS = -ldflags "-s -w"

# Build binary
build:
	mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./$(CMD_DIR)

# Run locally (once)
run:
	$(GO) run ./$(CMD_DIR)

# Run once with monthly reminder
run-monthly:
	$(GO) run ./$(CMD_DIR) -run-once -monthly

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GO) test -v ./internal/...

# Format code
fmt:
	$(GOFMT) -s -w ./

# Update dependencies
deps:
	$(GO) mod tidy
	$(GO) mod download

# Install to system
install: build
	sudo mkdir -p /opt/$(APP_NAME)
	sudo cp $(BUILD_DIR)/$(APP_NAME) /opt/$(APP_NAME)/
	sudo cp deploy/$(APP_NAME).service /etc/systemd/system/
	sudo systemctl daemon-reload
	@echo "To start the service, run: sudo systemctl start $(APP_NAME)"
	@echo "To enable on boot, run: sudo systemctl enable $(APP_NAME)"

# Uninstall from system
uninstall:
	sudo systemctl stop $(APP_NAME) || true
	sudo systemctl disable $(APP_NAME) || true
	sudo rm -f /etc/systemd/system/$(APP_NAME).service
	sudo rm -rf /opt/$(APP_NAME)
	sudo systemctl daemon-reload

# Show service logs
logs:
	journalctl -u $(APP_NAME) -f 