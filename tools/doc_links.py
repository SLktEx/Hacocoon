"""Local Markdown destinations and GitHub-style heading anchors.

Stdlib-only so documentation checks need no renderer or downloaded dependency.
Code examples are not links. Both inline/reference Markdown and HTML href/src
destinations are checked; remote URLs are outside this offline check.
"""
from html import unescape
from pathlib import Path
import re
import unicodedata
from urllib.parse import unquote, urlsplit


def prose(text):
    """Blank code/comments while retaining line offsets for diagnostics."""
    output = []
    fence = None
    for line in text.splitlines(keepends=True):
        marker = re.match(r"^ {0,3}(`{3,}|~{3,})", line)
        if marker:
            run = marker[1]
            if fence is None:
                fence = run
            elif run[0] == fence[0] and len(run) >= len(fence):
                fence = None
            output.append("\n" if line.endswith("\n") else "")
        elif fence is not None or line.startswith(("    ", "\t")):
            output.append("\n" if line.endswith("\n") else "")
        else:
            output.append(line)
    return re.sub(r"<!--.*?-->", lambda m: "\n" * m[0].count("\n"),
                  "".join(output), flags=re.S)


def heading_anchors(text):
    body = prose(text)
    anchors = set(re.findall(r'<[^>]+\b(?:id|name)=["\']([^"\']+)["\']', body))
    used = set()
    lines = body.splitlines()
    for index, line in enumerate(lines):
        match = re.match(r"^ {0,3}#{1,6}\s+(.+?)(?:\s+#+\s*)?$", line)
        heading = match[1] if match else None
        if heading is None and index + 1 < len(lines) and line.strip():
            if re.fullmatch(r" {0,3}(?:=+|-+)\s*", lines[index + 1]):
                heading = line.strip()
        if heading is None:
            continue
        heading = re.sub(r"!?(\[([^\]]+)\])\([^)]*\)", r"\2", heading)
        heading = unescape(re.sub(r"<[^>]*>", "", heading)).lower()
        slug = "".join(
            c for c in heading
            if c in "-_" or not unicodedata.category(c).startswith(("P", "S"))
        ).replace(" ", "-")
        candidate, suffix = slug, 0
        while candidate in used:
            suffix += 1
            candidate = f"{slug}-{suffix}"
        used.add(candidate)
        anchors.add(candidate)
    return anchors


def destinations(text):
    body = prose(text)
    # Inline code spans cannot contain operative Markdown links.
    body = re.sub(r"(`+).*?\1", lambda m: " " * len(m[0]), body)
    references = {}
    definitions = re.compile(
        r'^ {0,3}\[([^\]]+)\]:\s*(<[^>]+>|\S+)(?:\s+.*)?$', re.M
    )
    for match in definitions.finditer(body):
        references[" ".join(match[1].lower().split())] = match[2].strip("<>")
        yield match.start(), match[2].strip("<>")
    for match in re.finditer(r"!?\[([^\]\n]*(?:\][^\[\]\n]+)*?)\]\(", body):
        start = match.end()
        pos, depth = start, 0
        angle = body[start:start + 1] == "<"
        if angle:
            end = body.find(">", start + 1)
            if end != -1:
                yield match.start(), body[start + 1:end]
            continue
        while pos < len(body):
            char = body[pos]
            if char == "\\":
                pos += 2
                continue
            if char == "(":
                depth += 1
            elif char == ")":
                if depth == 0:
                    break
                depth -= 1
            elif char.isspace() and depth == 0:
                break
            pos += 1
        yield match.start(), body[start:pos]
    for match in re.finditer(r"!?\[([^\]\n]+)\](?:\[([^\]\n]*)\])?", body):
        if body[match.end():match.end() + 1] in ("(", ":"):
            continue
        label = match[2] if match[2] else match[1]
        key = " ".join(label.lower().split())
        if key in references:
            yield match.start(), references[key]
    for match in re.finditer(r'<[^>]+\b(?:href|src)=["\']([^"\']+)["\']', body):
        yield match.start(), match[1]


def check_links(root, files):
    root = root.resolve()
    errors, cache = [], {}
    for path in files:
        text = path.read_text(encoding="utf-8")
        for offset, raw in destinations(text):
            raw = unescape(re.sub(r"\\([() ])", r"\1", raw))
            if not raw:
                continue
            url = urlsplit(raw)
            if url.scheme or url.netloc:
                continue
            target = unquote(url.path)
            resolved = ((root / target.lstrip("/")) if target.startswith("/")
                        else (path.parent / target) if target else path).resolve()
            label = f"{path.relative_to(root)}:{prose(text)[:offset].count(chr(10)) + 1}"
            try:
                resolved.relative_to(root)
            except ValueError:
                errors.append(f"{label}: link escapes repository: {raw}")
                continue
            if not resolved.exists():
                errors.append(f"{label}: broken local link: {raw}")
                continue
            fragment = unquote(url.fragment)
            if fragment and resolved.suffix.lower() == ".md":
                if resolved not in cache:
                    cache[resolved] = heading_anchors(resolved.read_text(encoding="utf-8"))
                if fragment not in cache[resolved]:
                    errors.append(f"{label}: broken heading anchor: {raw}")
    return errors
