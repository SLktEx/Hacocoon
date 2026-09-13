#!/usr/bin/env python3
"""Run a Go test command once; missing/skipped required tests are not success."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

RESULT = re.compile(r"^--- (PASS|FAIL|SKIP): (Test[A-Za-z0-9_]+) \(")


def verify(expected, results, exit_code):
    return exit_code == 0 and bool(expected) and all(results.get(name) == ["PASS"] for name in expected)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--expect", required=True, help="comma-separated exact top-level test names")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    expected = args.expect.split(",")
    if not all(re.fullmatch(r"Test[A-Za-z0-9_]+", name) for name in expected) or len(set(expected)) != len(expected):
        parser.error("expected tests must be distinct exact names")
    command = args.command[1:] if args.command[:1] == ["--"] else args.command
    if not command:
        parser.error("test command required")
    # Go's own -timeout bounds native commands; the workflow bounds the wrapper.
    # stderr stays on stderr. Never retry a test command or reinterpret its error.
    process = subprocess.Popen(command, stdout=subprocess.PIPE, text=True, errors="replace")
    results = {}
    for line in process.stdout:
        sys.stdout.write(line)
        sys.stdout.flush()
        match = RESULT.match(line)
        if match:
            results.setdefault(match[2], []).append(match[1])
    exit_code = process.wait()
    if os.environ.get("GITHUB_ACTIONS") == "true":
        # Local fixed metadata only. Do not retain argv, output, or environment.
        receipt = {"schema": 1, "sha": os.environ.get("GITHUB_SHA"),
                   "run": os.environ.get("GITHUB_RUN_ID"), "job": os.environ.get("GITHUB_JOB"),
                   "attempt": os.environ.get("GITHUB_RUN_ATTEMPT"), "exit_code": exit_code,
                   "tests": {name: results.get(name, []) for name in expected}}
        path = Path(__file__).resolve().parent.parent / "ci-test-results.jsonl"
        with path.open("a", encoding="utf-8") as stream:
            stream.write(json.dumps(receipt, ensure_ascii=True) + "\n")
    if not verify(expected, results, exit_code):
        missing = [name for name in expected if results.get(name) != ["PASS"]]
        print("Required test execution unproven: " + ", ".join(missing) + f"; exit_code={exit_code}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
