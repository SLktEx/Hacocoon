"""Compare already-read references. No provider calls, data access or authority."""
import re
import base64
import binascii

LIMIT = 4096


def native_reference(ref):
    """Read the existing router envelope; never select another provider."""
    prefix = "haco-runtime-v1:runtime.incus:"
    if ref.startswith("haco-runtime-v1:"):
        if not ref.startswith(prefix):
            return None
        encoded = ref[len(prefix):]
        if not re.fullmatch(r"[A-Za-z0-9_-]{1,4096}", encoded):
            return None
        try:
            raw = base64.b64decode(encoded + "=" * (-len(encoded) % 4), altchars=b"-_", validate=True)
            if base64.urlsafe_b64encode(raw).decode().rstrip("=") != encoded:
                return None
            ref = raw.decode("ascii")
        except (ValueError, UnicodeError, binascii.Error):
            return None
    parts = ref.split("/")
    if any(not re.fullmatch(r"[A-Za-z0-9_.-]+", part) or part in (".", "..") for part in parts):
        return None
    return parts


def compare_associations(native, catalog=None, repositories=None):
    result = {"authority": False, "review_required": True, "rows": [], "errors": []}
    index = {}
    for project in native.get("projects", []):
        for item in project.get("instances", []):
            key = ("instance", item["name"])
            index.setdefault(key, []).append({"project": project["name"], "name": item["name"], "owner_marker": item.get("owner_marker")})
        for item in project.get("volumes", []):
            if item.get("type") != "custom":
                continue
            key = ("volume", item["pool"], item["name"])
            index.setdefault(key, []).append({"project": project["name"], "pool": item["pool"], "name": item["name"], "owner_marker": item.get("owner_marker")})

    def records():
        if catalog:
            if not catalog.get("projection_complete"):
                result["errors"].append("catalog-projection-incomplete")
            for row in catalog.get("records", []):
                source = {"section": row["section"], "key": row["key"], "state": row.get("state")}
                if row["section"] == "snapshots":
                    if not row.get("components"):
                        yield source, row, "unreviewed"
                    for component in row.get("components", []):
                        yield {**source, "role": component["role"]}, component, "snapshot"
                elif row["section"] == "persistent_resources":
                    yield source, row, "volume"
                else:
                    yield source, row, "unreviewed"
        if repositories:
            if not repositories.get("projection_complete"):
                result["errors"].append("repository-projection-incomplete")
            for file in repositories.get("files", []):
                for row in file.get("records", []):
                    yield {"file": file["file"], "id": row["id"]}, row, "volume"

    for source, item, kind in records():
        if len(result["rows"]) >= LIMIT:
            result["errors"].append("comparison-budget-exhausted")
            break
        ref = item.get("native_ref", "")
        parts = native_reference(ref) or []
        key = None
        if kind == "volume" and len(parts) == 2 and all(parts):
            key = ("volume", *parts)
        elif kind == "snapshot" and ((len(parts) == 2 and parts[0] == "instance") or (len(parts) == 3 and parts[0] == "volume")) and all(parts):
            key = tuple(parts)
        candidates = index.get(key, []) if key else []
        owner = item.get("owner", "")
        if key is None:
            status = "unsupported-reference"
        elif not native.get("native_queries_complete"):
            status = "incomplete-native-inventory"
        elif len(candidates) > 1:
            status = "ambiguous"
        elif not candidates:
            status = "not-observed"
        elif not isinstance(owner, str) or not re.fullmatch(r"[a-f0-9]{32}", owner):
            status = "catalog-owner-unavailable"
        elif candidates[0]["owner_marker"] is None:
            status = "native-owner-unavailable"
        elif candidates[0]["owner_marker"] != owner:
            status = "owner-mismatch"
        else:
            status = "reference-and-marker-observed"
        result["rows"].append({"source": source, "native_ref": ref, "status": status, "recorded_state": item.get("state"), "candidates": [dict(c) for c in candidates]})
    return result
