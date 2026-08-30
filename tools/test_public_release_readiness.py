#!/usr/bin/env python3
from __future__ import annotations

import unittest

from check_public_release_readiness import REQUIRED_STATUS_CONTEXTS, validate_snapshot


def valid_snapshot():
    return {
        "repository": {
            "private": False,
            "default_branch": "main",
            "owner": {"type": "User", "login": "SLktEx"},
        },
        "main_ruleset": {
            "name": "protect-main",
            "enforcement": "active",
            "target": "branch",
            "bypass_actors": [],
            "conditions": {"ref_name": {"include": ["~DEFAULT_BRANCH"], "exclude": []}},
            "rules": [
                {"type": "deletion"},
                {"type": "non_fast_forward"},
                {
                    "type": "pull_request",
                    "parameters": {
                        "required_approving_review_count": 1,
                        "dismiss_stale_reviews_on_push": True,
                        "require_code_owner_review": False,
                        "require_last_push_approval": True,
                        "required_review_thread_resolution": True,
                    },
                },
                {
                    "type": "required_status_checks",
                    "parameters": {
                        "strict_required_status_checks_policy": True,
                        "required_status_checks": [
                            {"context": context} for context in sorted(REQUIRED_STATUS_CONTEXTS)
                        ],
                    },
                },
            ],
        },
        "tag_ruleset": {
            "name": "protect-release-tags",
            "enforcement": "active",
            "target": "tag",
            "bypass_actors": [
                {"actor_type": "RepositoryRole", "actor_id": 5, "bypass_mode": "always"}
            ],
            "conditions": {"ref_name": {"include": ["refs/tags/v*"], "exclude": []}},
            "rules": [
                {"type": "creation"},
                {"type": "deletion"},
                {"type": "non_fast_forward"},
            ],
        },
        "release_environment": {
            "name": "release",
            "protection_rules": [
                {
                    "type": "required_reviewers",
                    "prevent_self_review": True,
                    "reviewers": [{"type": "User", "reviewer": {"login": "trusted-reviewer"}}],
                }
            ],
        },
        "fork_policy": {"approval_policy": "all_external_contributors"},
        "runners": {"total_count": 0, "runners": []},
        "org_runner_groups": None,
    }


def messages(snapshot):
    return validate_snapshot(snapshot)


class PublicReleaseReadinessTests(unittest.TestCase):
    def assert_rejected(self, mutate, needle):
        snapshot = valid_snapshot()
        mutate(snapshot)
        errors, _ = messages(snapshot)
        self.assertTrue(any(needle in error for error in errors), errors)

    def test_valid_snapshot_passes(self):
        errors, warnings = messages(valid_snapshot())
        self.assertEqual(errors, [])
        self.assertTrue(any("CODEOWNER" in warning for warning in warnings))

    def test_private_repository_fails(self):
        self.assert_rejected(lambda s: s["repository"].update(private=True), "must be public")

    def test_main_bypass_fails(self):
        self.assert_rejected(
            lambda s: s["main_ruleset"].update(
                bypass_actors=[
                    {"actor_type": "RepositoryRole", "actor_id": 5, "bypass_mode": "always"}
                ]
            ),
            "must not have bypass actors",
        )

    def test_missing_force_push_rule_fails(self):
        self.assert_rejected(
            lambda s: s["main_ruleset"].update(
                rules=[r for r in s["main_ruleset"]["rules"] if r["type"] != "non_fast_forward"]
            ),
            "non_fast_forward",
        )

    def test_missing_required_status_fails(self):
        def mutate(s):
            rule = next(
                r for r in s["main_ruleset"]["rules"] if r["type"] == "required_status_checks"
            )
            rule["parameters"]["required_status_checks"] = [
                x for x in rule["parameters"]["required_status_checks"] if x["context"] != "race"
            ]

        self.assert_rejected(mutate, "race")

    def test_tag_creation_restriction_is_required(self):
        self.assert_rejected(
            lambda s: s["tag_ruleset"].update(
                rules=[r for r in s["tag_ruleset"]["rules"] if r["type"] != "creation"]
            ),
            "creation",
        )

    def test_tag_bypass_must_be_minimal(self):
        def mutate(s):
            s["tag_ruleset"]["bypass_actors"].append(
                {"actor_type": "RepositoryRole", "actor_id": 4, "bypass_mode": "always"}
            )

        self.assert_rejected(mutate, "exactly one")

    def test_release_environment_requires_reviewer(self):
        self.assert_rejected(
            lambda s: s["release_environment"]["protection_rules"][0].update(reviewers=[]),
            "at least one required reviewer",
        )

    def test_release_environment_prevents_self_review(self):
        self.assert_rejected(
            lambda s: s["release_environment"]["protection_rules"][0].update(
                prevent_self_review=False
            ),
            "prevent self-review",
        )

    def test_all_external_contributors_approval_is_required(self):
        self.assert_rejected(
            lambda s: s["fork_policy"].update(
                approval_policy="first_time_contributors_new_to_github"
            ),
            "all external contributors",
        )

    def test_repository_self_hosted_runner_fails(self):
        def mutate(s):
            s["runners"] = {"total_count": 1, "runners": [{"name": "danger-runner"}]}

        self.assert_rejected(mutate, "zero repository self-hosted runners")

    def test_incomplete_runner_inventory_fails(self):
        def mutate(s):
            s["runners"] = {"total_count": 2, "runners": [{"name": "one"}]}

        self.assert_rejected(mutate, "inventory is incomplete")

    def test_visible_org_runner_group_fails(self):
        def mutate(s):
            s["repository"]["owner"]["type"] = "Organization"
            s["org_runner_groups"] = {
                "total_count": 1,
                "runner_groups": [{"name": "shared-privileged"}],
            }

        self.assert_rejected(mutate, "no organization self-hosted runner group")

    def test_missing_org_runner_group_snapshot_fails(self):
        def mutate(s):
            s["repository"]["owner"]["type"] = "Organization"
            s["org_runner_groups"] = None

        self.assert_rejected(mutate, "could not be verified")


if __name__ == "__main__":
    unittest.main()
