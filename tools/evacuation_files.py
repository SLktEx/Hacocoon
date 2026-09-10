"""Bounded Linux directory metadata observation; never a content backup."""
import os
import re
import stat
import time


def mountpoints():
    with open("/proc/self/mountinfo", "rb") as source:
        raw = source.read(2 * 1024 * 1024 + 1)
    if len(raw) > 2 * 1024 * 1024:
        raise ValueError("mount table oversized")
    result = set()
    for line in os.fsdecode(raw).splitlines():
        fields = line.split(" ")
        if len(fields) < 10 or " - " not in line:
            raise ValueError("invalid mount table")
        path = re.sub(r"\\([0-7]{3})", lambda m: chr(int(m[1], 8)), fields[4])
        if not path.startswith("/"):
            raise ValueError("invalid mount path")
        result.add(path)
    return result


def mount_id(fd):
    with open("/proc/self/fdinfo/" + str(fd), "r", encoding="ascii") as source:
        raw = source.read(4097)
    values = re.findall(r"^mnt_id:\s*([0-9]+)$", raw, re.M)
    if len(raw) > 4096 or len(values) != 1:
        raise ValueError("mount identity unavailable")
    return int(values[0])


def open_directory(path):
    """Pin the explicitly selected root without following any symlink component."""
    if not hasattr(os, "O_NOFOLLOW") or not path.startswith("/") or os.path.normpath(path) != path:
        raise ValueError("canonical absolute Linux directory required")
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
    fd = os.open("/", flags)
    try:
        for component in path.split("/")[1:]:
            if not component:
                continue
            child = os.open(component, flags, dir_fd=fd)
            os.close(fd)
            fd = child
        return fd
    except BaseException:
        os.close(fd)
        raise


def file_inventory(root, entry_limit=50000, depth_limit=64, seconds=60):
    report = {"root": root, "enumeration_complete": False, "backup_complete": False,
              "entries": [], "deferred": [], "errors": [],
              "unreviewed": ["all file contents, ACLs and xattrs; none captured",
                             "manual-data classification, capture and restore comparison",
                             "concurrent file changes; this is not an atomic snapshot"]}
    def gap(path, reason):
        report["errors"].append({"path": path, "reason": reason})
    try:
        mounts = mountpoints()
        fd = open_directory(root)
    except (OSError, ValueError):
        gap(".", "root-or-mount-table-unavailable")
        return report
    deadline = time.monotonic() + seconds
    try:
        root_info = os.fstat(fd)
        root_mount = mount_id(fd)
    except (OSError, ValueError):
        os.close(fd)
        gap(".", "root-mount-identity-unavailable")
        return report
    exhausted = False
    def identity(value):
        return value.st_dev, value.st_ino
    def record(path, value):
        mode = value.st_mode
        kind = "directory" if stat.S_ISDIR(mode) else "file" if stat.S_ISREG(mode) else "symlink" if stat.S_ISLNK(mode) else "special"
        report["entries"].append({"path": path, "kind": kind, "mode": oct(stat.S_IMODE(mode)),
                                  "uid": value.st_uid, "gid": value.st_gid, "bytes": value.st_size,
                                  "device": value.st_dev, "inode": value.st_ino, "links": value.st_nlink,
                                  "mtime_ns": value.st_mtime_ns})
        return kind
    record(".", root_info)
    def walk(directory, relative, depth):
        nonlocal exhausted
        before = os.fstat(directory)
        try:
            with os.scandir(directory) as entries:
                for entry in entries:
                    if exhausted:
                        break
                    if len(report["entries"]) >= entry_limit or time.monotonic() >= deadline:
                        gap(relative, "enumeration-budget-exhausted")
                        exhausted = True
                        break
                    path = entry.name if relative == "." else relative + "/" + entry.name
                    try:
                        value = entry.stat(follow_symlinks=False)
                        kind = record(path, value)
                        absolute = os.path.join(root, path)
                        if absolute in mounts or value.st_dev != root_info.st_dev:
                            report["deferred"].append({"path": path, "reason": "separate-mount-or-filesystem"})
                        elif kind == "directory":
                            if depth >= depth_limit:
                                gap(path, "depth-limit")
                                continue
                            child = os.open(entry.name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC, dir_fd=directory)
                            try:
                                if identity(os.fstat(child)) != identity(value):
                                    gap(path, "directory-replaced")
                                elif mount_id(child) != root_mount:
                                    report["deferred"].append({"path": path, "reason": "separate-mount-or-filesystem"})
                                else:
                                    walk(child, path, depth + 1)
                            finally:
                                os.close(child)
                        elif kind in ("symlink", "special"):
                            report["deferred"].append({"path": path, "reason": kind + "-not-followed"})
                    except (OSError, ValueError):
                        gap(path, "entry-unavailable-or-changing")
        except OSError:
            gap(relative, "directory-unreadable")
        after = os.fstat(directory)
        if (before.st_mtime_ns, before.st_ctime_ns) != (after.st_mtime_ns, after.st_ctime_ns):
            gap(relative, "directory-changed-during-enumeration")
    try:
        walk(fd, ".", 0)
        current = open_directory(root)
        try:
            if identity(os.fstat(current)) != identity(root_info):
                gap(".", "root-replaced")
        finally:
            os.close(current)
        if mountpoints() != mounts:
            gap(".", "mount-table-changed")
    except (OSError, ValueError):
        gap(".", "root-or-mount-table-changed")
    finally:
        os.close(fd)
    report["enumeration_complete"] = not report["errors"] and not report["deferred"]
    return report
