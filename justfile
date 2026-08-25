set shell := ["bash", "-euo", "pipefail", "-c"]

# List available development tasks.
default:
    @just --list

# Format all Go source files.
fmt:
    mise exec -- go fmt ./...

# Fail if any Go source file needs formatting.
fmt-check:
    @unformatted="$(mise exec -- gofmt -l $(rg --files -g '*.go'))"; \
      if [[ -n "$unformatted" ]]; then \
        printf 'Go files need formatting:\n%s\n' "$unformatted"; \
        exit 1; \
      fi

# Fail if go.mod or go.sum is not tidy.
mod-check:
    mise exec -- go mod tidy -diff

# Run static analysis.
vet:
    mise exec -- go vet ./...

# Run the test suite.
test:
    mise exec -- go test ./...

# Run tests with the race detector.
test-race:
    mise exec -- go test -race ./...

# Run tests and report package coverage.
test-cover:
    mise exec -- go test -cover ./...

# Scan dependencies and source for known vulnerabilities.
vuln:
    mise exec -- go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

# Build every package for the host platform.
build:
    mise exec -- go build ./...

# Verify the supported release targets build without CGO.
cross-build:
    env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 mise exec -- go build ./...
    env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 mise exec -- go build ./...

# Run the standard local quality checks.
check: fmt-check mod-check vet test test-race test-cover vuln

# Run all checks expected in CI.
ci: check cross-build benchmark-test

# Test the benchmark harness without Forge or hyperfine.
benchmark-test:
    scripts/test-benchmark.sh

# Benchmark a candidate Forge executable (usage: just benchmark ./forge).
benchmark forge_bin:
    scripts/benchmark.sh "{{forge_bin}}"
