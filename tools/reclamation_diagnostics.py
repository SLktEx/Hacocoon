"""Read-only, fixed-vocabulary observations after a retention assertion fails."""
import json
import subprocess
import tempfile


def project(raw):
    tokens = {
        "base_resolution": "resolve Base ",
        "project": "ensure Incus project:",
        "root_storage": "resolve isolated root storage:",
        "routed_substrate": "ensure Hacocoon routed sandbox substrate:",
        "sandbox_proxy": "resolve Hacocoon sandbox proxy configuration:",
        "instance_init": "init isolated Incus environment ",
        "runtime_unavailable": "runtime unavailable",
        "cleanup_incomplete": "manual recovery required",
        "stale_identity": "capability request no longer matches current state",
        "workspace_busy": "workspace busy",
        "storage_busy": "storage busy",
    }
    # No journal fields or arbitrary messages leave this projection.
    observed = set()
    if len(raw) > 1 << 20:
        return {"state": "oversized"}
    try:
        for line in raw.splitlines():
            row = json.loads(line)
            message = row.get("MESSAGE")
            if not isinstance(message, str):
                continue
            observed.update(label for label, token in tokens.items() if token in message)
    except (ValueError, TypeError, AttributeError):
        return {"state": "invalid"}
    return {"state": "observed", "observations": sorted(observed)}


def main():
    try:
        with tempfile.TemporaryFile() as output:
            result = subprocess.run(["journalctl", "-u", "haco-controller.service",
                                     "--since=-5min", "--lines=200", "--output=json", "--no-pager"],
                                    stdout=output, stderr=subprocess.DEVNULL, timeout=10)
            output.seek(0)
            result = project(output.read((1 << 20) + 1)) if result.returncode == 0 else {"state": "unavailable"}
    except (OSError, subprocess.TimeoutExpired):
        result = {"state": "unavailable"}
    print(json.dumps(result))


if __name__ == "__main__":
    main()
