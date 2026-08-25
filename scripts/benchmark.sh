#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: scripts/benchmark.sh [--runs N] [--warmup N] FORGE_BIN

Requires hyperfine. N must be a non-negative integer; runs must be at least 30.
EOF
}

runs=30
warmup=5

while [[ $# -gt 0 ]]; do
  case $1 in
    --runs)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      runs=$2
      shift 2
      ;;
    --warmup)
      [[ $# -ge 2 ]] || { usage; exit 2; }
      warmup=$2
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --)
      shift
      break
      ;;
    -*)
      echo "error: unknown option: $1" >&2
      usage
      exit 2
      ;;
    *)
      break
      ;;
  esac
done

[[ $runs =~ ^[0-9]+$ ]] && (( runs >= 30 )) || {
  echo "error: --runs must be an integer of at least 30" >&2
  exit 2
}
[[ $warmup =~ ^[0-9]+$ ]] || {
  echo "error: --warmup must be a non-negative integer" >&2
  exit 2
}
[[ $# -eq 1 ]] || { usage; exit 2; }

forge_bin=$1
if [[ ! -f $forge_bin || ! -x $forge_bin ]]; then
  echo "error: Forge executable is not an executable file: $forge_bin" >&2
  exit 1
fi
command -v hyperfine >/dev/null 2>&1 || {
  echo "error: hyperfine 1.18.0 or newer is required" >&2
  exit 1
}
hyperfine_version=$(hyperfine --version 2>/dev/null | awk 'NR == 1 { print $2 }')
if [[ ! $hyperfine_version =~ ^([0-9]+)\.([0-9]+)(\.|$) ]] \
    || (( BASH_REMATCH[1] < 1 )) \
    || (( BASH_REMATCH[1] == 1 && BASH_REMATCH[2] < 18 )); then
  echo "error: hyperfine 1.18.0 or newer is required" >&2
  exit 1
fi

forge_dir=$(cd "$(dirname "$forge_bin")" && pwd)
forge_bin=$forge_dir/$(basename "$forge_bin")
work_dir=$(mktemp -d "${TMPDIR:-/tmp}/forge-benchmark.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT
data_dir=$work_dir/data
binary_size=$(wc -c <"$forge_bin" | tr -d '[:space:]')

quote_command() {
  local quoted
  printf -v quoted '%q ' env "FORGE_DATA_DIR=$data_dir" "$forge_bin" "$@"
  printf '%s' "${quoted% }"
}

summarize_times() {
  awk '
    /"times"[[:space:]]*:/ { in_times = 1; next }
    in_times && /]/ { in_times = 0 }
    in_times {
      value = $0
      gsub(/[^0-9.eE+-]/, "", value)
      if (value != "") times[++count] = value + 0
    }
    END {
      if (count == 0) exit 1
      for (i = 2; i <= count; i++) {
        value = times[i]
        j = i - 1
        while (j >= 1 && times[j] > value) {
          times[j + 1] = times[j]
          j--
        }
        times[j + 1] = value
      }
      if (count % 2) median = times[(count + 1) / 2]
      else median = (times[count / 2] + times[count / 2 + 1]) / 2
      rank = int(0.9 * count)
      if (rank < 0.9 * count) rank++
      printf "median_ms=%.3f p90_ms=%.3f", median * 1000, times[rank] * 1000
    }
  ' "$1"
}

peak_rss_kib() {
  local output=$work_dir/time.out
  if [[ -x /usr/bin/time ]] && /usr/bin/time --version 2>&1 | grep -q GNU; then
    /usr/bin/time -f '%M' -o "$output" \
      env "FORGE_DATA_DIR=$data_dir" "$forge_bin" "$@" \
      >/dev/null 2>/dev/null || return 1
    tr -d '[:space:]' <"$output"
    return
  fi
  if [[ -x /usr/bin/time ]]; then
    /usr/bin/time -l \
      env "FORGE_DATA_DIR=$data_dir" "$forge_bin" "$@" \
      >/dev/null 2>"$output" || return 1
    awk '/maximum resident set size/ { printf "%d", $1 / 1024; found = 1 }
         END { if (!found) exit 1 }' "$output"
    return
  fi
  return 1
}

benchmark_case() {
  local name=$1
  shift
  local result=$work_dir/$name.json
  local command rss summary
  command=$(quote_command "$@")

  hyperfine --style none --warmup "$warmup" --runs "$runs" \
    --export-json "$result" "$command" >/dev/null
  summary=$(summarize_times "$result") || {
    echo "error: could not read hyperfine timing data for $name" >&2
    exit 1
  }
  rss=$(peak_rss_kib "$@" || true)
  [[ -n $rss ]] || rss=unavailable
  printf 'case=%s runs=%s %s peak_rss_kib=%s\n' \
    "$name" "$runs" "$summary" "$rss"
}

printf 'host_os=%s host_arch=%s hyperfine_version=%s\n' \
  "$(uname -s)" "$(uname -m)" "$hyperfine_version"
printf 'binary_size_bytes=%s\n' "$binary_size"
benchmark_case help --help
benchmark_case version --version

if [[ -e $data_dir ]]; then
  echo "error: a data-free command created FORGE_DATA_DIR" >&2
  exit 1
fi
