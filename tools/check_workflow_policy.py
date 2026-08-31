#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

FULL_SHA_RE = re.compile(r"^[0-9a-fA-F]{40}$")
ACTIVE_TRUE = r'["\']?(?:1|true|yes|on)["\']?'
PRIVILEGED_ENV_RE = re.compile(
    rf"\b(HACO_E2E_INCUS|HACO_EXPERIMENTAL_EC2)\s*[:=]\s*{ACTIVE_TRUE}\b",
    re.IGNORECASE,
)
BLOCK_SCALARS = {"|", ">", "|-", ">-", "|+", ">+"}


@dataclass(frozen=True)
class Violation:
    path: Path
    line: int
    message: str

    def render(self) -> str:
        return f"ERROR {self.path}:{self.line}: {self.message}"


@dataclass
class YAMLNode:
    line: int
    indent: int
    key: str | None
    value: Any = None
    list_item: bool = False
    parent: YAMLNode | None = field(default=None, repr=False)
    children: list[YAMLNode] = field(default_factory=list)


class YAMLStructureError(ValueError):
    def __init__(self, line: int, message: str) -> None:
        super().__init__(message)
        self.line = line
        self.message = message


def _strip_comment(text: str) -> str:
    quote: str | None = None
    index = 0
    while index < len(text):
        char = text[index]
        if quote == "'":
            if char == "'":
                if index + 1 < len(text) and text[index + 1] == "'":
                    index += 2
                    continue
                quote = None
            index += 1
            continue
        if quote == '"':
            if char == "\\":
                index += 2
                continue
            if char == '"':
                quote = None
            index += 1
            continue
        if char in {"'", '"'}:
            quote = char
            index += 1
            continue
        if char == "#" and (index == 0 or text[index - 1].isspace()):
            return text[:index].rstrip()
        index += 1
    return text.rstrip()


def _split_flow_items(text: str) -> list[str]:
    parts: list[str] = []
    start = 0
    quote: str | None = None
    depth = 0
    index = 0
    while index < len(text):
        char = text[index]
        if quote == "'":
            if char == "'":
                if index + 1 < len(text) and text[index + 1] == "'":
                    index += 2
                    continue
                quote = None
            index += 1
            continue
        if quote == '"':
            if char == "\\":
                index += 2
                continue
            if char == '"':
                quote = None
            index += 1
            continue
        if char in {"'", '"'}:
            quote = char
        elif char in "[{(":
            depth += 1
        elif char in "]})":
            depth -= 1
            if depth < 0:
                raise ValueError("unbalanced flow collection")
        elif char == "," and depth == 0:
            parts.append(text[start:index].strip())
            start = index + 1
        index += 1
    if quote is not None or depth != 0:
        raise ValueError("unterminated quoted or flow value")
    parts.append(text[start:].strip())
    return parts


def _split_key_value(text: str) -> tuple[str, str] | None:
    quote: str | None = None
    depth = 0
    index = 0
    while index < len(text):
        char = text[index]
        if quote == "'":
            if char == "'":
                if index + 1 < len(text) and text[index + 1] == "'":
                    index += 2
                    continue
                quote = None
            index += 1
            continue
        if quote == '"':
            if char == "\\":
                index += 2
                continue
            if char == '"':
                quote = None
            index += 1
            continue
        if char in {"'", '"'}:
            quote = char
        elif char in "[{(":
            depth += 1
        elif char in "]})":
            depth -= 1
        elif char == ":" and depth == 0:
            if index + 1 == len(text) or text[index + 1].isspace():
                return text[:index].strip(), text[index + 1 :].strip()
        index += 1
    return None


def _unquote(value: str) -> str:
    if len(value) >= 2 and value[0] == value[-1] == "'":
        return value[1:-1].replace("''", "'")
    if len(value) >= 2 and value[0] == value[-1] == '"':
        try:
            decoded = json.loads(value)
        except json.JSONDecodeError as exc:
            raise ValueError("invalid double-quoted scalar") from exc
        if not isinstance(decoded, str):
            raise ValueError("quoted YAML key/scalar must decode to a string")
        return decoded
    return value


