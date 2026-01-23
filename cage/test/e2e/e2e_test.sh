#!/bin/bash
# E2E test for cage CLI
# Requires: KVM, libvirt, qemu, virtiofsd
# Usage: ./e2e_test.sh [--keep] [--image IMAGE]

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CAGE_NAME="e2e-test-$$"
IMAGE="${IMAGE:-alpine-3.20}"
KEEP_ON_FAIL=false
TIMEOUT=120
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CAGE_BIN="${SCRIPT_DIR}/../../cage"

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --keep) KEEP_ON_FAIL=true; shift ;;
        --image) IMAGE="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_test() { echo -e "${GREEN}[TEST]${NC} $1"; }

pass() {
    ((TESTS_PASSED++))
    echo -e "  ${GREEN}✓${NC} $1"
}

fail() {
    ((TESTS_FAILED++))
    echo -e "  ${RED}✗${NC} $1"
}

cleanup() {
    log_info "Cleaning up..."
    $CAGE_BIN stop "$CAGE_NAME" --force 2>/dev/null || true
    # Give libvirt time to clean up
    sleep 2
}

# Cleanup on exit
trap cleanup EXIT

# Build cage if needed
if [[ ! -f "$CAGE_BIN" ]]; then
    log_info "Building cage..."
    (cd "$SCRIPT_DIR/../.." && make build)
fi

# Check prerequisites
log_info "Checking prerequisites..."
if ! command -v virsh &>/dev/null; then
    log_error "virsh not found. Install libvirt."
    exit 1
fi

if ! virsh capabilities &>/dev/null; then
    log_error "Cannot connect to libvirt. Is libvirtd running?"
    exit 1
fi

# Check if KVM is available
if [[ ! -e /dev/kvm ]]; then
    log_warn "/dev/kvm not found. Tests may be slow without hardware virtualization."
fi

echo ""
echo "=========================================="
echo "  Cage E2E Test Suite"
echo "=========================================="
echo "  Image:     $IMAGE"
echo "  Cage name: $CAGE_NAME"
echo "  Timeout:   ${TIMEOUT}s"
echo "=========================================="
echo ""

# ============================================
# TEST: cage version
# ============================================
log_test "Testing: cage version"
((TESTS_RUN++))
if $CAGE_BIN version | grep -q "cage version"; then
    pass "cage version works"
else
    fail "cage version failed"
fi

# ============================================
# TEST: cage doctor
# ============================================
log_test "Testing: cage doctor"
((TESTS_RUN++))
if $CAGE_BIN doctor 2>&1 | grep -qE "(✓|Checking)"; then
    pass "cage doctor runs"
else
    fail "cage doctor failed"
fi

# ============================================
# TEST: cage config init
# ============================================
log_test "Testing: cage config init"
((TESTS_RUN++))
# Use temp config dir
export CAGE_CONFIG_DIR=$(mktemp -d)
trap "rm -rf $CAGE_CONFIG_DIR; cleanup" EXIT

if $CAGE_BIN config init --force 2>&1; then
    pass "cage config init works"
else
    fail "cage config init failed"
fi

# ============================================
# TEST: cage setup (download image)
# ============================================
log_test "Testing: cage setup --base $IMAGE"
((TESTS_RUN++))

# Check if image already exists
if $CAGE_BIN setup --list 2>&1 | grep -q "$IMAGE.*downloaded"; then
    log_info "Image $IMAGE already downloaded, skipping download"
    pass "cage setup (image exists)"
else
    log_info "Downloading $IMAGE (this may take a while)..."
    if timeout 600 $CAGE_BIN setup --base "$IMAGE" 2>&1; then
        pass "cage setup downloaded image"
    else
        fail "cage setup failed"
        exit 1
    fi
fi

# ============================================
# TEST: cage start
# ============================================
log_test "Testing: cage start --name $CAGE_NAME --image $IMAGE"
((TESTS_RUN++))

if timeout $TIMEOUT $CAGE_BIN start --name "$CAGE_NAME" --image "$IMAGE" --profile light 2>&1; then
    pass "cage start succeeded"
else
    fail "cage start failed"
    exit 1
fi

# Wait for VM to be ready
log_info "Waiting for VM to boot..."
sleep 10

