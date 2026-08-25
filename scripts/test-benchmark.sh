#!/usr/bin/env bash

set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
test_dir=$(mktemp -d "${TMPDIR:-/tmp}/forge-benchmark-test.XXXXXX")
trap 'rm -rf "$test_dir"' EXIT

fake_bin=$test_dir/forge
printf '%s\n' \
  '#!/usr/bin/env bash' \
  '[[ ! -e ${FORGE_DATA_DIR:?} ]]' \
  'case ${1-} in --help) echo help;; --version) echo "forge test";; *) exit 2;; esac' \
  >"$fake_bin"
chmod +x "$fake_bin"

# This shim makes the harness self-test independent of an installed hyperfine.
fake_tools=$test_dir/tools
mkdir "$fake_tools"
fake_hyperfine=$fake_tools/hyperfine
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'if [[ ${1-} == --version ]]; then echo "hyperfine 1.18.0"; exit; fi' \
  'output=' \
  'command=' \
  'while [[ $# -gt 0 ]]; do' \
  '  case $1 in' \
  '    --export-json) output=$2; shift 2;;' \
  '    --style|--warmup|--runs) shift 2;;' \
  '    *) command=$1; shift;;' \
  '  esac' \
  'done' \
  'bash -c "$command" >/dev/null' \
  'printf "%s\n" '\''{ "results": [{ "times": ['\'' >"$output"' \
  'for ((i = 1; i <= 30; i++)); do printf "0.%03d,\n" "$i" >>"$output"; done' \
  'printf "%s\n" '\''] }] }'\'' >>"$output"' \
  >"$fake_hyperfine"
chmod +x "$fake_hyperfine"

report=$test_dir/report
PATH="$fake_tools:$PATH" "$repo_dir/scripts/benchmark.sh" \
  --runs 30 --warmup 0 "$fake_bin" >"$report"

grep -Eq '^host_os=.+ host_arch=.+ hyperfine_version=1\.18\.0$' "$report"
grep -Eq '^binary_size_bytes=[1-9][0-9]*$' "$report"
grep -Eq '^case=help runs=30 median_ms=15\.500 p90_ms=27\.000 ' "$report"
grep -Eq '^case=version runs=30 median_ms=15\.500 p90_ms=27\.000 ' "$report"

if PATH="$fake_tools:$PATH" "$repo_dir/scripts/benchmark.sh" \
    --runs 29 "$fake_bin" >/dev/null 2>&1; then
  echo "expected fewer than 30 runs to be rejected" >&2
  exit 1
fi

writing_bin=$test_dir/writing-forge
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'mkdir -p "${FORGE_DATA_DIR:?}"' \
  >"$writing_bin"
chmod +x "$writing_bin"
if PATH="$fake_tools:$PATH" "$repo_dir/scripts/benchmark.sh" \
    --runs 30 --warmup 0 "$writing_bin" >/dev/null 2>&1; then
  echo "expected data-directory creation to fail the benchmark" >&2
  exit 1
fi

echo "benchmark harness tests passed"
