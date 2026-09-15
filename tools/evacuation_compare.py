"""Read-only Linux tree manifests and portable restored-data comparison."""
import argparse
import hashlib
import json
import os
import stat
import sys
import time

from evacuation_files import file_inventory, mount_id, open_directory

MAX_ENTRIES = 50000
MAX_MANIFEST = 64 * 1024**2
FIELDS = ("kind", "mode", "uid", "gid", "content", "xattrs", "hardlink")


def _stamp(value):
    return (value.st_dev, value.st_ino, value.st_mode, value.st_uid, value.st_gid,
            value.st_nlink, value.st_size, value.st_mtime_ns, value.st_ctime_ns)


def _matches(value, row):
    return (value.st_dev == row["device"] and value.st_ino == row["inode"] and
            oct(stat.S_IMODE(value.st_mode)) == row["mode"] and
            all(getattr(value, "st_" + key) == row[key] for key in ("uid", "gid", "mtime_ns", "ctime_ns")) and
            value.st_size == row["bytes"] and value.st_nlink == row["links"])


def _parent(root, relative, expected_mount):
    """Walk under the held root, never via a later replacement of its pathname."""
    fd = os.dup(root)
    try:
        for part in relative.split("/")[:-1]:
            child = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=fd)
            os.close(fd)
            fd = child
            if mount_id(fd) != expected_mount:
                raise ValueError("mount changed")
        return fd
    except BaseException:
        os.close(fd)
        raise


def _xattrs(fd):
    # Names and values can contain private data; retain only a framed digest.
    digest = hashlib.sha256()
    for name in sorted(os.listxattr(fd)):
        key, value = os.fsencode(name), os.getxattr(fd, name)
        for field in (key, value):
            digest.update(len(field).to_bytes(8, "big"))
            digest.update(field)
    return digest.hexdigest()


def scan_tree(root, *, byte_limit=64 * 1024**3, seconds=900):
    if type(byte_limit) is not int or byte_limit < 1 or not 0 < seconds <= 86400:
        raise ValueError("positive scan budgets required")
    deadline = time.monotonic() + seconds
    report = {"format": 1, "complete": False, "entries": {}, "errors": [],
              "quiescence_required": True, "backup_complete": False,
              "unreviewed": ["independent retention and destination ownership",
                             "application consistency and authenticated development",
                             "timestamps, symlink ACLs/xattrs and external hardlink targets"]}
    total = 0
    root_fd = None
    def gap(path, reason):
        report["errors"].append({"path": path, "reason": reason})
    try:
        root_fd = open_directory(root)
        root_identity = _stamp(os.fstat(root_fd))
        root_mount = mount_id(root_fd)
        before = file_inventory(root, entry_limit=MAX_ENTRIES, seconds=min(seconds, 60))
        if before["errors"] or any(x["reason"] != "symlink-not-followed" for x in before["deferred"]):
            gap(".", "inventory-incomplete")
            return report
        rows = sorted(before["entries"], key=lambda x: x["path"])
        root_row = next((row for row in rows if row["path"] == "."), None)
        if root_row is None or not _matches(os.fstat(root_fd), root_row):
            gap(".", "root-changed")
            return report
        groups = {}
        for row in rows:
            path = row["path"]
            if time.monotonic() >= deadline:
                gap(path, "scan-budget-exhausted")
                break
            parent = fd = None
            try:
                if path == ".":
                    fd = os.dup(root_fd)
                else:
                    parent = _parent(root_fd, path, root_mount)
                    leaf = path.rsplit("/", 1)[-1]
                    if row["kind"] == "symlink":
                        value = os.stat(leaf, dir_fd=parent, follow_symlinks=False)
                        if not stat.S_ISLNK(value.st_mode) or not _matches(value, row):
                            raise ValueError("link changed")
                        target = os.fsencode(os.readlink(leaf, dir_fd=parent))
                        if _stamp(os.stat(leaf, dir_fd=parent, follow_symlinks=False)) != _stamp(value):
                            raise ValueError("link changed")
                        report["entries"][path] = {k: row[k] for k in ("kind", "mode", "uid", "gid")}
                        report["entries"][path].update(content=hashlib.sha256(target).hexdigest(), xattrs=None, hardlink=None)
                        continue
                    flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC
                    if row["kind"] == "directory":
                        flags |= os.O_DIRECTORY
                    fd = os.open(leaf, flags, dir_fd=parent)
                value = os.fstat(fd)
                if not _matches(value, row) or mount_id(fd) != root_mount:
                    raise ValueError("entry changed")
                if not stat.S_ISREG(value.st_mode) and not stat.S_ISDIR(value.st_mode):
                    raise ValueError("unsupported entry")
                record = {k: row[k] for k in ("kind", "mode", "uid", "gid")}
                record.update(content=None, xattrs=_xattrs(fd), hardlink=None)
                if row["kind"] == "file":
                    digest = hashlib.sha256()
                    while True:
                        if time.monotonic() >= deadline:
                            raise ValueError("scan budget")
                        block = os.read(fd, min(1024 * 1024, byte_limit - total + 1))
                        if not block:
                            break
                        total += len(block)
                        if total > byte_limit:
                            raise ValueError("scan budget")
                        digest.update(block)
                    record["content"] = digest.hexdigest()
                    identity = row["device"], row["inode"]
                    if row["links"] > 1:
                        record["hardlink"] = groups.setdefault(identity, path)
                if _stamp(os.fstat(fd)) != _stamp(value):
                    raise ValueError("entry changed")
                report["entries"][path] = record
            except (OSError, ValueError):
                gap(path, "entry-unreadable-changing-or-budget-exhausted")
            finally:
                if fd is not None:
                    os.close(fd)
                if parent is not None:
                    os.close(parent)
            if total > byte_limit:
                break
        after = file_inventory(root, entry_limit=MAX_ENTRIES, seconds=min(max(deadline-time.monotonic(), 0), 60))
        if after["errors"] or after["deferred"] != before["deferred"] or sorted(after["entries"], key=lambda x: x["path"]) != rows:
            gap(".", "tree-changed-or-final-inventory-incomplete")
        if _stamp(os.fstat(root_fd)) != root_identity:
            gap(".", "held-root-changed")
    except (OSError, ValueError):
        gap(".", "root-or-metadata-unavailable")
    finally:
        if root_fd is not None:
            os.close(root_fd)
    # Only links within this selected tree can be compared across filesystems.
    members = {}
    for path, row in report["entries"].items():
        if row["hardlink"] is not None:
            members.setdefault(row["hardlink"], []).append(path)
    for paths in members.values():
        if len(paths) == 1:
            report["entries"][paths[0]]["hardlink"] = None
    report["complete"] = not report["errors"]
    report["hashed_bytes"] = total
    return report