def _parse_value(value: str) -> Any:
    value = value.strip()
    if value == "":
        return None
    if value in BLOCK_SCALARS:
        return value
    if value.startswith("["):
        if not value.endswith("]"):
            raise ValueError("unterminated flow sequence")
        inner = value[1:-1].strip()
        if not inner:
            return []
        return [_parse_value(item) for item in _split_flow_items(inner)]
    if value.startswith("{"):
        if not value.endswith("}"):
            raise ValueError("unterminated flow mapping")
        inner = value[1:-1].strip()
        if not inner:
            return {}
        result: dict[str, Any] = {}
        for item in _split_flow_items(inner):
            pair = _split_key_value(item)
            if pair is None:
                raise ValueError("flow mapping item must contain an explicit ': '")
            raw_key, raw_value = pair
            key = _unquote(raw_key)
            if not key:
                raise ValueError("flow mapping key must not be empty")
            if key in result:
                raise ValueError(f"duplicate flow mapping key {key!r}")
            result[key] = _parse_value(raw_value)
        return result
    if (value.startswith("'") and value.endswith("'")) or (
        value.startswith('"') and value.endswith('"')
    ):
        return _unquote(value)
    lowered = value.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    if lowered in {"null", "~"}:
        return None
    return value


def _parse_tree(text: str) -> YAMLNode:
    root = YAMLNode(line=0, indent=-1, key=None)
    stack = [root]
    block_parent_indent: int | None = None

    for line_number, raw in enumerate(text.splitlines(), 1):
        leading = raw[: len(raw) - len(raw.lstrip(" \t"))]
        if "\t" in leading:
            raise YAMLStructureError(line_number, "tabs are not supported for YAML indentation")
        indent = len(raw) - len(raw.lstrip(" "))

        if block_parent_indent is not None:
            if raw.strip() == "" or indent > block_parent_indent:
                continue
            block_parent_indent = None

        stripped = _strip_comment(raw[indent:])
        if not stripped.strip():
            continue
        content = stripped.strip()
        list_item = False
        if content == "-":
            list_item = True
            content = ""
        elif content.startswith("- "):
            list_item = True
            content = content[2:].lstrip()

        pair = _split_key_value(content) if content else None
        try:
            if pair is not None:
                raw_key, raw_value = pair
                key = _unquote(raw_key)
                if not key:
                    raise ValueError("mapping key must not be empty")
                value = _parse_value(raw_value)
            elif list_item:
                key = None
                value = _parse_value(content)
            else:
                raise ValueError("expected mapping entry or sequence item")
        except ValueError as exc:
            raise YAMLStructureError(line_number, str(exc)) from exc

        while len(stack) > 1 and indent <= stack[-1].indent:
            stack.pop()
        parent = stack[-1]
        node = YAMLNode(
            line=line_number,
            indent=indent,
            key=key,
            value=value,
            list_item=list_item,
            parent=parent,
        )
        parent.children.append(node)
        stack.append(node)
        if isinstance(value, str) and value in BLOCK_SCALARS:
            block_parent_indent = indent

    return root


def _walk(node: YAMLNode) -> Iterator[YAMLNode]:
    for child in node.children:
        yield child
        yield from _walk(child)


def _materialize(node: YAMLNode) -> Any:
    if node.value is not None:
        return node.value
    if not node.children:
        return None
    if all(child.list_item and child.key is None for child in node.children):
        return [_materialize(child) for child in node.children]

    result: dict[str, Any] = {}
    for child in node.children:
        if child.list_item and child.key is None:
            raise ValueError("mixed mapping and sequence structure")
        if child.key is None:
            raise ValueError("mapping child is missing a key")
        if child.key in result:
            raise ValueError(f"duplicate key {child.key!r}")
        result[child.key] = _materialize(child)
    return result


def _root_nodes(root: YAMLNode, key: str) -> list[YAMLNode]:
    return [node for node in root.children if not node.list_item and node.key == key]


def _trigger_names(node: YAMLNode) -> set[str]:
    value = _materialize(node)
    if value is None:
        return set()
    if isinstance(value, str):
        return {value}
    if isinstance(value, list):
        if not all(isinstance(item, str) for item in value):
            raise ValueError("workflow trigger sequence must contain only strings")
        return set(value)
    if isinstance(value, dict):
        return set(value)
    raise ValueError("unsupported workflow trigger structure")


