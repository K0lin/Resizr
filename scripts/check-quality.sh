#!/bin/bash
#
# Quality Check Script for Resizr
# Checks: Tests, Coverage, Formatting, and Linting
#
# Usage: ./scripts/check-quality.sh [--verbose]
#

set -e

# Change to project root directory (parent of scripts directory)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Colors for output (works on Unix/Linux/macOS)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Detect if colors are supported
if [[ ! -t 1 ]] || [[ "$TERM" == "dumb" ]]; then
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    NC=''
fi

VERBOSE=false
if [[ "$1" == "--verbose" ]] || [[ "$1" == "-v" ]]; then
    VERBOSE=true
fi

# Counters
PASSED=0
FAILED=0
COVERAGE_THRESHOLD=45.0

echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║           Resizr Quality Check Script                         ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Function to print check header
print_check() {
    echo -e "${YELLOW}▶ Checking: $1${NC}"
}

# Function to print success
print_success() {
    echo -e "${GREEN}✓ $1${NC}"
    PASSED=$((PASSED + 1))
}

# Function to print error
print_error() {
    echo -e "${RED}✗ $1${NC}"
    FAILED=$((FAILED + 1))
}

# Function to print info
print_info() {
    echo -e "${BLUE}  $1${NC}"
}

# ============================================================================
# 1. CHECK GO FMT
# ============================================================================
print_check "Go Formatting (gofmt)"
echo ""

UNFORMATTED=$(gofmt -s -l . 2>/dev/null)
if [ -z "$UNFORMATTED" ]; then
    print_success "All files are properly formatted"
else
    print_error "Some files are not properly formatted:"
    echo "$UNFORMATTED" | while read -r file; do
        echo -e "${RED}    - $file${NC}"
    done
    echo ""
    print_info "Run 'gofmt -s -w .' to fix formatting"
fi
echo ""

# ============================================================================
# 2. CHECK LINTER
# ============================================================================
print_check "Go Linter (golangci-lint)"
echo ""

# Check if golangci-lint is installed
if ! command -v golangci-lint &> /dev/null; then
    print_error "golangci-lint is not installed"
    print_info "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    echo ""
else
    LINT_OUTPUT=$(golangci-lint run ./... 2>&1)
    if [ -z "$LINT_OUTPUT" ]; then
        print_success "No linter errors found"
    else
        print_error "Linter errors found:"
        if [ "$VERBOSE" = true ]; then
            echo "$LINT_OUTPUT"
        else
            echo "$LINT_OUTPUT" | head -10
            TOTAL_LINES=$(echo "$LINT_OUTPUT" | wc -l)
            if [ "$TOTAL_LINES" -gt 10 ]; then
                print_info "... and $(($TOTAL_LINES - 10)) more errors (use --verbose to see all)"
            fi
        fi
    fi
fi
echo ""

# ============================================================================
# 3. RUN TESTS
# ============================================================================
print_check "Running Tests"
echo ""

TEST_OUTPUT=$(go test ./... -v 2>&1)
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    # Count passed tests
    TOTAL_TESTS=$(echo "$TEST_OUTPUT" | grep -c "^PASS" || echo "0")
    print_success "All tests passed ($TOTAL_TESTS packages)"

    if [ "$VERBOSE" = true ]; then
        echo ""
        print_info "Test summary:"
        echo "$TEST_OUTPUT" | grep -E "^(ok|PASS)" | sed 's/^/    /'
    fi
else
    print_error "Some tests failed"
    if [ "$VERBOSE" = true ]; then
        echo "$TEST_OUTPUT"
    else
        echo "$TEST_OUTPUT" | grep -E "(FAIL|--- FAIL)" | head -20
    fi
fi
echo ""

# ============================================================================
# 4. CHECK COVERAGE
# ============================================================================
print_check "Code Coverage"
echo ""

print_info "Running tests with coverage analysis..."
COVERAGE_FILE=".coverage-check.out"
go test -coverprofile=$COVERAGE_FILE -covermode=atomic ./... > /dev/null 2>&1

if [ ! -f "$COVERAGE_FILE" ]; then
    print_error "Failed to generate coverage report"
