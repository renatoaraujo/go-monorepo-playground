#!/bin/bash

# Test runner script for Go monorepo workspace
# This script ensures all tests run properly from the root directory

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}⚠${NC} $1"
}

# Function to run tests in a specific directory
run_tests_in_dir() {
    local dir=$1
    local test_args=${2:-"-cover"}

    if [[ -f "$dir/go.mod" ]]; then
        print_status "Testing module in $dir"

        # Check if there are any test files
        if find "$dir" -name "*_test.go" -type f | grep -q .; then
            cd "$dir"

            # Run the tests
            if go test $test_args ./...; then
                print_success "Tests passed in $dir"
            else
                print_error "Tests failed in $dir"
                return 1
            fi

            cd - > /dev/null
        else
            print_warning "No test files found in $dir"
        fi
    else
        print_warning "No go.mod found in $dir, skipping"
    fi
}

# Main function to run all tests
run_all_tests() {
    local test_args=${1:-"-cover"}

    print_status "Running tests for all modules in workspace"

    # Sync the workspace
    go work sync

    # Test all modules in workspace
    local failed=0

    # Get all module directories from go.work
    while IFS= read -r dir; do
        if [[ -n "$dir" && "$dir" != "." ]]; then
            if ! run_tests_in_dir "$dir" "$test_args"; then
                failed=1
            fi
        fi
    done < <(grep -E "^\s*\." go.work | grep -v "^//" | sed 's/.*\.\///g' | sed 's/[[:space:]]*//g')

    # Also test the root module if it exists
    if [[ -f "go.mod" ]]; then
        if ! run_tests_in_dir "." "$test_args"; then
            failed=1
        fi
    fi

    if [[ $failed -eq 0 ]]; then
        print_success "All tests passed!"
        return 0
    else
        print_error "Some tests failed"
        return 1
    fi
}

# Function to run tests for a specific service
run_service_tests() {
    local service=$1
    local test_args=${2:-"-cover"}

    local service_dir="services/$service"

    if [[ -d "$service_dir" ]]; then
        run_tests_in_dir "$service_dir" "$test_args"
    else
        print_error "Service directory $service_dir not found"
        return 1
    fi
}

# Function to run tests for all packages
run_package_tests() {
    local test_args=${1:-"-cover"}

    print_status "Running tests for all packages"

    local failed=0

    for pkg_dir in pkg/*/; do
        if [[ -d "$pkg_dir" ]]; then
            if ! run_tests_in_dir "$pkg_dir" "$test_args"; then
                failed=1
            fi
        fi
    done

    if [[ $failed -eq 0 ]]; then
        print_success "All package tests passed!"
        return 0
    else
        print_error "Some package tests failed"
        return 1
    fi
}

# Function to generate coverage report
generate_coverage() {
    print_status "Generating coverage report"

    go work sync

    # Remove existing coverage files
    rm -f coverage.out coverage.tmp

    # Initialize coverage file
    echo "mode: atomic" > coverage.out

    # Run tests with coverage for each module
    while IFS= read -r dir; do
        if [[ -n "$dir" && "$dir" != "." ]]; then
            if [[ -f "$dir/go.mod" ]]; then
                print_status "Collecting coverage for $dir"

                cd "$dir"

                if find . -name "*_test.go" -type f | grep -q .; then
                    if go test -coverprofile=coverage.tmp -cover ./...; then
                        if [[ -f coverage.tmp ]]; then
                            tail -n +2 coverage.tmp >> "../coverage.out"
                            rm coverage.tmp
                        fi
                    fi
                fi

                cd - > /dev/null
            fi
        fi
    done < <(grep -E "^\s*\." go.work | grep -v "^//" | sed 's/.*\.\///g' | sed 's/[[:space:]]*//g')

    # Also handle root module if it exists
    if [[ -f "go.mod" ]]; then
        if find . -maxdepth 1 -name "*_test.go" -type f | grep -q .; then
            if go test -coverprofile=coverage.tmp -cover .; then
                if [[ -f coverage.tmp ]]; then
                    tail -n +2 coverage.tmp >> coverage.out
                    rm coverage.tmp
                fi
            fi
        fi
    fi

    print_success "Coverage report generated: coverage.out"
}

# Parse command line arguments
case "${1:-all}" in
    "all")
        run_all_tests "${2:-"-cover"}"
        ;;
    "verbose")
        run_all_tests "-v -cover"
        ;;
    "producer")
        run_service_tests "producer" "${2:-"-cover"}"
        ;;
    "consumer")
        run_service_tests "consumer" "${2:-"-cover"}"
        ;;
    "packages")
        run_package_tests "${2:-"-cover"}"
        ;;
    "coverage")
        generate_coverage
        ;;
    "race")
        run_all_tests "-race -cover"
        ;;
    "bench")
        run_all_tests "-bench=. -benchmem"
        ;;
    *)
        echo "Usage: $0 [all|verbose|producer|consumer|packages|coverage|race|bench]"
        echo ""
        echo "Commands:"
        echo "  all       - Run all tests (default)"
        echo "  verbose   - Run all tests with verbose output"
        echo "  producer  - Run tests for producer service only"
        echo "  consumer  - Run tests for consumer service only"
        echo "  packages  - Run tests for all packages only"
        echo "  coverage  - Generate coverage report"
        echo "  race      - Run tests with race detection"
        echo "  bench     - Run benchmark tests"
        exit 1
        ;;
esac
