"""Explicit Linux tree capture using GNU tar and age; no restore or authority."""
import argparse
import hashlib
import json
import os
import re
import selectors
import stat
import subprocess
import sys
import time

from evacuation_files import file_inventory, open_directory


class CaptureError(RuntimeError):
    pass


def _tree(root):
    report = file_inventory(root)
    if report["errors"] or any(x["reason"] != "symlink-not-followed" for x in report["deferred"]):
        raise CaptureError("source tree requires separate review")
    return sorted(report["entries"], key=lambda x: x["path"])


def _identity(fd):
    value = os.fstat(fd)
    return value.st_dev, value.st_ino


def _file_stamp(value):
    return (value.st_dev, value.st_ino, value.st_mode, value.st_uid, value.st_nlink,
            value.st_size, value.st_mtime_ns, value.st_ctime_ns)


def _same_directory(path, expected):
    current = open_directory(path)
    try:
        if _identity(current) != expected:
            raise CaptureError("directory identity changed")
    finally:
        os.close(current)


def _create(directory, name):
    fd = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=directory)
    return os.fdopen(fd, "wb")


def _receipt(directory, name, value):
    with _create(directory, name) as output:
        output.write((json.dumps(value, sort_keys=True) + "\n").encode())
        output.flush()
        os.fsync(output.fileno())
    os.fsync(directory)


def _stop(process):
    if process is not None and process.poll() is None:
        process.terminate()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait(timeout=5)


def capture_tree(source, destination, recipient, *, byte_limit=64 * 1024**3, seconds=900):
    """Caller must quiesce writers and separately arrange external retention.

    Destination must be a new, empty, private Linux directory. Completion proves
    a successful encrypted stream and unchanged observed metadata, not an atomic
    snapshot or a complete installation backup. Failures retain partial artifacts.
    """
    if not re.fullmatch(r"age1[0-9a-z]{58}", recipient):
        raise ValueError("an age public recipient is required")
    if type(byte_limit) is not int or byte_limit <= 0 or not 0 < seconds <= 86400:
        raise ValueError("positive bounded capture budgets required")
    if destination == source or destination.startswith(source.rstrip("/") + "/"):
        raise ValueError("destination must be outside source")
    source_fd = open_directory(source)
    destination_fd = None
    producer = consumer = None
    try:
        destination_fd = open_directory(destination)
        info = os.fstat(destination_fd)
        if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) & 0o077 or os.listdir(destination_fd):
            raise CaptureError("destination must be empty and private")
        source_identity, destination_identity = _identity(source_fd), _identity(destination_fd)
        before = _tree(source)
        if (before[0]["device"], before[0]["inode"]) != source_identity:
            raise CaptureError("source identity changed")
        if any(row["kind"] == "directory" and (row["device"], row["inode"]) == destination_identity for row in before):
            raise CaptureError("destination resolves inside source")
        intent = {"version": 1, "source": source, "source_identity": source_identity,
                  "destination_identity": destination_identity, "recipient": recipient,
                  "archive": "data.tar.age", "backup_complete": False}
        _receipt(destination_fd, "capture-intent.json", intent)
        env = {"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C"}
        deadline = time.monotonic() + seconds
        def remaining():
            value = deadline - time.monotonic()
            if value <= 0:
                raise CaptureError("capture deadline exceeded")
            return value
        digest = hashlib.sha256()
        size = 0
        with _create(destination_fd, "data.tar.age") as output:
            producer = subprocess.Popen(["tar", "--one-file-system", "--acls", "--xattrs", "--numeric-owner", "--sparse", "-cpf", "-", "-C", "/proc/self/fd/" + str(source_fd), "."],
                                        pass_fds=(source_fd,), stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=env)
            consumer = subprocess.Popen(["age", "-r", recipient], stdin=producer.stdout,
                                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, env=env)
            producer.stdout.close()
            with selectors.DefaultSelector() as selector:
                selector.register(consumer.stdout, selectors.EVENT_READ)
                while True:
                    if not selector.select(remaining()):
                        raise CaptureError("capture deadline exceeded")
                    block = os.read(consumer.stdout.fileno(), 65536)
                    if not block:
                        break
                    size += len(block)
                    if size > byte_limit:
                        raise CaptureError("encrypted archive exceeds byte limit")
                    output.write(block)
                    digest.update(block)
            if consumer.wait(timeout=remaining()) != 0:
                raise CaptureError("age encryption failed")
            if producer.wait(timeout=remaining()) != 0:
                raise CaptureError("tar capture failed")
            output.flush()
            os.fsync(output.fileno())
            output_stamp = _file_stamp(os.fstat(output.fileno()))
        if _tree(source) != before:
            raise CaptureError("source metadata changed during capture")
        _same_directory(source, source_identity)
        _same_directory(destination, destination_identity)
        if _file_stamp(os.stat("data.tar.age", dir_fd=destination_fd, follow_symlinks=False)) != output_stamp:
            raise CaptureError("encrypted output changed")
        complete = {**intent, "archive_complete": True, "bytes": size,
                    "sha256": digest.hexdigest(), "consistency_requires_quiescence": True,
                    "external_retention_verified": False}
        _receipt(destination_fd, "capture-complete.json", complete)
        return complete
    finally:
        try:
            _stop(consumer)
        finally:
            try:
                _stop(producer)
            finally:
                if consumer is not None and consumer.stdout is not None:
                    consumer.stdout.close()
                if producer is not None and producer.stdout is not None:
                    producer.stdout.close()
                os.close(source_fd)
                if destination_fd is not None:
                    os.close(destination_fd)


def main(argv=None):
    parser = argparse.ArgumentParser(description="Encrypt one reviewed Linux tree; does not produce a whole-installation backup.")
    parser.add_argument("source", help="absolute path to a quiescent source directory")
    parser.add_argument("destination", help="new empty private directory outside source")
    parser.add_argument("recipient", help="age public recipient; never supply a private key")
    parser.add_argument("--quiesced", action="store_true", required=True,
                        help="confirm all writers to this tree have been stopped")
    parser.add_argument("--byte-limit", type=int, default=64 * 1024**3)
    parser.add_argument("--seconds", type=float, default=900)
    args = parser.parse_args(argv)
    try:
        result = capture_tree(args.source, args.destination, args.recipient,
                              byte_limit=args.byte_limit, seconds=args.seconds)
    except (CaptureError, OSError, ValueError, subprocess.TimeoutExpired):
        # Paths and subprocess diagnostics may contain private data.
        print("Capture failed; retain and inspect the destination. No backup completion is claimed.", file=sys.stderr)
        return 1
    json.dump(result, sys.stdout, sort_keys=True)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
