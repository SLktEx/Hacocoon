#!/usr/bin/env python3
"""Read-only Actions evidence: a rerun never erases a failed attempt.

Only metadata is read. No remote logs, artifacts, commands or credentials are
copied to the report. API errors and incomplete pagination fail closed.
"""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

BAD = {"failure", "timed_out", "action_required", "startup_failure"}
CLASSES = {"product", "fixture", "infrastructure"}


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise ValueError("Actions metadata unexpectedly redirected")


class Actions:
    def __init__(self, repository, token):
        if not re.fullmatch(r"[A-Za-z0-9_-][A-Za-z0-9_.-]*/[A-Za-z0-9_-][A-Za-z0-9_.-]*", repository):
            raise ValueError("invalid repository")
        self.base = f"https://api.github.com/repos/{repository}/actions/"
        self.token = token

    def get(self, path):
        if not re.fullmatch(r"[A-Za-z0-9_./?=&%-]+", path) or ".." in path:
            raise ValueError("invalid Actions metadata path")
        request = urllib.request.Request(self.base + path, headers={
            "Accept": "application/vnd.github+json",
            "Authorization": "Bearer " + self.token,
            "X-GitHub-Api-Version": "2022-11-28",
        })
        # No retry: a metadata outage is an infrastructure failure, not permission
        # to forget earlier failures. Never include the response body in errors.
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
            raw = response.read(8 * 1024 * 1024 + 1)
        if len(raw) > 8 * 1024 * 1024:
            raise ValueError("Actions metadata exceeds limit")
        return json.loads(raw)

    def pages(self, path, key):
        rows = []
        separator = "&" if "?" in path else "?"
        for page in range(1, 101):
            data = self.get(f"{path}{separator}per_page=100&page={page}")
            batch = data[key]
            if not isinstance(batch, list):
                raise ValueError("invalid metadata collection")
            rows.extend(batch)
            if len(batch) < 100:
                if len(rows) < data.get("total_count", len(rows)):
                    raise ValueError("incomplete Actions metadata")
                return rows
        raise ValueError("Actions pagination limit reached")


def failure_boundary(step):
    match = re.match(r"^\[(product|fixture|infrastructure)\] ", step)
    return match.group(1) if match else "unclassified"


def summarize(runs, jobs_for_attempt, required_steps=None):
    """Keep attempts distinct, including successful jobs from a partial rerun."""
    records = []
    for run in runs:
        for attempt in range(1, int(run["run_attempt"]) + 1):
            for job in jobs_for_attempt(run["id"], attempt):
                if job["name"].endswith("-evidence"):
                    continue
                failures = [s for s in job.get("steps", []) if s.get("conclusion") in BAD]
                job_key = job["name"].split(" (")[0]
                expected = (required_steps or {}).get(job_key, [])
                unproven = [name for name in expected if
                            [s.get("conclusion") for s in job.get("steps", []) if s["name"] == name] != ["success"]]
                records.append({
                    "workflow": run["name"], "workflow_id": run["workflow_id"],
                    "sha": run["head_sha"], "event": run["event"],
                    "run_id": run["id"], "attempt": attempt,
                    "job": job["name"], "job_id": job["id"],
                    "conclusion": job.get("conclusion"),
                    "failed_steps": [{"step": s["name"], "boundary": failure_boundary(s["name"])} for s in failures],
                    "runner_labels": job.get("labels", []),
                    "red_then_green": False,
                    "unproven_steps": unproven,
                })
    for row in records:
        if row["conclusion"] not in BAD:
            continue
        row["red_then_green"] = any(
            (other["workflow_id"], other["sha"], other["event"], other["job"]) ==
            (row["workflow_id"], row["sha"], row["event"], row["job"])
            and (other["run_id"], other["attempt"]) > (row["run_id"], row["attempt"])
            and other["conclusion"] == "success" for other in records
        )
    return records


def check_needs(needs, required):
    if set(needs) != set(required):
        raise ValueError("required job set differs from needs evidence")
    return all(value.get("result") == "success" for value in needs.values())


def missing_job_variants(records, run_id, expected):
    if not expected or len(set(expected)) != len(expected):
        raise ValueError("required job variants are empty or duplicated")
    passed = {r["job"] for r in records if r["run_id"] == run_id and r["conclusion"] == "success"}
    return sorted(set(expected) - passed)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--gate", action="store_true")
    parser.add_argument("--recent", type=int, default=30, help="most recent runs to inspect (1..100)")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args(argv)
    if not 1 <= args.recent <= 100:
        parser.error("--recent must be 1..100")
    api = Actions(os.environ["GITHUB_REPOSITORY"], os.environ["GITHUB_TOKEN"])
    current = None
    required_steps = {}
    needs_ok = True
    if args.gate:
        current = api.get("runs/" + str(int(os.environ["GITHUB_RUN_ID"])))
        contracts = json.loads(Path(__file__).with_name("ci_contracts.json").read_text())
        required = contracts[current["name"]]["jobs"]
        required_steps = contracts[current["name"]]["steps"]
        needs_ok = check_needs(json.loads(os.environ["CI_NEEDS"]), required)
        # Query all runs for this workflow and source SHA, not only the latest
        # attempt of this run. Reopening a PR must not reset failure history.
        path = f"workflows/{int(current['workflow_id'])}/runs?head_sha={current['head_sha']}"
        runs = [r for r in api.pages(path, "workflow_runs") if r["event"] == current["event"]]
        if not any(r["id"] == current["id"] for r in runs):
            raise ValueError("current run missing from Actions history")
    else:
        runs = api.get(f"runs?per_page={args.recent}")["workflow_runs"]
    records = summarize(runs, lambda run, attempt: api.pages(f"runs/{run}/attempts/{attempt}/jobs", "jobs"), required_steps)
    history_failed = any(r["conclusion"] in BAD for r in records)
    missing_variants = missing_job_variants(records, current["id"], contracts[current["name"]]["job_names"]) if args.gate else []
    # Inspect actual Actions step conclusions even when the job itself is green.
    # Previous failed/skipped attempts are retained; only successful jobs assert
    # that they ran their contract. A cancelled attempt is not labelled a flake.
    steps_unproven = any(r["conclusion"] == "success" and r["unproven_steps"] for r in records)
    report = {"schema": 1, "checked_sha": os.environ.get("GITHUB_SHA"),
              "needs_success": needs_ok, "unresolved_failure": history_failed,
              "required_steps_unproven": steps_unproven, "records": records}
    report["missing_job_variants"] = missing_variants
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as stream:
            stream.write("### CI evidence\n\n")
            stream.write(f"Required jobs passed: {needs_ok}. Recorded failed attempts: {history_failed}. Required steps unproven: {steps_unproven}. Missing variants: {len(missing_variants)}.\n\n")
            stream.write("Read ci-evidence.json for workflow, SHA, job, attempt and failure boundary. "
                         "A later pass never resolves an earlier failure. Investigate and commit a fix/evidence; do not rerun until green.\n")
    return 1 if args.gate and (not needs_ok or history_failed or steps_unproven or missing_variants) else 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (OSError, ValueError, KeyError, TypeError) as exc:
        # Do not render arbitrary HTTP bodies or token-bearing request objects.
        print(f"CI evidence infrastructure failure ({type(exc).__name__}); history is unproven", file=sys.stderr)
        sys.exit(2)