def _trigger_line(on_nodes: list[YAMLNode], trigger: str) -> int:
    for node in on_nodes:
        if isinstance(node.value, list) and trigger in node.value:
            return node.line
        if isinstance(node.value, dict) and trigger in node.value:
            return node.line
        if node.value == trigger:
            return node.line
        for child in node.children:
            if child.key == trigger or (child.key is None and child.value == trigger):
                return child.line
    return on_nodes[0].line if on_nodes else 1


def _nearest_list_item(node: YAMLNode) -> YAMLNode | None:
    current: YAMLNode | None = node
    while current is not None and current.parent is not None:
        if current.list_item:
            return current
        current = current.parent
    return None


def _has_ancestor_key(node: YAMLNode, key: str) -> bool:
    current = node.parent
    while current is not None:
        if current.key == key:
            return True
        current = current.parent
    return False


def _first_descendant(node: YAMLNode, key: str) -> YAMLNode | None:
    return next((candidate for candidate in _walk(node) if candidate.key == key), None)


def _explicit_false(value: Any) -> bool:
    return value is False or (isinstance(value, str) and value.lower() == "false")


def _is_alias_anchor_or_tag(value: Any) -> bool:
    return isinstance(value, str) and value.startswith(("&", "*", "!"))


def _check_duplicate_keys(path: Path, root: YAMLNode, violations: list[Violation]) -> None:
    for parent in [root, *_walk(root)]:
        seen: set[str] = set()
        for child in parent.children:
            if child.list_item or child.key is None:
                continue
            if child.key in seen:
                violations.append(
                    Violation(path, child.line, f"duplicate YAML key {child.key!r} is not permitted")
                )
            seen.add(child.key)


def _check_permissions(path: Path, root: YAMLNode, violations: list[Violation]) -> None:
    for node in _walk(root):
        if node.key != "permissions":
            continue
        try:
            value = _materialize(node)
        except ValueError as exc:
            violations.append(Violation(path, node.line, f"PR workflow permissions are ambiguous: {exc}"))
            continue
        if value is None:
            continue
        if isinstance(value, str):
            if value != "read-all":
                violations.append(
                    Violation(path, node.line, f"PR workflow permissions must be read-only, got {value!r}")
                )
            continue
        if isinstance(value, dict):
            for scope, permission in value.items():
                if not (isinstance(permission, str) and permission in {"read", "none"}):
                    violations.append(
                        Violation(
                            path,
                            node.line,
                            f"PR workflow permission {scope!r} must be read/none, got {permission!r}",
                        )
                    )
            continue
        violations.append(
            Violation(path, node.line, f"PR workflow permissions must be read-only, got {value!r}")
        )


def _check_runner(path: Path, node: YAMLNode, violations: list[Violation]) -> None:
    try:
        value = _materialize(node)
    except ValueError as exc:
        violations.append(Violation(path, node.line, f"runner declaration is ambiguous: {exc}"))
        return

    if isinstance(value, str):
        if value == "self-hosted" or "${{" in value:
            violations.append(
                Violation(
                    path,
                    node.line,
                    f"runner {value!r} is not approved; self-hosted or dynamic runners are not permitted",
                )
            )
        return

    if isinstance(value, list):
        if not all(isinstance(item, str) for item in value):
            violations.append(Violation(path, node.line, "runner declaration is ambiguous"))
            return
        if "self-hosted" in value or any("${{" in item for item in value):
            violations.append(
                Violation(
                    path,
                    node.line,
                    f"runner {value!r} is not approved; self-hosted or dynamic runners are not permitted",
                )
            )
        return

    violations.append(Violation(path, node.line, f"runner declaration is ambiguous: {value!r}"))


