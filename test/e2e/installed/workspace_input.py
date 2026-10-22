"""Installed Linux CLI/controller acceptance for independent linked-worktree input.

Run as an ordinary user on the disposable, fresh GitHub-hosted installation in
the installer journey, never against an existing installation: repo list omits
excluded registrations. Runner safeguards enforce the CI context, not freshness.
Only public product commands create/delete Environments and Workspaces or
register/unregister the source; native source data remains until runner teardown.
No catalog or Incus repair is used. Optional guest Git uses the documented
package-permission path through public config/exec commands, with temporary
exact-Environment grants. The registered upstream supplies routing, not input.
"""
import hashlib
import json
import os
from datetime import datetime, timedelta, timezone
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tempfile
import uuid


UPSTREAM = "https://github.com/SLktEx/Hacocoon.git"
REPORT_LIMIT = 16384
GIT_PREREQUISITE = '''
if ! command -v git >/dev/null 2>&1; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends git
fi
command -v git >/dev/null
git --version
'''


def public_receipt(value):
    """Retain only bounded public identities, never arbitrary response fields."""
    if not isinstance(value, dict):
        return {}
    result = {}
    patterns = {
        "name": r"[a-z0-9][a-z0-9-]{0,63}",
        "id": r"[a-z0-9][a-z0-9-]{0,63}",
        "owner": r"[a-f0-9]{32}",
        "workspace": r"workspace:managed:[a-f0-9]{32}",
        # Installed composition uses env.Router's routed Incus references. Keep
        # this bounded public value opaque; never reconstruct a native identity.
        "runtime_ref": r"(?:haco-[a-z0-9][a-z0-9-]{0,63}|haco-runtime-v1:runtime\.incus:[A-Za-z0-9_-]{1,192})",
        "created_at": r"[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]{8,24}(?:Z|[+-][0-9]{2}:[0-9]{2})",
        "state": r"ready|importing|recovery-required|preparing|creating|created|deleting",
    }
    for key, pattern in patterns.items():
        item = value.get(key)
        if isinstance(item, str) and len(item) <= 256 and re.fullmatch(pattern, item):
            result[key] = item
    workspace = value.get("workspace")
    if isinstance(workspace, dict):
        item = workspace.get("id")
        if isinstance(item, str) and re.fullmatch(patterns["workspace"], item):
            result["workspace"] = {"id": item}
    if type(value.get("created")) is bool:
        result["created"] = value["created"]
    # One fixed nesting level; recurse neither through arbitrary metadata nor
    # through a hostile environment.environment chain.
    environment = value.get("environment")
    if isinstance(environment, dict):
        result["environment"] = public_receipt({key: environment[key] for key in
            ("name", "created_at", "runtime_ref", "workspace") if key in environment})
    return result


def failed_receipt(stdout):
    # workspace import emits its public PathReference even on failure. open
    # currently emits JSON only on success. Never retain raw subprocess output.
    if not isinstance(stdout, str) or len(stdout) > REPORT_LIMIT:
        return {}
    try:
        return public_receipt(json.loads(stdout))
    except (ValueError, RecursionError):
        return {}


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def require_disposable_runner(environ):
    require(environ.get("GITHUB_ACTIONS") == "true" and
            environ.get("HACO_CI_RUNNER_ENVIRONMENT") == "github-hosted",
            "installed input acceptance requires the disposable GitHub-hosted installer journey")


def run(*args, env=None):
    return subprocess.run(args, check=True, text=True, stdout=subprocess.PIPE,
                          stdin=subprocess.DEVNULL, env=env, timeout=300).stdout.strip()


def git(path, *args):
    # The fixture never borrows the runner's Git identity, hooks or credentials.
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL="/dev/null",
               GIT_OPTIONAL_LOCKS="0")
    return run("git", "-C", str(path), "-c", "core.hooksPath=/dev/null",
               "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
               *args, env=env)


