"""Ready catalog fixtures for repository-only E2E; no provider resource is created."""
import datetime
import json
import os
from pathlib import Path
import sys
import tempfile
import uuid

path, name, workspace = sys.argv[1:]
path = Path(path)
path.parent.mkdir(parents=True, exist_ok=True)
stamp = datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z")
state = json.loads(path.read_text()) if path.exists() else {
    "version": 4, "environments": {}, "workspace_leases": {}
}
work_id = "path:" + workspace
runtime = "test:" + name
state["environments"][name] = {
    "name": name, "workspace": {"id": work_id, "path": workspace},
    "access_mode": "rw", "runtime_ref": runtime, "created_at": stamp,
}
state["workspace_leases"][name] = {
    "instance_id": "env-" + uuid.uuid4().hex,
    "environment_id": name, "workspace_id": work_id, "source_path": workspace,
    "access_mode": "rw", "owner": name, "runtime_ref": runtime,
    "state": "active", "acquired_at": stamp,
}
fd, temporary = tempfile.mkstemp(dir=path.parent)
try:
    with os.fdopen(fd, "w") as handle:
        json.dump(state, handle)
    os.replace(temporary, path)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
