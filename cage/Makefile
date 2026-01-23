.PHONY: build test test-short coverage coverage-html e2e install clean

VERSION := 0.1.0
LDFLAGS := -ldflags "-X github.com/stiivo/cage/internal/cmd.Version=$(VERSION)"
COVERAGE_FILE := coverage.out
COVERAGE_HTML := coverage.html
COVERAGE_THRESHOLD := 40

build:
	go build $(LDFLAGS) -o cage ./cmd/cage

test:
	go test ./... -v

test-short:
	go test ./...

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
	if [ $$(echo "$$total < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "⚠️  Coverage ($$total%) is below threshold ($(COVERAGE_THRESHOLD)%)"; \
	else \
		echo "✓ Coverage ($$total%) meets threshold ($(COVERAGE_THRESHOLD)%)"; \
	fi

coverage-html: coverage
	@go tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"

coverage-check:
	@go test ./... -coverprofile=$(COVERAGE_FILE) -covermode=atomic > /dev/null 2>&1
	@total=$$(go tool cover -func=$(COVERAGE_FILE) | grep total | awk '{print $$3}' | tr -d '%'); \
	if [ $$(echo "$$total < $(COVERAGE_THRESHOLD)" | bc -l) -eq 1 ]; then \
		echo "❌ Coverage check failed: $$total% < $(COVERAGE_THRESHOLD)%"; \
		exit 1; \
	else \
		echo "✓ Coverage check passed: $$total% >= $(COVERAGE_THRESHOLD)%"; \
	fi

e2e: build
	@echo "Running E2E tests (requires KVM, libvirt)..."
	./test/e2e/e2e_test.sh

e2e-ubuntu: build
	@echo "Running E2E tests with Ubuntu..."
	./test/e2e/e2e_test.sh --image ubuntu-24.04

install: build
	install -m 755 cage ~/.local/bin/cage

clean:
	rm -f cage $(COVERAGE_FILE) $(COVERAGE_HTML)