def fixture(root):
    main, linked = root / "main", root / "linked"
    main.mkdir(mode=0o700)
    git(main, "init", "--template=", "--initial-branch=main")
    (main / "tracked").write_text("initial\n")
    git(main, "add", "tracked")
    git(main, "commit", "-m", "initial")
    git(main, "worktree", "add", "-b", "feature/input", str(linked))
    (linked / "selected").write_text("linked-only\n")
    git(linked, "add", "selected")
    git(linked, "commit", "-m", "selected worktree")
    selected_head = git(linked, "rev-parse", "HEAD")
    (linked / "tracked").write_text("staged\n")
    git(linked, "add", "tracked")
    (linked / "tracked").write_text("dirty\n")
    (linked / "extra").write_text("untracked\n")
    (main / "other-only").write_text("main-only\n")
    git(main, "config", "fixture.private", "source-config-must-not-be-imported")
    hooks = main / ".git/hooks"
    hooks.mkdir()
    (hooks / "pre-commit").write_text("#!/bin/sh\nexit 79\n")
    (hooks / "pre-commit").chmod(0o700)
    require((linked / ".git").is_file(), "fixture is not a linked worktree")
    require(git(main, "rev-parse", "HEAD") != selected_head, "fixture HEADs must differ")
    return main, linked, selected_head


def tree_digest(root):
    # Include the main checkout's common-dir, refs, indexes, hooks and config.
    # No source Git command runs between these snapshots, so index refreshes
    # cannot hide an import or guest write to Host administration state.
    digest = hashlib.sha256()
    for path in sorted(root.rglob("*")):
        mode = path.lstat().st_mode
        require(stat.S_ISREG(mode) or stat.S_ISDIR(mode), "unexpected source fixture entry")
        digest.update(str(path.relative_to(root)).encode() + b"\0")
        digest.update(str(mode).encode() + b"\0")
        if stat.S_ISREG(mode):
            digest.update(path.read_bytes())
    return digest.hexdigest()


def environment_identity(env):
    return (env["name"], env["created_at"], env["runtime_ref"], env["workspace"]["id"])


