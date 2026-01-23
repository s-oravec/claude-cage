.PHONY: build test test-quick test-verbose coverage coverage-html coverage-check e2e e2e-short e2e-user e2e-ubuntu install clean

VERSION := 0.1.0
LDFLAGS := -ldflags "-X github.com/s-oravec/claude-cage/internal/cmd.Version=$(VERSION)"
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
COVERAGE_THRESHOLD := 40

build:
	go build $(LDFLAGS) -o cage ./cmd/cage

# Default test target includes coverage
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

# Quick test without coverage (faster)
test-quick:
	go test ./...

# Verbose test with coverage
test-verbose:
	@echo "Running tests with coverage (verbose)..."
	@go test ./... -v -coverprofile=$(COVERAGE_FILE) -covermode=atomic -race

# Detailed coverage report
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

coverage-html: coverage
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

# CI-friendly coverage check (fails if below threshold)
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

e2e: build
	@echo "Running E2E tests (requires KVM, libvirt)..."
	CAGE_BIN=$(PWD)/cage go test -v -timeout 10m ./test/e2e/...

e2e-short: build
	@echo "Running E2E tests (short mode)..."
	CAGE_BIN=$(PWD)/cage go test -v -short ./test/e2e/...

e2e-user: build
	@echo "Running E2E tests with user-mode networking..."
	CAGE_BIN=$(PWD)/cage CAGE_USER_NETWORK=1 go test -v -timeout 10m ./test/e2e/...

e2e-ubuntu: build
	@echo "Running E2E tests with Ubuntu..."
	CAGE_BIN=$(PWD)/cage CAGE_TEST_IMAGE=ubuntu-24.04 go test -v -timeout 15m ./test/e2e/...

install: build
	install -m 755 cage ~/.local/bin/cage

clean:
	rm -f cage $(COVERAGE_FILE) $(COVERAGE_HTML)
