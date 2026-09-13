#!/usr/bin/env bash
set -euo pipefail

real_go="${HACO_REAL_GO:-}"
ci_bin_dir="${HACO_CI_BIN_DIR:-}"

if [[ -z "$real_go" || ! -x "$real_go" ]]; then
  echo "HACO_REAL_GO must point to the real Go executable" >&2
  exit 2
fi

# Reuse an artifact only for the exact plain build shape used by repository E2Es:
#   go build -o <path> ./cmd/<name>
# Any tags, linker flags, alternate packages, or future build shape falls through
# to the real Go tool so this optimization cannot silently change test semantics.
if [[ "$#" == 4 && "$1" == "build" && "$2" == "-o" && -n "$ci_bin_dir" ]]; then
  output="$3"
  package="$4"
  source_name=""
  case "$package" in
    ./cmd/haco-product) source_name="haco" ;;
    ./cmd/haco) source_name="hacoq" ;;
    ./cmd/haco-controller) source_name="haco-controller" ;;
    ./cmd/haco-host) source_name="haco-host" ;;
    ./cmd/haco-vscode) source_name="haco-vscode" ;;
    ./cmd/haco-agent-host) source_name="haco-agent-host" ;;
    ./cmd/haco-notify) source_name="haco-notify" ;;
  esac

  if [[ -n "$source_name" && -f "$ci_bin_dir/$source_name" ]]; then
    install -m 0755 "$ci_bin_dir/$source_name" "$output"
    echo "CI: reused prebuilt $source_name for $package" >&2
    exit 0
  fi
fi

exec "$real_go" "$@"
