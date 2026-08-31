#!/usr/bin/env python3
"""Serialize repository checkpoint bumps without auto-stealing stale locks."""

from __future__ import annotations

from datetime import datetime, timezone
import json
import os
from pathlib import Path
import subprocess
import sys
from typing import Sequence

LOCK_NAME = "haco-bump-milestone.lock"
FALLBACK_LOCK_NAME = ".haco-bump-milestone.lock"
OWNER_NAME = "owner.json"


class LockHeldError(RuntimeError):
    """Raised when another checkpoint bump already owns the worktree lock."""


def resolve_git_dir(root: Path) -> Path | None:
    marker = root / ".git"
    if marker.is_dir():
        return marker
    if not marker.is_file():
        return None

    try:
        text = marker.read_text(encoding="utf-8").strip()
    except OSError:
        return None
    prefix = "gitdir:"
    if not text.lower().startswith(prefix):
        return None
    value = text[len(prefix) :].strip()
    if not value:
        return None
    git_dir = Path(value)
    if not git_dir.is_absolute():
        git_dir = root / git_dir
    return git_dir.resolve()


def milestone_lock_path(root: Path) -> Path:
    root = root.resolve()
    git_dir = resolve_git_dir(root)
    if git_dir is not None:
        return git_dir / LOCK_NAME
    return root / FALLBACK_LOCK_NAME


def _owner_summary(lock_path: Path) -> str:
    owner_path = lock_path / OWNER_NAME
    try:
        owner = json.loads(owner_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return "owner metadata unavailable"

    fields = []
    for key in ("pid", "started", "root"):
        value = owner.get(key)
        if value not in (None, ""):
            fields.append(f"{key}={value}")
    return ", ".join(fields) if fields else "owner metadata unavailable"


def acquire_lock(root: Path) -> Path:
    root = root.resolve()
    lock_path = milestone_lock_path(root)
    try:
        lock_path.mkdir(mode=0o700)
    except FileExistsError as exc:
        owner = _owner_summary(lock_path)
        raise LockHeldError(
            "checkpoint bump already in progress; "
            f"lock={lock_path}; {owner}. "
            "If you have verified that no bump process is active, remove this "
            "stale lock directory manually and retry."
        ) from exc
    except OSError as exc:
        raise RuntimeError(f"create checkpoint bump lock {lock_path}: {exc}") from exc

    owner = {
        "pid": os.getpid(),
        "started": datetime.now(timezone.utc).isoformat(),
        "root": str(root),
    }
    try:
        (lock_path / OWNER_NAME).write_text(
            json.dumps(owner, sort_keys=True) + "\n", encoding="utf-8"
        )
    except OSError:
        try:
            lock_path.rmdir()
        except OSError:
            pass
        raise
    return lock_path


def release_lock(lock_path: Path) -> None:
    owner_path = lock_path / OWNER_NAME
    try:
        owner_path.unlink()
    except FileNotFoundError:
        pass
    lock_path.rmdir()


def run_locked(root: Path, command: Sequence[str]) -> int:
    if not command:
        raise ValueError("a command is required")
    lock_path = acquire_lock(root)
    try:
        try:
            completed = subprocess.run(list(command), cwd=root, check=False)
        except OSError as exc:
            print(f"bump-milestone: failed to start locked command: {exc}", file=sys.stderr)
            return 1
        return completed.returncode
    finally:
        try:
            release_lock(lock_path)
        except OSError as exc:
            print(
                "bump-milestone: warning: checkpoint bump lock cleanup failed; "
                f"remove {lock_path} manually after verifying no bump is active: {exc}",
                file=sys.stderr,
            )


def main(argv: Sequence[str]) -> int:
    try:
        separator = argv.index("--")
    except ValueError:
        print(
            "usage: milestone_lock.py -- <command> [args...]",
            file=sys.stderr,
        )
        return 2
    command = argv[separator + 1 :]
    if not command:
        print("milestone_lock.py: command is required after --", file=sys.stderr)
        return 2

    root = Path.cwd().resolve()
    try:
        return run_locked(root, command)
    except LockHeldError as exc:
        print(f"bump-milestone: {exc}", file=sys.stderr)
        return 1
    except (OSError, RuntimeError, ValueError) as exc:
        print(f"bump-milestone: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