# ============================================
# TEST: cage list
# ============================================
log_test "Testing: cage list"
((TESTS_RUN++))
if $CAGE_BIN list 2>&1 | grep -q "$CAGE_NAME.*running"; then
    pass "cage list shows running cage"
else
    fail "cage list doesn't show cage"
fi

# ============================================
# TEST: cage status
# ============================================
log_test "Testing: cage status $CAGE_NAME"
((TESTS_RUN++))
if $CAGE_BIN status "$CAGE_NAME" 2>&1 | grep -q "Status:.*running"; then
    pass "cage status shows running"
else
    fail "cage status failed"
fi

# ============================================
# TEST: cage status --json
# ============================================
log_test "Testing: cage status $CAGE_NAME --json"
((TESTS_RUN++))
if $CAGE_BIN status "$CAGE_NAME" --json 2>&1 | grep -q '"status"'; then
    pass "cage status --json outputs JSON"
else
    fail "cage status --json failed"
fi

# ============================================
# TEST: cage ssh (basic connectivity)
# ============================================
log_test "Testing: cage ssh $CAGE_NAME (connectivity)"
((TESTS_RUN++))

# Wait for SSH to be available (with retries)
SSH_READY=false
for i in {1..30}; do
    if $CAGE_BIN ssh "$CAGE_NAME" "echo 'SSH_OK'" 2>&1 | grep -q "SSH_OK"; then
        SSH_READY=true
        break
    fi
    log_info "Waiting for SSH... ($i/30)"
    sleep 5
done

if $SSH_READY; then
    pass "SSH connection works"
else
    fail "SSH connection failed"
    # Show VM console for debugging
    log_error "SSH not available after 150s. VM may have failed to boot."
    if $KEEP_ON_FAIL; then
        log_warn "Keeping cage for debugging. Clean up manually with: cage stop $CAGE_NAME"
        trap - EXIT
    fi
    exit 1
fi

# ============================================
# TEST: cage exec
# ============================================
log_test "Testing: cage exec $CAGE_NAME -- uname -a"
((TESTS_RUN++))
if $CAGE_BIN exec "$CAGE_NAME" -- uname -a 2>&1 | grep -qi "linux"; then
    pass "cage exec works"
else
    fail "cage exec failed"
fi

# ============================================
# TEST: cage ssh (run command)
# ============================================
log_test "Testing: cage ssh $CAGE_NAME 'hostname'"
((TESTS_RUN++))
HOSTNAME_OUTPUT=$($CAGE_BIN ssh "$CAGE_NAME" "hostname" 2>&1 || true)
if [[ -n "$HOSTNAME_OUTPUT" ]]; then
    pass "cage ssh command execution works (hostname: $HOSTNAME_OUTPUT)"
else
    fail "cage ssh command failed"
fi

# ============================================
# TEST: cage port list
# ============================================
log_test "Testing: cage port list $CAGE_NAME"
((TESTS_RUN++))
if $CAGE_BIN port list "$CAGE_NAME" 2>&1 | grep -qE "(No port|PORT)"; then
    pass "cage port list works"
else
    fail "cage port list failed"
fi

# ============================================
# TEST: cage stop
# ============================================
log_test "Testing: cage stop $CAGE_NAME"
((TESTS_RUN++))
if $CAGE_BIN stop "$CAGE_NAME" 2>&1; then
    pass "cage stop succeeded"
else
    fail "cage stop failed"
fi

# Wait for VM to stop
sleep 3

# ============================================
# TEST: cage list (after stop)
# ============================================
log_test "Testing: cage list (after stop)"
((TESTS_RUN++))
if $CAGE_BIN list 2>&1 | grep -q "$CAGE_NAME.*stopped"; then
    pass "cage shows stopped status"
else
    # May be completely removed
    if ! $CAGE_BIN list 2>&1 | grep -q "$CAGE_NAME"; then
        pass "cage removed after stop"
    else
        fail "cage status unclear after stop"
    fi
fi

# ============================================
# SUMMARY
# ============================================
echo ""
echo "=========================================="
echo "  Test Results"
echo "=========================================="
echo "  Total:  $TESTS_RUN"
echo -e "  Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "  Failed: ${RED}$TESTS_FAILED${NC}"
echo "=========================================="

if [[ $TESTS_FAILED -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
