#!/usr/bin/env python3
"""Installed acceptance through ordinary haco commands in a disposable SSH fixture.

Runs inside the trusted Host. No controller/provider repair, direct Policy write,
credential injection or guest management endpoint is used.
"""

import json
import pathlib
import re
import subprocess
import sys
import tempfile
import time


class CommandFailure(RuntimeError):
    def __init__(self, result):
        self.category = command_failure_category(result.stdout, result.stderr)
        super().__init__("haco command failed")


def command_failure_category(stdout, stderr):
    if re.search(r"Unit hacocoon-project-setup(?:\.service)? (?:already exists|is already loaded|was already loaded)", stderr):
        return "unit-busy"
    if "Temporary failure resolving" in stderr or "Temporary failure resolving" in stdout:
        return "dns-failed"
    if "Could not get lock" in stderr:
        return "package-lock"
    for marker, category in (("PENDING_PREREQ_READY", "after-prerequisite"),
                             ("PENDING_PREREQ_UPDATED", "package-install"),
                             ("PENDING_PREREQ_STARTED", "package-update")):
        if marker in stdout.splitlines():
            return category
    return "command"


def command(*args, input_text=None, timeout=90):
    result = subprocess.run(
        ["/usr/local/bin/haco", *args], input=input_text, text=True,
        stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=timeout,
    )
    if result.returncode:
        raise CommandFailure(result)
    return result.stdout


def configuration_update(directory, mutate):
    snapshot = json.loads(command("config"))
    mutate(snapshot["policy"])
    file = directory / "configuration.json"
    file.write_text(json.dumps(snapshot), encoding="utf-8")
    file.chmod(0o600)
    receipt = json.loads(command("config", "--file", str(file)))
    if receipt.get("policy") != snapshot["policy"] or not re.fullmatch(
        r"sha256:[a-f0-9]{64}", receipt.get("revision", "")
    ):
        raise RuntimeError("configuration receipt mismatch")


def valid_network_output(output):
    return output.splitlines() == ["PENDING_NETWORK_RESULT_OK", "Project setup completed."]


