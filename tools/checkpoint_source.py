#!/usr/bin/env python3
"""Parser for Hacocoon's deliberately constrained checkpoint YAML source."""

from dataclasses import dataclass
import json
from pathlib import Path
import re

_CHECKPOINT_RE = re.compile(r"^v0\.(\d+)$")
_CURRENT_RE = re.compile(r'^current: ("v0\.\d+")$')
_MILESTONE_RE = re.compile(r'^  "(v0\.\d+)": ("(?:[^"\\]|\\.)*")$')


class CheckpointSourceError(ValueError):
    pass


@dataclass(frozen=True)
class Milestone:
    version: str
    gate: str
    minor: int


@dataclass(frozen=True)
class CheckpointSource:
    current: str
    milestones: tuple[Milestone, ...]


def _parse_checkpoint(value: str) -> int:
    match = _CHECKPOINT_RE.fullmatch(value)
    if not match:
        raise CheckpointSourceError(f"checkpoint must match canonical v0.N: {value!r}")
    minor = int(match.group(1))
    if minor < 1 or value != f"v0.{minor}":
        raise CheckpointSourceError(f"checkpoint must use canonical numbering from v0.1: {value!r}")
    return minor


def parse_checkpoint_source_text(text: str) -> CheckpointSource:
    seen_schema = False
    seen_current = False
    seen_milestones = False
    current = None
    milestones = []
    seen_versions = set()

    for line_number, line in enumerate(text.splitlines(), start=1):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if line == "schema: 1":
            if seen_schema:
                raise CheckpointSourceError(f"line {line_number}: duplicate schema")
            seen_schema = True
            continue
        current_match = _CURRENT_RE.fullmatch(line)
        if current_match:
            if seen_current:
                raise CheckpointSourceError(f"line {line_number}: duplicate current")
            current = json.loads(current_match.group(1))
            seen_current = True
            continue
        if line == "milestones:":
            if seen_milestones:
                raise CheckpointSourceError(f"line {line_number}: duplicate milestones mapping")
            seen_milestones = True
            continue
        milestone_match = _MILESTONE_RE.fullmatch(line)
        if milestone_match:
            if not seen_milestones:
                raise CheckpointSourceError(f"line {line_number}: milestone appears before milestones mapping")
            version = milestone_match.group(1)
            if version in seen_versions:
                raise CheckpointSourceError(f"line {line_number}: duplicate milestone {version}")
            minor = _parse_checkpoint(version)
            gate = json.loads(milestone_match.group(2)).strip()
            if not gate or any(character in gate for character in "\r\n|"):
                raise CheckpointSourceError(f"line {line_number}: invalid gate name")
            seen_versions.add(version)
            milestones.append(Milestone(version=version, gate=gate, minor=minor))
            continue
        raise CheckpointSourceError(f"line {line_number}: unsupported YAML syntax {line!r}")

    if not seen_schema:
        raise CheckpointSourceError("schema: 1 is required")
    if not seen_current or current is None:
        raise CheckpointSourceError("current is required")
    if not seen_milestones or not milestones:
        raise CheckpointSourceError("at least one milestone is required")

    for index, milestone in enumerate(milestones, start=1):
        if milestone.minor != index or milestone.version != f"v0.{index}":
            raise CheckpointSourceError(
                f"milestones must be contiguous from v0.1; position {index} is {milestone.version}"
            )
    latest = milestones[-1].version
    if current != latest:
        raise CheckpointSourceError(f"current {current} must equal newest milestone {latest}")

    return CheckpointSource(current=current, milestones=tuple(milestones))


def parse_checkpoint_source(path: Path) -> CheckpointSource:
    try:
        text = path.read_text()
    except OSError as exc:
        raise CheckpointSourceError(f"read {path}: {exc}") from exc
    return parse_checkpoint_source_text(text)
