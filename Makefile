# Claude Cage Makefile
#
# Build and test targets for the cage CLI tool.
#
# Usage:
#   make build          - Build the cage binary
#   make test           - Run tests with coverage summary
#   make test-quick     - Run tests without coverage (faster)
#   make test-verbose   - Run tests with verbose output and coverage
#   make coverage       - Detailed coverage report by package
#   make coverage-html  - Generate HTML coverage report
#   make coverage-check - CI-friendly coverage check (fails if below threshold)
#   make e2e            - Run E2E tests with bridge networking (requires root)
#   make e2e-short      - Run E2E tests in short mode
#   make e2e-user       - Run E2E tests with user-mode networking (no root)
#   make e2e-ubuntu     - Run E2E tests with Ubuntu image
#   make install        - Install cage + bash/zsh/fish completions to ~/.local/...
#   make install-system - Install cage + completions system-wide (needs sudo)
#   make clean          - Remove build artifacts

.PHONY: build test test-quick test-verbose coverage coverage-html coverage-check e2e e2e-short e2e-user e2e-ubuntu install install-system clean

VERSION := 0.1.0
LDFLAGS := -ldflags "-X github.com/s-oravec/claude-cage/internal/cmd.Version=$(VERSION)"
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
COVERAGE_THRESHOLD := 40

# Build the cage binary with version info
build:
	go build $(LDFLAGS) -o cage ./cmd/cage

# Run tests with coverage summary (default target)
test:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic -race 2>&1 | tee /tmp/test-output.txt
	@echo ""
	@echo "=== Coverage Summary ==="
	@total=$$(go tool cover -func=$(COVERAGE_FILE) 2>/dev/null | grep total | awk '{print $$3}' || echo "0%"); \
	threshold_int=$$(echo "$(COVERAGE_THRESHOLD)" | cut -d. -f1); \
	total_int=$$(echo "$$total" | tr -d '%' | cut -d. -f1); \
	if [ "$$total_int" -lt "$$threshold_int" ]; then \
		echo "⚠️  Coverage: $$total (threshold: $(COVERAGE_THRESHOLD)%)"; \
	else \
		echo "✓ Coverage: $$total (threshold: $(COVERAGE_THRESHOLD)%)"; \
	fi

# Quick test without coverage (faster iteration)
test-quick:
	go test ./...

# Verbose test with coverage details
test-verbose:
	@echo "Running tests with coverage (verbose)..."
	@go test ./... -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic -race

# Detailed coverage report by package
coverage:
	@echo "Running tests with coverage..."
	@go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic 2>&1 | tee /tmp/coverage-run.txt
	@echo ""
	@echo "=== Coverage by Package ==="
	@printf "%-45s %s\n" "PACKAGE" "COVERAGE"
	@printf "%-45s %s\n" "-------" "--------"
	@grep -E "coverage:" /tmp/coverage-run.txt | sed 's/ok[[:space:]]*//' | awk '{printf "%-45s %s\n", $$1, $$4}'
	@echo ""
	@echo "=== Total Coverage ==="
	@go tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}'
	@echo ""
	@total=$$(go tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | tr -d '%'); \
	threshold_int=$$(echo "$(COVERAGE_THRESHOLD)" | cut -d. -f1); \
	total_int=$$(echo "$$total" | cut -d. -f1); \
	if [ "$$total_int" -lt "$$threshold_int" ]; then \
		echo "⚠️  Coverage ($$total%) is below threshold ($(COVERAGE_THRESHOLD)%)"; \
	else \
		echo "✓ Coverage ($$total%) meets threshold ($(COVERAGE_THRESHOLD)%)"; \
	fi

# Generate HTML coverage report (opens in browser)
coverage-html: coverage
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

# CI-friendly coverage check - exits with error if below threshold
coverage-check:
	@go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic > /dev/null 2>&1
	@total=$$(go tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | tr -d '%'); \
	threshold_int=$$(echo "$(COVERAGE_THRESHOLD)" | cut -d. -f1); \
	total_int=$$(echo "$$total" | cut -d. -f1); \
	if [ "$$total_int" -lt "$$threshold_int" ]; then \
		echo "❌ Coverage check failed: $$total% < $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "✓ Coverage check passed: $$total% >= $(COVERAGE_THRESHOLD)%"; \
	fi

# E2E tests with bridge networking (requires root for network setup)
e2e: build
	@echo "Running E2E tests with bridge networking (requires root)..."
	CAGE_BIN=$(PWD)/cage CAGE_NETWORK=bridge go test -v -timeout 10m ./test/e2e/...

# E2E tests in short mode (skip long-running tests)
e2e-short: build
	@echo "Running E2E tests (short mode)..."
	CAGE_BIN=$(PWD)/cage go test -v -short ./test/e2e/...

# E2E tests with user-mode networking (no root required)
e2e-user: build
	@echo "Running E2E tests with user-mode networking (no root)..."
	CAGE_BIN=$(PWD)/cage go test -v -timeout 10m ./test/e2e/...

# E2E tests with Ubuntu image (longer timeout due to larger image)
e2e-ubuntu: build
	@echo "Running E2E tests with Ubuntu..."
	CAGE_BIN=$(PWD)/cage CAGE_TEST_IMAGE=ubuntu-24.04 go test -v -timeout 15m ./test/e2e/...

# Install cage binary to user's local bin directory + shell completions in
# XDG-standard per-user locations. The completion files are harmless when the
# corresponding shell isn't used.
install: build
	install -m 755 cage ~/.local/bin/cage
	@install -d ~/.local/share/bash-completion/completions
	./cage completion bash > ~/.local/share/bash-completion/completions/cage
	@install -d ~/.local/share/zsh/site-functions
	./cage completion zsh  > ~/.local/share/zsh/site-functions/_cage
	@install -d ~/.config/fish/completions
	./cage completion fish > ~/.config/fish/completions/cage.fish
	@echo "Installed: ~/.local/bin/cage + bash/zsh/fish completions."
	@echo "Restart your shell (or 'source' the relevant rc file) to pick up completions."

# Install cage binary system-wide so it lives on sudo's default secure_path
# (required for `sudo cage` to work in root mode). Uses sudo internally so
# you can run plain `make install-system`; if already root, sudo is a no-op.
# Also installs shell completions in system-wide standard paths.
install-system: build
	sudo install -m 755 cage /usr/local/bin/cage
	sudo install -d /usr/share/bash-completion/completions
	./cage completion bash | sudo tee /usr/share/bash-completion/completions/cage > /dev/null
	sudo install -d /usr/local/share/zsh/site-functions
	./cage completion zsh  | sudo tee /usr/local/share/zsh/site-functions/_cage > /dev/null
	sudo install -d /usr/share/fish/vendor_completions.d
	./cage completion fish | sudo tee /usr/share/fish/vendor_completions.d/cage.fish > /dev/null
	@echo "Installed: /usr/local/bin/cage + bash/zsh/fish completions."
	@echo "Open a new shell to pick up completions."

# Remove build artifacts
clean:
	rm -f cage $(COVERAGE_FILE) $(COVERAGE_HTML)

# Lint with golangci-lint
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Install golangci-lint: https://golangci-lint.run/usage/install/" && exit 1)
	golangci-lint run ./...

# Fix common lint issues
lint-fix:
	golangci-lint run --fix ./...