def main(environment):
    if not re.fullmatch(r"win-ssh-[a-f0-9]{16}", environment):
        raise RuntimeError("requires a disposable Windows SSH acceptance Environment")
    rule = {
        "capability": "network.egress", "action": "connect", "resource": "example.com",
        "environment": environment, "attributes": {"protocol": "https", "port": "443"},
        "decision": "require-approval", "reason": "pending approval acceptance " + environment,
    }
    saved_rule = None
    process = None
    configured = False

    recipe_touched = False
    phase = "configure"
    failure = None
    step = "configuration"
    cleanup_failed = False
    with tempfile.TemporaryDirectory(prefix="haco-approval-acceptance-") as temporary:
        directory = pathlib.Path(temporary)
        try:
            def add(policy):
                nonlocal configured
                if any(r.get("environment") == environment and r.get("resource") == "example.com"
                       and r.get("capability") == "network.egress"
                       for r in policy.get("rules", []) + policy.get("saved_decisions", [])):
                    raise RuntimeError("test Policy scope already exists")
                configured = True
                policy.setdefault("rules", []).append(rule)

            # add marks cleanup necessary only after proving the scope absent,
            # but before the fallible apply/receipt check.
            configuration_update(directory, add)
            phase = "prepare"
            step = "python-prerequisite"
            prerequisite = directory / "prerequisite.sh"
            prerequisite.write_text("set -eu\necho PENDING_PREREQ_STARTED\nif ! test -x /usr/bin/python3; then\n  apt-get update\n  echo PENDING_PREREQ_UPDATED\n  apt-get install -y --no-install-recommends python3\nfi\necho PENDING_PREREQ_READY\n", encoding="utf-8")
            prerequisite.chmod(0o600)
            recipe_touched = True
            command("setup", "--script", str(prerequisite), environment, timeout=240)
            command("setup", "--clear-script", environment)
            for phase, answer, allowed in (
                ("saved-ask-deny", "5\nn\n", False),
                ("one-shot-allow", "y\n", True),
                ("reask-deny", "n\n", False),
            ):
                step = "start-probe"
                recipe = directory / "probe.sh"
                recipe.write_text("""set -eu
test -x /usr/bin/python3
/usr/bin/python3 - <<'PY'
import urllib.request, urllib.error
allowed = ALLOWED
try:
    with urllib.request.urlopen('https://example.com/', timeout=90) as response:
        if not allowed or response.status != 200:
            raise SystemExit('unexpected network success')
        response.read(1024)
except urllib.error.URLError as error:
    if allowed or '403' not in str(error):
        raise SystemExit('unexpected network failure')
print('PENDING_NETWORK_RESULT_OK')
PY
""".replace("ALLOWED", repr(allowed)), encoding="utf-8")
                recipe.chmod(0o600)
                recipe_touched = True
                process = subprocess.Popen(
                    ["/usr/local/bin/haco", "setup", "--script", str(recipe), environment],
                    text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                )
                step = "wait-pending"
                deadline = time.monotonic() + 60
                prompt = None
                while time.monotonic() < deadline:
                    requests = json.loads(command("approve", "--list"))
                    matches = [p for p in requests if p.get("request", {}).get("environment") == environment
                               and p.get("request", {}).get("capability") == "network.egress"
                               and p.get("request", {}).get("resource") == "example.com"]
                    if len(matches) > 1:
                        raise RuntimeError("ambiguous test approval")
                    if matches:
                        prompt = matches[0]
                        break
                    if process.poll() is not None:
                        raise RuntimeError("network probe exited before pending review")
                    time.sleep(0.1)
                step = "validate-prompt"
                if prompt is None or not re.fullmatch(r"[a-f0-9]{32}", prompt.get("request_id", "")):
                    raise RuntimeError("no valid pending request")
                if prompt.get("request", {}).get("attributes") != {"protocol": "https", "port": "443"}:
                    raise RuntimeError("unexpected request scope")
                if phase == "saved-ask-deny":
                    candidate = dict(prompt["saved_scope"])
                    if not re.fullmatch(r"env-[a-f0-9]{32}", candidate.get("environment_instance", "")):
                        raise RuntimeError("missing trusted Environment identity")
                    if any(candidate.get(key) != rule[key] for key in ("capability", "action", "resource", "environment", "attributes")):
                        raise RuntimeError("unexpected saved scope")
                    saved_rule = candidate
                    saved_rule["decision"] = "require-approval"
                step = "submit-review"
                receipt = json.loads(command("approve", "--json", prompt["request_id"], input_text=answer))
                step = "validate-receipt"
                expected_saved = "ask-environment" if phase == "saved-ask-deny" else ""
                if receipt.get("request_id") != prompt["request_id"] or receipt.get("saved_choice", "") != expected_saved:
                    raise RuntimeError("approval receipt mismatch")
                expected_state = "succeeded" if allowed else "not-executed"
                if receipt.get("execution_state") != expected_state or (allowed and not receipt.get("audit_complete")):
                    raise RuntimeError("approval execution state mismatch")
                step = "network-result"
                output, _ = process.communicate(timeout=120)
                if process.returncode or not valid_network_output(output):
                    raise RuntimeError("actual network result mismatch")
                process = None
                step = "clear-recipe"
                command("setup", "--clear-script", environment)
                if phase == "saved-ask-deny":
                    def remove_administrator_ask(policy):
                        if policy.get("saved_decisions", []).count(saved_rule) != 1:
                            raise RuntimeError("saved ask was not persisted exactly")
                        policy["rules"] = [r for r in policy["rules"] if r != rule]
                    step = "verify-saved-policy"
                    configuration_update(directory, remove_administrator_ask)
        except Exception as error:
            category = error.category if isinstance(error, CommandFailure) else "timeout" if isinstance(error, subprocess.TimeoutExpired) else "validation"
            failure = phase + "-" + step + "-" + category
        finally:
            if process is not None:
                try:
                    if process.poll() is None:
                        process.terminate()
                    process.communicate(timeout=15)
                except Exception:
                    cleanup_failed = True
            if recipe_touched:
                try:
                    command("setup", "--clear-script", environment)
                except Exception:
                    cleanup_failed = True
            if configured:
                try:
                    def remove(policy):
                        policy["rules"] = [r for r in policy.get("rules", []) if r != rule]
                        if saved_rule is not None and "saved_decisions" in policy:
                            policy["saved_decisions"] = [r for r in policy["saved_decisions"] if r != saved_rule]
                            if not policy["saved_decisions"]:
                                del policy["saved_decisions"]
                    configuration_update(directory, remove)
                except Exception:
                    cleanup_failed = True
        if failure or cleanup_failed:
            print("PENDING APPROVAL REVIEW: FAIL phase=" + (failure or "cleanup")
                  + " cleanup_failed=" + str(cleanup_failed).lower(), file=sys.stderr)
            return 1
    print("PENDING_REVIEW_SAVED_ASK_DENY / ONE_SHOT_ALLOW / REASK_DENY: PASS")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: test_pending_approvals.py <disposable-environment>")
    raise SystemExit(main(sys.argv[1]))