class Acceptance:
    def __init__(self, root):
        self.root = root
        self.prefix = "input-" + uuid.uuid4().hex[:12]
        self.source = None
        self.registration_receipt = None
        self.import_receipts = []
        self.open_receipts = []
        self.workspaces = []
        self.environments = []
        self.inputs = []
        self.package_rules = []
        self.package_add_observed = False
        self.package_remove_pending = False
        self.save()

    @staticmethod
    def haco(*args):
        return run("haco", *args)

    def query(self, *args):
        return json.loads(self.haco(*args))

    def report(self):
        return {
            "requested_source": self.prefix, "upstream": UPSTREAM,
            "registration_receipt": public_receipt(self.registration_receipt),
            "import_receipts": self.import_receipts, "open_receipts": self.open_receipts,
            "source": public_receipt(self.source) if self.source else None,
            "package_policy_pending": sorted({rule["environment"] for rule in self.package_rules}),
            "package_policy_add_observed": self.package_add_observed,
            "package_policy_remove_pending": self.package_remove_pending,
            "workspaces": [public_receipt(row) for row in self.workspaces],
            "environments": [public_receipt(row) for row in self.environments]}

    def save(self):
        (self.root / "owned.json").write_text(json.dumps(self.report(), indent=2) + "\n")

    def failure_diagnostic(self):
        diagnostic = json.dumps(dict(self.report(), status="failed"), separators=(",", ":"))
        if len(diagnostic) > REPORT_LIMIT:
            return json.dumps({"requested_source": self.prefix, "status": "failed", "report_truncated": True})
        return diagnostic

    def observe(self, operation, requested_name, *args):
        records = self.import_receipts if operation == "import" else self.open_receipts
        record = {"requested_name": requested_name, "result": "unconfirmed", "receipt": {}}
        records.append(record)
        self.save()
        try:
            value = self.query(*args)
        except subprocess.CalledProcessError as error:
            record.update(result="failed", exit_code=error.returncode,
                          receipt=failed_receipt(error.stdout))
            self.save()
            raise
        except (ValueError, RecursionError):
            record["result"] = "invalid-json"
            self.save()
            raise
        record.update(result="returned", exit_code=0, receipt=public_receipt(value))
        self.save()  # Observed response is durable before it can establish ownership.
        return record["receipt"]

    def import_open(self, label, source):
        path = self.root / label
        path.mkdir(mode=0o700)
        name = self.prefix + "-" + label
        ref = self.observe("import", name, "workspace", "import", "--json", "--repo", self.source["id"],
                         "--name", name, "--oci", "none", "--path", str(path), str(source))
        require(ref.get("name") == name and ref.get("state") == "ready" and
                isinstance(ref.get("workspace"), str), "import receipt mismatch")
        self.workspaces.append(ref)
        self.save()
        opened = self.observe("open", name, "open", "--client", "none", "--json", str(path))
        env = opened.get("environment", {})
        require(opened.get("created") is True and opened.get("name") == name and
                opened.get("workspace") == ref["workspace"] and
                isinstance(env.get("workspace"), dict) and
                env.get("workspace", {}).get("id") == ref["workspace"] and
                env.get("name") == "work-" + name and bool(env.get("runtime_ref")) and
                bool(env.get("created_at")), "selected Workspace/Environment mismatch")
        self.environments.append(env)
        self.save()
        return path, env

    def check_reopen(self, path, expected):
        opened = self.observe("open", expected["name"], "open", "--client", "none", "--json", str(path))
        require(not opened["created"] and environment_identity(opened["environment"]) ==
                environment_identity(expected), "path reopen replaced or changed ownership")

    def check_inputs(self):
        for path, expected in self.inputs:
            require(tree_digest(path) == expected, "client input/common Git state changed")

    def apply_package_policy(self, snapshot):
        # Use the revision-bound public command. Never edit protected Policy
        # storage or replay an uncertain save over a concurrent change.
        path = self.root / ("package-policy-" + uuid.uuid4().hex + ".json")
        with os.fdopen(os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), "w") as output:
            json.dump(snapshot, output)
        saved = self.query("config", "--file", str(path), "--json")
        require(saved.get("policy") == snapshot["policy"] and
                isinstance(saved.get("revision"), str) and
                re.fullmatch(r"sha256:[a-f0-9]{64}", saved["revision"]),
                "package Policy save is unconfirmed")
        require(self.query("config", "--json") == saved, "package Policy changed after save")
        path.unlink()

    def remove_package_policy(self):
        if not self.package_rules:
            return
        snapshot = self.query("config", "--json")
        current = snapshot["policy"]["rules"]
        require(isinstance(current, list), "invalid package Policy snapshot")
        reasons = {rule["reason"] for rule in self.package_rules}
        require(all(rule in self.package_rules for rule in current
                    if isinstance(rule, dict) and rule.get("reason") in reasons),
                "refusing cleanup of changed package Policy")
        # Snapshot is not a completion fence for a timed-out configuration save.
        # A delayed add may still commit after an early read reports absence.
        # Retain both pending rules and Env names until this add is positively
        # observed, or leave them for expiry and disposable runner teardown.
        if not self.package_add_observed:
            require(all(current.count(rule) == 1 for rule in self.package_rules),
                    "package Policy add is still unconfirmed; retain Environment names")
            self.package_add_observed = True
        remaining = list(current)
        for rule in self.package_rules:
            require(remaining.count(rule) <= 1, "ambiguous package Policy ownership")
            if rule in remaining:
                remaining.remove(rule)
        if remaining != current:
            require(not self.package_remove_pending,
                    "package Policy removal is still unconfirmed; retain Environment names")
            snapshot["policy"]["rules"] = remaining
            self.package_remove_pending = True
            self.save()
            self.apply_package_policy(snapshot)
        self.package_rules = []
        self.package_add_observed = False
        self.package_remove_pending = False
        self.save()

    def prepare_git(self, environment):
        # Getting started explicitly permits Git-less Images. Exercise the same
        # ordinary package path as installed Windows acceptance, not a Base or
        # guest repair. Keep every later Git/data assertion unchanged.
        # These are administrator name scopes in the fresh single-runner fixture,
        # not instance-bound permissions or an atomic name-based exec contract.
        current = self.query("env", "status", "--json", environment["name"])["environment"]
        require(environment_identity(current) == environment_identity(environment),
                "refusing package setup in a replaced Environment")
        require(re.fullmatch(r"work-" + re.escape(self.prefix) + r"-(other|selected)", environment["name"]),
                "invalid package Policy Environment")
        expiry = (datetime.now(timezone.utc) + timedelta(minutes=15)).isoformat(timespec="seconds").replace("+00:00", "Z")
        reason = "Installed Workspace input Git prerequisite " + environment["name"]
        rules = [{"capability": "network.egress", "action": "connect", "resource": host,
                  "environment": environment["name"], "attributes": {"protocol": protocol, "port": port},
                  "decision": "allow", "reason": reason, "expires_at": expiry}
                 for host in ("archive.ubuntu.com", "security.ubuntu.com")
                 for protocol, port in (("http", "80"), ("https", "443"))]
        snapshot = self.query("config", "--json")
        existing = snapshot["policy"]["rules"]
        require(isinstance(existing, list) and all(not isinstance(rule, dict) or rule.get("reason") != reason
                                                 for rule in existing),
                "package Policy already exists")
        self.package_rules.extend(rules)
        self.package_add_observed = False
        self.save()  # Cleanup intent survives an ambiguous configuration save.
        snapshot["policy"]["rules"] = existing + rules
        try:
            self.apply_package_policy(snapshot)
            self.package_add_observed = True
            current = self.query("env", "status", "--json", environment["name"])["environment"]
            require(environment_identity(current) == environment_identity(environment),
                    "refusing package setup in a replaced Environment")
            self.haco("exec", environment["name"], "--", "sh", "-ceu", GIT_PREREQUISITE)
        finally:
            self.remove_package_policy()

    def check_stale_reference(self, path, other_workspace):
        reference = path / ".haco-workspace.json"
        original = reference.read_bytes()
        changed = json.loads(original)
        changed["workspace"] = other_workspace
        try:
            reference.write_text(json.dumps(changed))
            try:
                self.observe("open", changed["name"], "open", "--client", "none", "--json", str(path))
            except subprocess.CalledProcessError:
                return
            raise RuntimeError("mismatched Workspace owner was accepted")
        finally:
            reference.write_bytes(original)

    def cleanup(self):
        # A failed/unknown creation is never adopted just by its predictable name.
        # Confirmed Environments/Workspaces can be deleted after a failure.
        # The public repo delete command only unregisters the source from active
        # selection; its native data is retained for disposable runner teardown.
        errors = []
        try:
            self.remove_package_policy()
        except (RuntimeError, subprocess.SubprocessError, KeyError, ValueError) as error:
            # Do not free an Env name while its package grant is uncertain.
            # Leave exact owned resources for inspection/disposable teardown.
            raise RuntimeError("owned cleanup incomplete; inspect retained package Policy") from error
        for env in reversed(self.environments):
            try:
                current = self.query("env", "status", "--json", env["name"])["environment"]
                require(environment_identity(current) == environment_identity(env),
                        "refusing cleanup of a replaced Environment")
                self.haco("env", "delete", "--force", env["name"])
                require(all(row["name"] != env["name"] for row in
                            self.query("env", "list", "--json")), "Environment cleanup incomplete")
            except (RuntimeError, subprocess.SubprocessError, KeyError, ValueError) as error:
                errors.append(error)
        for ref in reversed(self.workspaces):
            try:
                rows = [row for row in self.query("workspace", "list", "--json")
                        if row["name"] == ref["name"]]
                require(len(rows) == 1 and rows[0]["workspace"]["id"] == ref["workspace"],
                        "refusing cleanup of a replaced Workspace")
                self.haco("workspace", "delete", "--yes", ref["name"])
                require(all(row["name"] != ref["name"] for row in
                            self.query("workspace", "list", "--json")), "Workspace cleanup incomplete")
            except (RuntimeError, subprocess.SubprocessError, KeyError, ValueError) as error:
                errors.append(error)
        if self.source:
            try:
                rows = [row["source"] for row in self.query("repo", "list", "--json")["sources"]
                        if row["source"]["id"] == self.source["id"]]
                require(len(rows) == 1 and rows[0]["owner"] == self.source["owner"],
                        "refusing unregistration of a replaced source")
                self.haco("repo", "delete", self.source["id"])
                require(all(row["source"]["id"] != self.source["id"] for row in
                            self.query("repo", "list", "--json")["sources"]),
                        "source remains in active selection after unregistration")
            except (RuntimeError, subprocess.SubprocessError, KeyError, ValueError) as error:
                errors.append(error)
        if errors:
            raise RuntimeError("owned cleanup incomplete; inspect retained fixture receipts") from errors[0]

    def exercise(self):
        # Registration canonicalizes HTTPS, SSH and scp forms, and can refresh
        # existing records. Require the fresh install's empty public inventory
        # instead of attempting a second, incomplete canonicalizer here. Hidden
        # excluded records are why this must not run on an existing installation.
        sources = self.query("repo", "list", "--json")["sources"]
        require(sources == [], "acceptance requires a fresh installation with no registered sources")
        main, linked, head = fixture(self.root)
        self.inputs = [(path, tree_digest(path)) for path in (main, linked)]
        source = self.query("repo", "add", "--json", self.prefix, UPSTREAM)
        # Observation is not ownership. A deduplicated/mismatched receipt stays
        # available for diagnosis but must never enter the cleanup-owned set.
        self.registration_receipt = public_receipt(source)
        self.save()
        require(self.registration_receipt.get("id") == self.prefix and
                self.registration_receipt.get("state") == "ready" and
                bool(self.registration_receipt.get("owner")), "source receipt mismatch")
        self.source = self.registration_receipt
        self.save()
        _, other = self.import_open("other", main)
        path, selected = self.import_open("selected", linked)
        self.prepare_git(other)
        self.prepare_git(selected)
        self.haco("exec", selected["name"], "--", "sh", "-ceu", r'''
cd /workspace
test "$(git rev-parse --show-toplevel)" = /workspace
test "$(git rev-parse HEAD)" = "$1"
test "$(git symbolic-ref HEAD)" = refs/heads/feature/input
test "$(git show :tracked)" = staged
test "$(cat tracked)" = dirty
test "$(cat selected)" = linked-only
test "$(cat extra)" = untracked
test ! -e other-only
test -d .git
for entry in commondir gitdir worktrees hooks; do test ! -e ".git/$entry"; done
test "$(git remote get-url origin)" = "haco://$2"
test -z "$(git config --local --get fixture.private || :)"
test ! -e "$3"; test ! -e "$4"
test ! -S /run/hacocoon/control.sock
test ! -S /var/lib/incus/unix.socket
git -c user.name=Fixture -c user.email=fixture@example.invalid commit -m imported-index
test "$(git show HEAD:tracked)" = staged
test "$(cat tracked)" = dirty
printf 'guest-only\n' > tracked
git add tracked extra
git -c user.name=Fixture -c user.email=fixture@example.invalid commit -m independent-edit
test -z "$(git status --porcelain)"
''', "--", head, self.source["id"], str(main), str(linked))
        commit = self.haco("exec", selected["name"], "--", "git", "-C", "/workspace", "rev-parse", "HEAD")
        require(commit != head, "guest commit did not advance imported HEAD")
        self.check_stale_reference(path, other["workspace"]["id"])
        self.check_reopen(path, selected)
        self.haco("env", "stop", selected["name"])
        require(self.query("env", "status", "--json", selected["name"])["state"] == "stopped",
                "Environment did not stop")
        self.check_reopen(path, selected)
        self.haco("exec", selected["name"], "--", "sh", "-ceu",
                  'cd /workspace; test "$(git rev-parse HEAD)" = "$1"; test "$(cat tracked)" = guest-only',
                  "--", commit)
        self.haco("exec", other["name"], "--", "sh", "-ceu", r'''
cd /workspace
test "$(git symbolic-ref HEAD)" = refs/heads/main
test "$(cat tracked)" = initial
test "$(cat other-only)" = main-only
test ! -e selected; test ! -e extra
''')
        self.check_inputs()


def main():
    require_disposable_runner(os.environ)
    require(os.geteuid() != 0, "installed input acceptance requires an ordinary user")
    root = Path(tempfile.mkdtemp(prefix="haco-installed-input-"))
    print(f"Installed input fixture: {root}", flush=True)
    acceptance = Acceptance(root)
    try:
        try:
            acceptance.exercise()
        finally:
            acceptance.cleanup()
        acceptance.check_inputs()
    except BaseException:
        print(f"FAIL: client input and ownership receipts retained at {root}", flush=True)
        # The disposable runner will remove /tmp. This bounded public-only line
        # survives in its existing Actions logs; local trees are never uploaded.
        print("INSTALLED_INPUT_FAILURE " + acceptance.failure_diagnostic(), flush=True)
        raise
    # Only client-created fixture files and references live here. Guest data was
    # independently managed and deleted above; no guest-controlled tree is walked.
    shutil.rmtree(root)
    print("PASS: installed linked-worktree selection, Git index/commit, independent data, reopen/resume, "
          "owned Environment/Workspace deletion and source unregistration; native source retained until runner teardown")


if __name__ == "__main__":
    main()