else
    # Get overall coverage
    COVERAGE=$(go tool cover -func=$COVERAGE_FILE | grep total | awk '{print substr($3, 1, length($3)-1)}')

    echo ""
    print_info "Overall Coverage: ${COVERAGE}%"

    # Check if coverage meets threshold (portable comparison without bc)
    MEETS_THRESHOLD=$(awk -v coverage="$COVERAGE" -v threshold="$COVERAGE_THRESHOLD" 'BEGIN { print (coverage >= threshold) ? 1 : 0 }')
    if [ "$MEETS_THRESHOLD" -eq 1 ]; then
        print_success "Coverage meets threshold (≥${COVERAGE_THRESHOLD}%)"
    else
        print_error "Coverage below threshold: ${COVERAGE}% < ${COVERAGE_THRESHOLD}%"
    fi

    echo ""
    print_info "Coverage by Package:"
    echo ""

    # Extract package coverage details from test output
    echo -e "${BLUE}  Package                          Coverage    Status${NC}"
    echo "  ────────────────────────────────────────────────────────"

    go test ./... -cover 2>&1 | grep "coverage:" | grep "ok\|%" | while IFS= read -r line; do
        # Extract package name and coverage percentage
        if echo "$line" | grep -q "ok"; then
            PACKAGE=$(echo "$line" | awk '{print $2}' | awk -F'/' '{print $NF}')
            PERCENT=$(echo "$line" | grep -oE '[0-9]+\.[0-9]+%' | sed 's/%//')
        else
            PACKAGE=$(echo "$line" | awk '{print $1}' | awk -F'/' '{print $NF}')
            PERCENT=$(echo "$line" | grep -oE '[0-9]+\.[0-9]+%' | sed 's/%//')
        fi

        # Skip if we couldn't extract data
        [ -z "$PACKAGE" ] || [ -z "$PERCENT" ] && continue

        # Determine status icon (portable without bc)
        if [ "$(awk -v p="$PERCENT" 'BEGIN { print (p >= 90.0) ? 1 : 0 }')" -eq 1 ]; then
            STATUS="${GREEN}✓ Excellent${NC}"
        elif [ "$(awk -v p="$PERCENT" 'BEGIN { print (p >= 70.0) ? 1 : 0 }')" -eq 1 ]; then
            STATUS="${GREEN}✓ Good${NC}"
        elif [ "$(awk -v p="$PERCENT" 'BEGIN { print (p >= 50.0) ? 1 : 0 }')" -eq 1 ]; then
            STATUS="${YELLOW}⚠ Moderate${NC}"
        else
            STATUS="${RED}✗ Low${NC}"
        fi

        printf "  %-30s %6s%%    %b\n" "$PACKAGE" "$PERCENT" "$STATUS"
    done

    echo ""

    # Show detailed coverage for core packages if verbose
    if [ "$VERBOSE" = true ]; then
        echo ""
        print_info "Detailed Coverage (Core Packages):"
        echo ""

        CORE_PACKAGES=("internal/models" "internal/config" "internal/api/handlers" "internal/api/middleware" "pkg/logger")

        for pkg in "${CORE_PACKAGES[@]}"; do
            echo -e "${BLUE}  $pkg:${NC}"
            go tool cover -func=$COVERAGE_FILE | grep "$pkg" | grep -v "total:" | awk '{printf "    %-50s %s\n", $2, $3}' | head -10
            echo ""
        done
    fi

    # Cleanup coverage file
    rm -f $COVERAGE_FILE
fi

echo ""

# ============================================================================
# SUMMARY
# ============================================================================
echo -e "${BLUE}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                         SUMMARY                                ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""

TOTAL_CHECKS=$((PASSED + FAILED))

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! ($PASSED/$TOTAL_CHECKS)${NC}"
    echo ""
    echo -e "${GREEN}  Your code is ready to commit! 🚀${NC}"
    exit 0
else
    echo -e "${RED}✗ Some checks failed ($FAILED/$TOTAL_CHECKS failed, $PASSED/$TOTAL_CHECKS passed)${NC}"
    echo ""
    echo -e "${YELLOW}  Please fix the issues above before committing.${NC}"
    exit 1
fi
