#!/usr/bin/env bash
set -euo pipefail

if [[ "${HACO_E2E_OCI_NERDCTL:-}" != "1" ]]; then
  echo 'SKIP: set HACO_E2E_OCI_NERDCTL=1 with real containerd/nerdctl'
  exit 0
fi

for command in go nerdctl containerd sudo python3; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "missing required command: $command" >&2
    exit 1
  }
done

root="$(mktemp -d)"
haco="$root/haco"
cleanup() {
  set +e
  sudo rm -rf "$root"
}
trap cleanup EXIT

go build -o "$haco" ./cmd/haco
sudo mkdir -p "$root/haco-root"

run_haco() {
  sudo --preserve-env=PATH env \
    HACO_ROOT="$root/haco-root" \
    HACO_PLUGIN_OCI=nerdctl \
    HACO_STORAGE_PRIVILEGE_MODE=direct \
    "$haco" "$@"
}

# Prove the process is talking to the real Host containerd namespace before
# asking Hacocoon to exercise the same driver path.
sudo nerdctl --namespace hacocoon-seed version >/dev/null
[[ "$(run_haco plugin oci status)" == 'driver: nerdctl' ]]

sample_json="$(run_haco plugin oci seed sample --json)"
recommend_json="$(run_haco plugin oci seed recommend --json)"
python3 - "$sample_json" "$recommend_json" <<'PY'
import json,sys
sample=json.loads(sys.argv[1]); recommend=json.loads(sys.argv[2])
assert sample['sampled'] == 0 and sample['failed'] == 0, sample
assert recommend['recommendations'] == [], recommend
PY

identity='example.invalid/hacocoon/real-nerdctl:latest@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc'
delete_json="$(run_haco plugin oci image delete "$identity" --json)"
python3 - "$delete_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1])
assert row['host_cache'] == 'not-present', row
assert row['seed_rebuild_required'] is True, row
PY

reenable_json="$(run_haco plugin oci image reenable "$identity" --json)"
python3 - "$reenable_json" <<'PY'
import json,sys
row=json.loads(sys.argv[1]); assert row['removed'] is True, row
PY

# The exact namespace inventory call is the real integration boundary used by
# OCI deletion; it should remain healthy after the tombstone round trip.
sudo nerdctl --namespace hacocoon-seed images --format '{{.Repository}}\t{{.Tag}}\t{{.Digest}}' >/dev/null

echo 'PASS: haco plugin oci -> real nerdctl/containerd acceptance'
