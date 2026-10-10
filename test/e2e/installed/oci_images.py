"""Installed ordinary-user Host area copy and retained-image acceptance.

Only for the fresh disposable Ubuntu installer runner. All mutations use haco;
a fixed read-only root observer protects the Host recipe because its public CLI
has no recipe inventory or compare-and-clear. These are point-in-time checks,
not protection against concurrent administrators on a shared installation.
No guest packages, network grants, runtime injection or direct Incus operations.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
import uuid


# The installed Physical Host controller, not the haco-host container, owns this
# path: install/install.sh's HACO_ROOT and composition/app.go's HostService.Root.
# Do not generalize this into an arbitrary privileged file reader.
RECIPE_OBSERVER = r'''
import hashlib, json, os, stat, sys
expected = sys.argv[1]
if expected != "absent" and (len(expected) != 64 or any(c not in "0123456789abcdef" for c in expected)):
    raise SystemExit("invalid recipe observation")
fds = [os.open("/", os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)]
parents = []
try:
    for part in ("var", "lib", "hacocoon", "host-customization"):
        try:
            fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fds[-1])
        except FileNotFoundError:
            if part == "host-customization" and expected == "absent":
                print(json.dumps({"absent": True, "parents": parents}))
                raise SystemExit(0)
            raise
        fds.append(fd)
        info = os.fstat(fd)
        if info.st_uid != 0 or info.st_mode & 0o022 or (part in ("hacocoon", "host-customization") and info.st_mode & 0o077):
            raise SystemExit("unsafe recipe parent")
        parents.append([info.st_dev, info.st_ino])
    try:
        fd = os.open("recipe.sh", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=fds[-1])
    except FileNotFoundError:
        if expected != "absent":
            raise
        print(json.dumps({"absent": True, "parents": parents}))
        raise SystemExit(0)
    fds.append(fd)
    info = os.fstat(fd)
    if expected == "absent":
        raise SystemExit("existing Host recipe; refusing replacement")
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o077 or info.st_size > 1048576:
        raise SystemExit("unsafe recipe file")
    data = bytearray()
    while len(data) <= 1048576:
        chunk = os.read(fd, min(65536, 1048577 - len(data)))
        if not chunk:
            break
        data.extend(chunk)
    after = os.fstat(fd)
    named = os.stat("recipe.sh", dir_fd=fds[-2], follow_symlinks=False)
    identity = lambda s: [s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns]
    if len(data) > 1048576 or hashlib.sha256(data).hexdigest() != expected or identity(info) != identity(after) or identity(info) != identity(named):
        raise SystemExit("recipe changed")
    print(json.dumps({"parents": parents, "file": identity(info), "sha256": expected}))
finally:
    for fd in reversed(fds):
        os.close(fd)
'''


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def require_runner():
    require(os.environ.get("GITHUB_ACTIONS") == "true" and
            os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") == "github-hosted" and
            os.geteuid() != 0,
            "installed OCI acceptance requires the fresh ordinary-user GitHub-hosted installer journey")
    require(not any(os.environ.get(key) for key in ("HACO_ROOT", "HACO_CONTROL_SOCKET", "HACO_CLIENT_MODE")),
            "installed OCI acceptance requires the default Physical Host controller")


def command(*args, timeout=360, check=True):
    result = subprocess.run(args, stdin=subprocess.DEVNULL, capture_output=True,
                            text=True, timeout=timeout)
    if check and result.returncode:
        # No raw setup output or protected configuration goes into CI logs.
        raise RuntimeError(f"installed OCI command failed (exit {result.returncode})")
    return result


def recipe_observation(expected):
    return json.loads(command("sudo", "-n", "/usr/bin/python3", "-I", "-c",
                              RECIPE_OBSERVER, expected, timeout=15).stdout)


def ref(resource):
    return {key: resource[key] for key in ("id", "owner")}


def store_identity(resource):
    return {key: resource.get(key) for key in
            ("id", "owner", "kind", "native_ref", "created_at", "workspace_id", "source_only")}


def environment_identity(env):
    return {key: env.get(key) for key in
            ("name", "created_at", "runtime_ref", "workspace", "persistent_resource", "owned_workspace")}


def ready_store(row, *, host=False):
    require(isinstance(row, dict) and re.fullmatch(r"[a-f0-9]{32}", row.get("owner", "")) and
            bool(row.get("native_ref")) and bool(row.get("created_at")) and
            row.get("kind") == "oci-containerd" and row.get("state") == "ready" and
            bool(row.get("source_only")) == host and
            (row.get("id") == "oci-source:host" if host else
             re.fullmatch(r"oci:[a-z0-9][a-z0-9-]{0,39}", row.get("id", ""))) and
            not any(row.get(key) for key in ("copy_operation", "copy_completed", "copy_cleanup", "restore_source", "import_pending")) and
            not any(row.get("copy_source", {}).values()), "invalid or unfinished Store receipt")


def inventory(value, store, *, host=False):
    target = value.get("target", {})
    require(target.get("store") == ref(store) and target.get("runtime") == "nerdctl" and
            bool(target.get("host")) == host and bool(target.get("detached")) != host and
            target.get("environment") == "" and target.get("instance") == "",
            "image inventory target mismatch")
    images = value.get("images")
    require(isinstance(images, list), "missing image inventory")
    result = {}
    for row in images:
        image_id, tags = row.get("id", ""), row.get("tags")
        require(re.fullmatch(r"sha256:[a-f0-9]{64}", image_id) and image_id not in result and
                isinstance(tags, list) and all(isinstance(tag, str) for tag in tags) and
                len(tags) == len(set(tags)) and "containers" in row and row["containers"] in (None, []),
                "invalid, duplicated or referenced fixture image")
        result[image_id] = sorted(tags)
    return result


def host_recipe(prefix):
    require(re.fullmatch(r"oci-[a-f0-9]{16}", prefix), "invalid fixture prefix")
    return """set -eu