def check_text(path: Path, text: str) -> list[Violation]:
    violations: list[Violation] = []
    try:
        root = _parse_tree(text)
    except YAMLStructureError as exc:
        return [
            Violation(
                path,
                exc.line,
                f"workflow YAML structure is not supported by security parser: {exc.message}",
            )
        ]

    _check_duplicate_keys(path, root, violations)

    for node in _walk(root):
        if node.key == "<<":
            violations.append(Violation(path, node.line, "YAML merge keys are not permitted"))
        if node.key != "run" and _is_alias_anchor_or_tag(node.value):
            violations.append(
                Violation(
                    path,
                    node.line,
                    "YAML anchors, aliases, and tags are not permitted in workflow security structure",
                )
            )

    on_nodes = _root_nodes(root, "on")
    triggers: set[str] = set()
    for node in on_nodes:
        try:
            triggers.update(_trigger_names(node))
        except ValueError as exc:
            violations.append(
                Violation(path, node.line, f"workflow triggers must use an explicit supported structure: {exc}")
            )

    is_pr = "pull_request" in triggers
    for forbidden in ("pull_request_target", "workflow_run"):
        if forbidden in triggers:
            violations.append(
                Violation(path, _trigger_line(on_nodes, forbidden), f"forbidden workflow trigger: {forbidden}")
            )

    for node in _walk(root):
        if node.key == "runs-on":
            _check_runner(path, node, violations)

    if is_pr:
        _check_permissions(path, root, violations)

    for node in _walk(root):
        if node.key != "uses":
            continue
        target = node.value
        if not isinstance(target, str):
            violations.append(Violation(path, node.line, "uses must be an explicit string"))
            continue

        if not _has_ancestor_key(node, "steps"):
            violations.append(
                Violation(path, node.line, "reusable workflow jobs are not permitted by workflow trust policy")
            )

        if target.startswith("./"):
            continue
        if "@" not in target:
            violations.append(Violation(path, node.line, f"external action must use an immutable SHA: {target}"))
            continue
        action, ref = target.rsplit("@", 1)
        if not FULL_SHA_RE.fullmatch(ref):
            violations.append(
                Violation(path, node.line, f"external action {action} must be pinned to a full commit SHA")
            )

        if not is_pr:
            continue
        step = _nearest_list_item(node)
        if step is None:
            continue
        with_node = _first_descendant(step, "with")
        with_map: dict[str, Any] = {}
        if with_node is not None:
            try:
                with_value = _materialize(with_node)
            except ValueError as exc:
                violations.append(
                    Violation(path, with_node.line, f"action with: configuration is ambiguous: {exc}")
                )
            else:
                if isinstance(with_value, dict):
                    with_map = with_value
                else:
                    violations.append(
                        Violation(path, with_node.line, "action with: configuration must be an explicit mapping")
                    )

        if action == "actions/checkout" and not _explicit_false(with_map.get("persist-credentials")):
            violations.append(
                Violation(path, node.line, "actions/checkout in PR workflow requires persist-credentials: false")
            )
        if action == "actions/cache":
            violations.append(Violation(path, node.line, "actions/cache is not permitted in untrusted PR workflows"))
        if action == "actions/setup-go" and not _explicit_false(with_map.get("cache")):
            violations.append(
                Violation(path, node.line, "actions/setup-go in PR workflow requires cache: false")
            )
        if action == "actions/download-artifact" and any(
            key in with_map for key in ("run-id", "github-token", "repository")
        ):
            violations.append(
                Violation(path, node.line, "cross-run/external artifact download is not permitted in PR workflows")
            )

    if is_pr:
        secret_match = re.search(r"\$\{\{\s*secrets\.", text)
        if secret_match:
            line = text[: secret_match.start()].count("\n") + 1
            violations.append(Violation(path, line, "repository/environment secrets are not permitted in PR workflows"))
        for match in PRIVILEGED_ENV_RE.finditer(text):
            line = text[: match.start()].count("\n") + 1
            violations.append(Violation(path, line, f"{match.group(1)} must remain disabled in normal PR CI"))

    return violations


def check_workflow_file(path: Path) -> list[Violation]:
    return check_text(path, path.read_text(encoding="utf-8"))


def discover(root: Path) -> list[Path]:
    return sorted([*root.glob("*.yml"), *root.glob("*.yaml")])


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate Hacocoon GitHub Actions trust boundaries.")
    parser.add_argument(
        "workflow_dir",
        nargs="?",
        type=Path,
        default=Path(".github/workflows"),
        help="workflow directory (default: .github/workflows)",
    )
    args = parser.parse_args(argv)

    files = discover(args.workflow_dir)
    if not files:
        print(f"ERROR {args.workflow_dir}: no workflow files found", file=sys.stderr)
        return 2

    violations = [violation for path in files for violation in check_workflow_file(path)]
    if violations:
        for violation in violations:
            print(violation.render(), file=sys.stderr)
        return 1

    print(f"workflow trust policy: OK ({len(files)} files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
