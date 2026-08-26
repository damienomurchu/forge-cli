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

# Build a local Forge executable at bin/forge.
build:
    mkdir -p bin
    mise exec -- go build -o bin/forge ./cmd/forge

# Build a stripped local executable for performance measurement.
build-release:
    mkdir -p bin
    mise exec -- go build -trimpath -ldflags="-s -w" -o bin/forge ./cmd/forge

# Build Forge and exercise the implemented help and version paths.
try: build
    ./bin/forge --help
    ./bin/forge --version

# Verify the supported release targets build without CGO.
cross-build:
    env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 mise exec -- go build ./...
    env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 mise exec -- go build ./...

# Run the standard local quality checks.
check: fmt-check mod-check vet test test-race test-cover vuln

# Run all checks expected in CI.
ci: check cross-build benchmark-test completion-check

# Generate completion scripts (usage: just completions [OUTPUT_DIR]).
completions output_dir="completions": build
    mkdir -p "{{output_dir}}"
    ./bin/forge completion bash > "{{output_dir}}/forge.bash"
    ./bin/forge completion fish > "{{output_dir}}/forge.fish"
    ./bin/forge completion zsh > "{{output_dir}}/_forge"

# Generate and syntax-check completion scripts with installed shells.
completion-check: build
    completion_dir="$(mktemp -d)"; trap 'rm -rf "$completion_dir"' EXIT; \
      ./bin/forge completion bash > "$completion_dir/forge.bash"; \
      ./bin/forge completion fish > "$completion_dir/forge.fish"; \
      ./bin/forge completion zsh > "$completion_dir/_forge"; \
      bash -n "$completion_dir/forge.bash"; \
      if command -v fish >/dev/null; then fish -n "$completion_dir/forge.fish"; fi; \
      if command -v zsh >/dev/null; then zsh -n "$completion_dir/_forge"; fi

# Test the benchmark harness without Forge or hyperfine.
benchmark-test:
    scripts/test-benchmark.sh

# Build and benchmark a release-style local Forge executable.
bench: build-release
    scripts/benchmark.sh ./bin/forge

# Benchmark a candidate Forge executable (usage: just benchmark ./forge).
benchmark forge_bin:
    scripts/benchmark.sh "{{forge_bin}}"
