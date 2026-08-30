#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path
from typing import Any

API_VERSION = "2026-03-10"
MAIN_RULESET_NAME = "Protect main"
TAG_RULESET_NAME = "Protect release tags"
RELEASE_ENVIRONMENT = "release"
REQUIRED_PR_CREATION_POLICY = "collaborators_only"

REQUIRED_STATUS_CONTEXTS = {
    "docs",
    "workflow-policy",
    "release-config",
    "test (1.26.x)",
    "test (1.27.x)",
    "race",
    "e2e",
}
RECOMMENDED_STATUS_CONTEXTS = {"gitleaks"}


def gh_api(path: str) -> Any:
    proc = subprocess.run(
        [
            "gh",
            "api",
            "-H",
            "Accept: application/vnd.github+json",
            "-H",
            f"X-GitHub-Api-Version: {API_VERSION}",
            path,
        ],
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if proc.returncode != 0:
        detail = proc.stderr.strip() or f"gh api exited {proc.returncode}"
        raise RuntimeError(f"GitHub API check failed for {path}: {detail}")
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"GitHub API returned invalid JSON for {path}: {exc}") from exc


def rules_by_type(ruleset: dict[str, Any]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for rule in ruleset.get("rules") or []:
        rule_type = rule.get("type")
        if isinstance(rule_type, str):
            result[rule_type] = rule
    return result


def _ref_condition(ruleset: dict[str, Any]) -> tuple[set[str], set[str]]:
    ref_name = ((ruleset.get("conditions") or {}).get("ref_name") or {})
    return set(ref_name.get("include") or []), set(ref_name.get("exclude") or [])


def ref_matches(ruleset: dict[str, Any], expected: str, default_branch: str) -> bool:
    includes, excludes = _ref_condition(ruleset)
    accepted = {expected}
    if expected == f"refs/heads/{default_branch}":
        accepted.add("~DEFAULT_BRANCH")
    return bool(includes & accepted) and not bool(excludes & accepted)


def validate_repository_policy(repository: dict[str, Any]) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []

    if repository.get("private") is not False:
        errors.append("repository must be public before public-release readiness can pass")

    policy = repository.get("pull_request_creation_policy")
    if policy != REQUIRED_PR_CREATION_POLICY:
        errors.append(
            "external pull requests must remain disabled: "
            f"pull_request_creation_policy must be {REQUIRED_PR_CREATION_POLICY!r}, got {policy!r}"
        )

    return errors, warnings


def validate_collaborators(collaborators: Any, owner_login: str) -> tuple[list[str], list[str]]:
    if not isinstance(collaborators, list):
        return ["direct collaborator inventory is incomplete or malformed"], []

    non_owner: list[str] = []
    for item in collaborators:
        if not isinstance(item, dict):
            return ["direct collaborator inventory is incomplete or malformed"], []
        login = item.get("login")
        if isinstance(login, str) and login != owner_login:
            non_owner.append(login)

    if non_owner:
        return [
            "solo-maintainer public policy requires no non-owner direct collaborators; found: "
            + ", ".join(sorted(non_owner))
        ], []

    return [], []


def validate_main_ruleset(ruleset: dict[str, Any], default_branch: str) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []

    if ruleset.get("enforcement") != "active":
        errors.append(f"{MAIN_RULESET_NAME} must be active")
    if ruleset.get("target") != "branch":
        errors.append(f"{MAIN_RULESET_NAME} must target branches")
    if not ref_matches(ruleset, f"refs/heads/{default_branch}", default_branch):
        errors.append(f"{MAIN_RULESET_NAME} must include the default branch and not exclude it")
    if ruleset.get("bypass_actors"):
        errors.append(f"{MAIN_RULESET_NAME} must not have bypass actors")

    rules = rules_by_type(ruleset)
    for required in ("deletion", "non_fast_forward", "pull_request", "required_status_checks"):
        if required not in rules:
            errors.append(f"{MAIN_RULESET_NAME} missing required rule: {required}")

    pr = rules.get("pull_request") or {}
    params = pr.get("parameters") or {}
    review_count = int(params.get("required_approving_review_count") or 0)
    if review_count < 0:
        errors.append(f"{MAIN_RULESET_NAME} approving review count is invalid")
    elif review_count == 0:
        warnings.append(
            f"{MAIN_RULESET_NAME} requires no independent approval; this is intentional only "
            "for the solo-maintainer / external-PR-disabled policy"
        )

    if params.get("dismiss_stale_reviews_on_push") is not True:
        errors.append(f"{MAIN_RULESET_NAME} must dismiss stale approvals after new pushes")
    last_push_approval = params.get("require_last_push_approval")
    if last_push_approval is not False:
        errors.append(
            f"{MAIN_RULESET_NAME} must disable approval of the most recent push in solo-maintainer mode; "
            "enabling it makes self-authored maintenance unmergeable without a second trusted actor"
        )
    if params.get("required_review_thread_resolution") is not True:
        errors.append(f"{MAIN_RULESET_NAME} must require review-thread resolution")
    if params.get("require_code_owner_review") is not True:
        warnings.append(
            f"{MAIN_RULESET_NAME} does not require CODEOWNER review; enable it when a second trusted reviewer exists"
        )

    status = rules.get("required_status_checks") or {}
    status_params = status.get("parameters") or {}
    if status_params.get("strict_required_status_checks_policy") is not True:
        errors.append(f"{MAIN_RULESET_NAME} must require status checks against the latest base branch")

    contexts = {
        item.get("context")
        for item in status_params.get("required_status_checks") or []
        if isinstance(item, dict) and isinstance(item.get("context"), str)
    }
    missing = sorted(REQUIRED_STATUS_CONTEXTS - contexts)
    if missing:
        errors.append(f"{MAIN_RULESET_NAME} missing required status checks: {', '.join(missing)}")

    recommended_missing = sorted(RECOMMENDED_STATUS_CONTEXTS - contexts)
    if recommended_missing:
        warnings.append(
            f"{MAIN_RULESET_NAME} does not require recommended checks: "
            + ", ".join(recommended_missing)
        )

    return errors, warnings


def validate_tag_ruleset(ruleset: dict[str, Any]) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []

    if ruleset.get("enforcement") != "active":
        errors.append(f"{TAG_RULESET_NAME} must be active")
    if ruleset.get("target") != "tag":
        errors.append(f"{TAG_RULESET_NAME} must target tags")

    includes, excludes = _ref_condition(ruleset)
    if "refs/tags/v*" not in includes or "refs/tags/v*" in excludes:
        errors.append(f"{TAG_RULESET_NAME} must include refs/tags/v* and not exclude it")

    if ruleset.get("bypass_actors"):
        errors.append(f"{TAG_RULESET_NAME} must not have bypass actors in solo-maintainer mode")

    rules = rules_by_type(ruleset)
    for required in ("deletion", "update", "non_fast_forward"):
        if required not in rules:
            errors.append(f"{TAG_RULESET_NAME} missing required rule: {required}")

    if "creation" not in rules:
        warnings.append(
            f"{TAG_RULESET_NAME} does not restrict tag creation; this is accepted only because "
            "the repository has no non-owner collaborators and release trust requires current main HEAD"
        )

    return errors, warnings


def validate_release_environment(environment: dict[str, Any]) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []

    if environment.get("name") != RELEASE_ENVIRONMENT:
        errors.append(f"{RELEASE_ENVIRONMENT} Environment is missing or has unexpected identity")
        return errors, warnings

    protection_rules = environment.get("protection_rules") or []
    reviewer_rule = next(
        (
            rule
            for rule in protection_rules
            if isinstance(rule, dict) and rule.get("type") == "required_reviewers"
        ),
        None,
    )
    if reviewer_rule is None:
        warnings.append(
            f"{RELEASE_ENVIRONMENT} Environment has no independent reviewer; accepted for the "
            "solo-maintainer policy and must be revisited when a second trusted maintainer exists"
        )

    return errors, warnings


def validate_runners(runners: dict[str, Any]) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    total = runners.get("total_count")
    entries = runners.get("runners")
    if not isinstance(total, int) or not isinstance(entries, list):
        return ["repository self-hosted runner inventory is incomplete or malformed"], []
    if total != len(entries):
        errors.append(
            f"repository self-hosted runner inventory is incomplete: total_count={total}, returned={len(entries)}"
        )
    if total != 0:
        names = ", ".join(
            str(item.get("name", "<unnamed>")) for item in entries if isinstance(item, dict)
        )
        errors.append(
            f"public Hacocoon repository must have zero repository self-hosted runners; found {total}: {names}"
        )
    return errors, []


def validate_org_runner_groups(groups: Any, owner_type: str) -> tuple[list[str], list[str]]:
    if owner_type != "Organization":
        return [], []
    if not isinstance(groups, dict):
        return ["organization runner-group visibility could not be verified"], []
    total = groups.get("total_count")
    entries = groups.get("runner_groups")
    if not isinstance(total, int) or not isinstance(entries, list):
        return ["organization runner-group visibility response is incomplete or malformed"], []
    if total != len(entries):
        return [
            f"organization runner-group visibility response is incomplete: total_count={total}, returned={len(entries)}"
        ], []
    if entries:
        names = ", ".join(
            str(item.get("name", "<unnamed>")) for item in entries if isinstance(item, dict)
        )
        return [
            f"no organization self-hosted runner group may be visible to this public repository; found: {names}"
        ], []
    return [], []


def find_named_ruleset(summaries: Any, name: str) -> dict[str, Any]:
    if not isinstance(summaries, list):
        raise RuntimeError("repository ruleset list is incomplete or malformed")
    matches = [item for item in summaries if isinstance(item, dict) and item.get("name") == name]
    if len(matches) != 1:
        raise RuntimeError(f"expected exactly one repository ruleset named {name!r}, found {len(matches)}")
    return matches[0]


def load_live_snapshot(repo: str) -> dict[str, Any]:
    repository = gh_api(f"repos/{repo}")
    if repository.get("private") is not False:
        raise RuntimeError(
            "repository is not public; server-side public-release protections cannot yet be verified"
        )

    default_branch = repository.get("default_branch")
    owner = repository.get("owner") or {}
    owner_type = owner.get("type")
    owner_login = owner.get("login")
    if not isinstance(default_branch, str) or not default_branch:
        raise RuntimeError("repository default branch is unavailable")
    if owner_type not in {"User", "Organization"} or not isinstance(owner_login, str):
        raise RuntimeError("repository owner metadata is unavailable")

    summaries = gh_api(f"repos/{repo}/rulesets?per_page=100")
    main_summary = find_named_ruleset(summaries, MAIN_RULESET_NAME)
    tag_summary = find_named_ruleset(summaries, TAG_RULESET_NAME)
    for label, summary in ((MAIN_RULESET_NAME, main_summary), (TAG_RULESET_NAME, tag_summary)):
        if not isinstance(summary.get("id"), int):
            raise RuntimeError(f"{label} ruleset id is unavailable")

    snapshot: dict[str, Any] = {
        "repository": repository,
        "collaborators": gh_api(f"repos/{repo}/collaborators?affiliation=direct&per_page=100"),
        "main_ruleset": gh_api(f"repos/{repo}/rulesets/{main_summary['id']}"),
        "tag_ruleset": gh_api(f"repos/{repo}/rulesets/{tag_summary['id']}"),
        "release_environment": gh_api(f"repos/{repo}/environments/{RELEASE_ENVIRONMENT}"),
        "runners": gh_api(f"repos/{repo}/actions/runners?per_page=100"),
        "org_runner_groups": None,
    }
    if owner_type == "Organization":
        short_repo = repo.split("/", 1)[1]
        snapshot["org_runner_groups"] = gh_api(
            f"orgs/{owner_login}/actions/runner-groups?visible_to_repository={short_repo}&per_page=100"
        )
    return snapshot


def validate_snapshot(snapshot: dict[str, Any]) -> tuple[list[str], list[str]]:
    errors: list[str] = []
    warnings: list[str] = []

    repository = snapshot.get("repository")
    if not isinstance(repository, dict):
        return ["repository metadata missing from readiness snapshot"], []

    default_branch = repository.get("default_branch")
    if not isinstance(default_branch, str) or not default_branch:
        errors.append("repository default branch is unavailable")
        default_branch = "main"

    owner = repository.get("owner") or {}
    owner_type = owner.get("type")
    owner_login = owner.get("login")
    if owner_type not in {"User", "Organization"}:
        errors.append("repository owner type is unavailable or unsupported")
        owner_type = "User"
    if not isinstance(owner_login, str) or not owner_login:
        errors.append("repository owner login is unavailable")
        owner_login = "<unknown>"

    validators = [
        validate_repository_policy(repository),
        validate_collaborators(snapshot.get("collaborators"), owner_login),
        validate_main_ruleset(snapshot.get("main_ruleset") or {}, default_branch),
        validate_tag_ruleset(snapshot.get("tag_ruleset") or {}),
        validate_release_environment(snapshot.get("release_environment") or {}),
        validate_runners(snapshot.get("runners") or {}),
        validate_org_runner_groups(snapshot.get("org_runner_groups"), owner_type),
    ]
    for validation_errors, validation_warnings in validators:
        errors.extend(validation_errors)
        warnings.extend(validation_warnings)

    return errors, warnings


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Fail-closed validation of Hacocoon public GitHub trust-boundary settings."
    )
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--repo", help="GitHub repository in owner/name form; requires authenticated gh")
    source.add_argument("--snapshot", type=Path, help="offline JSON snapshot for regression/testing")
    args = parser.parse_args(argv)

    try:
        if args.snapshot:
            snapshot = json.loads(args.snapshot.read_text(encoding="utf-8"))
        else:
            snapshot = load_live_snapshot(args.repo)
        errors, warnings = validate_snapshot(snapshot)
    except (OSError, RuntimeError, json.JSONDecodeError) as exc:
        print(f"PUBLIC RELEASE READINESS UNVERIFIED: {exc}", file=sys.stderr)
        return 2

    for warning in warnings:
        print(f"WARNING: {warning}", file=sys.stderr)
    if errors:
        print("PUBLIC RELEASE READINESS FAILED", file=sys.stderr)
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1

    print("PUBLIC RELEASE READINESS OK")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