def read_manifest(path):
    with open(path, "rb") as source:
        raw = source.read(MAX_MANIFEST + 1)
    if len(raw) > MAX_MANIFEST:
        raise ValueError("manifest too large")
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise ValueError("duplicate field")
            value[key] = item
        return value
    result = json.loads(raw, object_pairs_hook=unique)
    if not isinstance(result, dict) or type(result.get("format")) is not int or result["format"] != 1 or type(result.get("complete")) is not bool:
        raise ValueError("invalid manifest")
    entries = result.get("entries")
    if not isinstance(result.get("errors"), list) or result["complete"] and result["errors"]:
        raise ValueError("inconsistent completion")
    if not isinstance(entries, dict) or not 0 < len(entries) <= MAX_ENTRIES or "." not in entries:
        raise ValueError("invalid entries")
    for path, row in entries.items():
        if not isinstance(path, str) or (path != "." and (path.startswith("/") or any(x in ("", ".", "..") for x in path.split("/")))):
            raise ValueError("invalid path")
        if not isinstance(row, dict) or set(row) != set(FIELDS) or row["kind"] not in ("file", "directory", "symlink"):
            raise ValueError("invalid entry")
        if any(type(row[x]) is not int or row[x] < 0 for x in ("uid", "gid")):
            raise ValueError("invalid owner")
        if not isinstance(row["mode"], str) or not row["mode"].startswith("0o") or not 0 <= int(row["mode"], 8) <= 0o7777:
            raise ValueError("invalid mode")
        for field in ("content", "xattrs"):
            value = row[field]
            if value is not None and (not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value)):
                raise ValueError("invalid digest")
        if (row["content"] is None) != (row["kind"] == "directory") or (row["xattrs"] is None) != (row["kind"] == "symlink"):
            raise ValueError("missing digest")
        if row["hardlink"] is not None and (row["kind"] != "file" or not isinstance(row["hardlink"], str) or row["hardlink"] not in entries or not isinstance(entries[row["hardlink"]], dict) or entries[row["hardlink"]].get("kind") != "file"):
            raise ValueError("invalid hardlink group")
    return result


def compare_manifests(source, restored):
    differences = []
    left, right = source["entries"], restored["entries"]
    for path in sorted(left.keys() | right.keys()):
        changed = ["missing-from-restored"] if path not in right else ["added-in-restored"] if path not in left else [x for x in FIELDS if left[path][x] != right[path][x]]
        if changed:
            differences.append({"path": path, "fields": changed})
    complete = source["complete"] and restored["complete"]
    return {"format": 1, "comparison_complete": complete, "matching": complete and not differences,
            "differences": differences, "backup_complete": False,
            "next": "Review differences and incomplete scans. Preserve original data; this report never authorizes deletion.",
            "unreviewed": ["numeric owner namespace correspondence", "independent destination and retention",
                           "application consistency, editor/build/OCI and authenticated Git",
                           "timestamps, symlink ACLs/xattrs, external hardlinks and filesystem flags"]}


def main(argv=None):
    parser = argparse.ArgumentParser(description="Compare reviewed restored trees; never restores, repairs or deletes data.")
    sub = parser.add_subparsers(dest="action", required=True)
    scan = sub.add_parser("scan", help="create a private Linux tree manifest")
    scan.add_argument("root")
    scan.add_argument("--quiesced", action="store_true", required=True)
    scan.add_argument("--byte-limit", type=int, default=64 * 1024**3)
    scan.add_argument("--seconds", type=float, default=900)
    compare = sub.add_parser("compare", help="compare two saved manifests on any platform")
    compare.add_argument("source")
    compare.add_argument("restored")
    args = parser.parse_args(argv)
    try:
        result = scan_tree(args.root, byte_limit=args.byte_limit, seconds=args.seconds) if args.action == "scan" else compare_manifests(read_manifest(args.source), read_manifest(args.restored))
    except (OSError, ValueError, TypeError, KeyError, RecursionError):
        print("Comparison unavailable; retain both trees and inspect the manifests. No data changed.", file=sys.stderr)
        return 2
    json.dump(result, sys.stdout, sort_keys=True)
    print()
    return 0 if result.get("matching", result.get("complete", False)) else 1


if __name__ == "__main__":
    sys.exit(main())
