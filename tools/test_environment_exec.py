"""Ordinary persistent Environment execution on a disposable real-Incus host."""
import json
import os
import secrets
import sys
from environment_exec_stream import verify_streaming
from environment_exec_cancel import cleanup_environment, run_product, verify_cancellation


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted":
        raise SystemExit("Environment acceptance requires the disposable GHA host")
    product, image = sys.argv[1:]
    name = "exec-" + secrets.token_hex(5)

    def invoke(*args, **kwargs):
        return run_product(product, *args, **kwargs)

    def rows(*, timeout=300):
        return {row["name"]: row for row in json.loads(invoke("env", "ls", "--json", timeout=timeout))}

    baseline = rows()
    created, original = False, None
    try:
        invoke("open", "--new", image, "--name", name, "--client", "none")
        created = True
        # Capture ownership before further guest work. If lookup fails, retain the
        # unverified target rather than letting a name-only cleanup guess ownership.
        original = rows().get(name)
        if not original or not original.get("runtime_ref") or not original.get("created_at"):
            original = None
            raise RuntimeError("created Environment identity was unavailable")
        assert invoke("exec", name, "--", "sh", "-ec", 'test "$PWD" = /workspace; printf exec-ok') == "exec-ok"
        invoke("exec", name, "--", "sh", "-c", "exit 17", expected=17)
        verify_streaming(product, name, rows)
        verify_cancellation(product, name, original, invoke, rows)
        invoke("stop", name)
        invoke("exec", name, "--", "true", expected=1)
    finally:
        if created:
            cleanup_environment(name, original, invoke, rows, sys.exc_info()[1])
    assert rows() == baseline
    print("ENVIRONMENT EXEC / EXIT 17 / STREAM / CANCEL / STOPPED REFUSAL: PASS")


if __name__ == "__main__":
    main()