umask 077
context=$(mktemp -d /root/haco-oci-context.XXXXXXXX)
cleanup_context() {
  result=$?
  trap - EXIT
  cleanup_result=0
  rm -f -- "$context/Dockerfile" "$context/payload" || cleanup_result=1
  rmdir -- "$context" || cleanup_result=1
  if [ "$result" = 0 ]; then result=$cleanup_result; fi
  exit "$result"
}
trap cleanup_context EXIT
printf 'FROM scratch\\nCOPY payload /payload\\n' > "$context/Dockerfile"
""" + "".join(
        f"printf '%s\\n' '{prefix}-{label}' > \"$context/payload\"\n"
        f"nerdctl --namespace default --snapshotter native build --network none "
        f"-t example.invalid/{prefix}:{label} \"$context\"\n"
        for label in ("remove", "keep"))


class Acceptance:
    def __init__(self, root):
        self.root = root
        self.prefix = "oci-" + uuid.uuid4().hex[:16]
        self.environments, self.stores, self.workspaces = [], [], []
        self.attempts = []
        self.source = None
        self.images = {}
        self.recipe = None
        self.recipe_saved = False
        self.pending = None
        self.phase = "preflight"
        self.save()

    def save(self):
        # The private receipt precedes every subsequent fallible observation.
        data = {key: getattr(self, key) for key in
                ("prefix", "environments", "stores", "attempts", "source", "images", "recipe", "recipe_saved", "pending", "phase")}
        path = self.root / "owned.json"
        temporary = self.root / ".owned-next.json"
        with open(temporary, "x", opener=lambda name, flags: os.open(name, flags | os.O_NOFOLLOW, 0o600)) as output:
            json.dump(data, output, indent=2)
            output.flush()
            os.fsync(output.fileno())
        temporary.replace(path)
        directory = os.open(self.root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)

    def checkpoint(self, phase):
        self.phase = phase
        self.save()
        print("Installed OCI phase: " + phase, flush=True)

    def begin(self, operation):
        require(self.pending is None, "earlier mutation remains unconfirmed")
        self.pending = operation
        self.save()

    def complete(self):
        self.pending = None
        self.save()

    @staticmethod
    def haco(*args, **kwargs):
        return command("haco", *args, **kwargs)

    def query(self, *args):
        return json.loads(self.haco(*args).stdout)

    def current_store(self, store):
        response = self.query("plugin", "oci", "store", "list", "--json")
        rows = [row for row in response["resources"] if row["id"] == store["id"]]
        require(len(rows) == 1 and store_identity(rows[0]) == store_identity(store),
                "Store ownership changed")
        ready_store(rows[0], host=bool(store.get("source_only")))
        uses = [row for row in response["uses"] if row["resource"] == ref(store)]
        require(len(uses) == 1 and not uses[0]["pending_copies"] and
                not uses[0]["independent_snapshots"], "Store use is unknown or busy")
        return uses[0]

    def list_images(self, store):
        self.current_store(store)
        host = bool(store.get("source_only"))
        return inventory(self.query("plugin", "oci", "image", "list", "--json",
                                    *( ["--host"] if host else [store["id"]] )), store, host=host)

    def workspace(self, label):
        path = self.root / label
        path.mkdir(mode=0o700)
        info = path.lstat()
        self.workspaces.append((path, info.st_dev, info.st_ino))
        return path

    def create(self, label, path, *, no_oci=False, expected=None):
        name = self.prefix + "-" + label
        require(all(row["name"] != name for row in self.query("env", "list", "--json")),
                "fixture Environment already exists")
        before = self.query("plugin", "oci", "store", "list", "--json")["resources"]
        attempt = {"name": name, "workspace_path": str(path), "state": "unconfirmed"}
        self.attempts.append(attempt)
        self.save()
        self.begin("create_environment:" + name)
        env = self.query("env", "create", "--json", "--workspace", str(path),
                         *(["--no-oci"] if no_oci else []), name)
        attempt.update(state="returned", receipt=environment_identity(env))
        self.save()
        require(env.get("name") == name and env.get("created_at") and env.get("runtime_ref") and
                env.get("workspace", {}).get("path") == str(path) and env["workspace"].get("id") and
                not env.get("owned_workspace"), "Environment receipt mismatch")
        self.environments.append(environment_identity(env))
        self.save()
        if expected is not None:
            require(env["workspace"] == expected["workspace"] and
                    env.get("persistent_resource") == expected.get("persistent_resource"),
                    "recreation did not reuse the same Workspace and Store")
        if no_oci:
            require(not any(env.get("persistent_resource", {}).values()), "--no-oci attached a Store")
            require(self.query("plugin", "oci", "store", "list", "--json")["resources"] == before,
                    "--no-oci changed the Store catalog")
            attempt["state"] = "confirmed"
            self.complete()
            return env
        selected = env.get("persistent_resource", {})
        rows = [row for row in self.query("plugin", "oci", "store", "list", "--json")["resources"]
                if ref(row) == selected]
        require(len(rows) == 1, "automatic Store receipt missing")
        store = rows[0]
        ready_store(store)
        require(store.get("workspace_id") == env["workspace"]["id"] and
                store["owner"] != self.source["owner"] and store["native_ref"] != self.source["native_ref"],
                "automatic Store is not independently owned")
        if expected is None:
            require(all(row["id"] != store["id"] for row in before), "automatic Store was not new")
            self.stores.append(store)
            self.save()
        else:
            require(any(store_identity(row) == store_identity(store) for row in self.stores),
                    "retained Store identity changed")
        require(self.current_store(store)["environments"] == [name], "Store lease mismatch")
        attempt["state"] = "confirmed"
        self.complete()
        return env

    def remove_environment(self, env):
        current = self.query("env", "status", "--json", env["name"])["environment"]
        require(environment_identity(current) == environment_identity(env), "Environment ownership changed")
        self.begin("delete_environment:" + env["name"])
        self.haco("env", "delete", "--force", env["name"])
        require(all(row["name"] != env["name"] for row in self.query("env", "list", "--json")),
                "Environment deletion unconfirmed")
        self.environments.remove(environment_identity(env))
        self.complete()

    def delete_image(self, store, image_id, expected):
        require(image_id in expected and self.list_images(store) == expected,
                "reviewed image inventory changed")
        self.begin("delete_image:" + store["id"] + ":" + image_id)
        self.haco("plugin", "oci", "image", "delete", "--yes",
                  *(["--host"] if store.get("source_only") else [store["id"]]), image_id)
        after = {key: value for key, value in expected.items() if key != image_id}
        require(self.list_images(store) == after, "image deletion not independently confirmed")
        self.complete()
        return after

    def seed_host(self):
        recipe_observation("absent")
        script = host_recipe(self.prefix)
        path = self.root / "host-setup.sh"
        path.write_text(script)
        path.chmod(0o600)
        digest = hashlib.sha256(script.encode()).hexdigest()
        self.recipe_saved = True  # Ambiguous setup must not authorize a retry.
        self.save()
        self.begin("save_and_apply_host_recipe")
        self.haco("setup", "--script", str(path), timeout=990)
        self.recipe = recipe_observation(digest)
        self.complete()
        self.clear_recipe()
        observed = self.list_images(self.source)
        require(len(observed) == 2 and sorted(observed.values()) ==
                sorted([f"example.invalid/{self.prefix}:{label}"] for label in ("remove", "keep")),
                "Host did not build the two distinct fixture images")
        # An observation is not cleanup authority. Only the complete, one-tag-per-
        # image fixture set may become owned; unexpected or untagged images stay.
        self.images = observed
        self.save()

    def clear_recipe(self):
        require(self.recipe is not None and recipe_observation(self.recipe["sha256"]) == self.recipe,
                "Host recipe identity changed; refusing clear")
        self.begin("clear_host_recipe")
        self.haco("setup", "--clear-script", timeout=990)
        absent = recipe_observation("absent")
        require(absent.get("absent") is True and absent.get("parents") == self.recipe["parents"],
                "Host recipe clear unconfirmed")
        self.recipe_saved = False
        self.complete()

    def exercise(self):
        require(self.query("env", "list", "--json") == [], "fixture requires an empty Environment inventory")
        stores = self.query("plugin", "oci", "store", "list", "--json")["resources"]
        require(len(stores) == 1, "fixture requires only the installed Host source")
        ready_store(stores[0], host=True)
        require(not stores[0].get("workspace_id"), "Host source has a Workspace")
        self.source = stores[0]
        self.save()
        require(self.list_images(self.source) == {}, "fixture requires an empty Host image inventory")
        self.checkpoint("build_host_images")
        self.seed_host()
        source_images = dict(self.images)
        self.checkpoint("automatic_copy_and_detached_deletion")
        first_path = self.workspace("first")
        first = self.create("first", first_path)
        store = self.stores[0]
        self.remove_environment(first)
        require(self.current_store(store)["environments"] == [], "Store did not detach")
        require(self.list_images(store) == source_images, "automatic copy changed actual image identities")
        remove_id = next(key for key, tags in source_images.items() if tags == [f"example.invalid/{self.prefix}:remove"])
        retained = self.delete_image(store, remove_id, source_images)
        require(self.list_images(self.source) == source_images, "copy deletion changed Host inventory")
        self.checkpoint("retained_store_recreation")
        second = self.create("recreated", first_path, expected=first)
        self.remove_environment(second)
        require(self.list_images(store) == retained, "recreation replaced guest changes from Host")
        self.checkpoint("independent_workspace_copy")
        separate = self.create("independent", self.workspace("independent"))
        other = self.stores[1]
        require(ref(other) != ref(store) and other["native_ref"] != store["native_ref"] and
                separate["workspace"]["id"] != first["workspace"]["id"], "new Workspace reused retained data")
        self.remove_environment(separate)
        require(self.list_images(other) == source_images, "new Workspace did not copy current Host images")
        self.checkpoint("populated_host_optout_and_refusal")
        no_oci = self.create("no-oci", self.workspace("no-oci"), no_oci=True)
        refused = self.haco("plugin", "oci", "image", "list", "--json", no_oci["name"], check=False)
        require(refused.returncode == 1, "image access without an OCI Store was not refused")
        self.remove_environment(no_oci)
        before_envs = self.query("env", "list", "--json")
        before_stores = self.query("plugin", "oci", "store", "list", "--json")
        self.begin("refuse_contradictory_selection")
        refused = self.haco("env", "create", "--json", "--workspace", str(first_path), "--no-oci",
                            "--resource", store["id"], self.prefix + "-contradictory", check=False)
        require(refused.returncode == 1 and self.query("env", "list", "--json") == before_envs and
                self.query("plugin", "oci", "store", "list", "--json") == before_stores,
                "contradictory Store selection was not refused without mutation")
        self.complete()
        require(self.list_images(self.source) == source_images, "selection checks changed Host images")

    def cleanup(self):
        # Stop after any ambiguous/replaced identity. Never adopt a failed create,
        # retry deletion or erase a primary failure to make this fixture green.
        require(self.pending is None and all(row["state"] == "confirmed" for row in self.attempts),
                "unconfirmed creation retained; refusing cleanup")
        self.checkpoint("cleanup")
        for env in list(reversed(self.environments)):
            self.remove_environment(env)
        for store in list(reversed(self.stores)):
            require(self.current_store(store)["environments"] == [], "Store still attached")
            self.begin("delete_store:" + store["id"])
            self.haco("plugin", "oci", "store", "delete", "--yes", store["id"].removeprefix("oci:"))
            require(all(row["id"] != store["id"] for row in
                        self.query("plugin", "oci", "store", "list", "--json")["resources"]),
                    "Store deletion unconfirmed")
            self.stores.remove(store)
            self.complete()
        if self.recipe_saved:
            self.clear_recipe()
        if self.images:
            current = self.list_images(self.source)
            require(current == self.images, "Host fixture image identities changed; refusing cleanup")
            for image_id in list(self.images):
                self.images = self.delete_image(self.source, image_id, self.images)
                self.save()
        for path, device, inode in self.workspaces:
            info = path.lstat()
            require(stat.S_ISDIR(info.st_mode) and (info.st_dev, info.st_ino) == (device, inode),
                    "Workspace directory changed; refusing cleanup")
            path.rmdir()  # Never recursively delete guest-visible data.


def execute(acceptance):
    try:
        acceptance.exercise()
    except BaseException:
        try:
            acceptance.cleanup()
        except BaseException:
            print("Installed OCI cleanup refused or incomplete; exact private receipts retained", flush=True)
        raise
    acceptance.cleanup()


def main():
    require_runner()
    root = Path(tempfile.mkdtemp(prefix="haco-installed-oci-"))
    print(f"Installed OCI private receipts: {root}", flush=True)
    execute(Acceptance(root))
    print("PASS: installed automatic Host image copy, detached deletion, retained-Store recreation, "
          "independent Workspace copy and populated-Host --no-oci/refusal; guest offline execution not tested", flush=True)


if __name__ == "__main__":
    main()
