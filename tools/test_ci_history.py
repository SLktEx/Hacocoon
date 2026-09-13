#!/usr/bin/env python3
import unittest

from ci_history import Actions, check_needs, failure_boundary, summarize


class HistoryTests(unittest.TestCase):
    def run_fixture(self, attempts, second_run=False):
        runs = [{"id": 1, "name": "test", "workflow_id": 42, "head_sha": "a" * 40,
                 "event": "pull_request", "run_attempt": len(attempts)}]
        if second_run:
            runs[0]["run_attempt"] = 1
            runs.append(dict(runs[0], id=2))
        def jobs(run, attempt):
            conclusion = attempts[(run if second_run else attempt) - 1]
            return [{"id": run * 100 + attempt, "name": "unit", "conclusion": conclusion,
                     "steps": [{"name": "[product] PTY contract", "conclusion": conclusion}]}]
        return summarize(runs, jobs)

    def test_unchanged_rerun_is_incident(self):
        rows = self.run_fixture(["failure", "success"])
        self.assertTrue(rows[0]["red_then_green"])
        self.assertEqual(rows[0]["failed_steps"][0]["boundary"], "product")

    def test_reopened_run_does_not_erase_failure(self):
        self.assertTrue(self.run_fixture(["failure", "success"], True)[0]["red_then_green"])

    def test_repeated_failure_is_not_flake_and_cancellation_is_not_pass(self):
        for results in (["failure", "failure"], ["failure", "cancelled"], ["cancelled", "success"]):
            self.assertFalse(self.run_fixture(results)[0]["red_then_green"])

    def test_partial_rerun_retains_other_job_failures(self):
        run = {"id": 1, "name": "test", "workflow_id": 1, "head_sha": "a"*40,
               "event": "pull_request", "run_attempt": 2}
        rows = summarize([run], lambda r, a: [{"id": a, "name": "one" if a == 1 else "two", "conclusion": "failure" if a == 1 else "success"}])
        self.assertEqual(len(rows), 2)
        self.assertFalse(rows[0]["red_then_green"])

    def test_needs_must_be_complete_and_successful(self):
        for state in ("skipped", "cancelled", "failure", None):
            self.assertFalse(check_needs({"unit": {"result": state}}, ["unit"]))
        self.assertTrue(check_needs({"unit": {"result": "success"}}, ["unit"]))
        for needs in ({}, {"unexpected": {"result": "success"}}):
            with self.assertRaises(ValueError):
                check_needs(needs, ["unit"])

    def test_successful_job_cannot_hide_skipped_missing_or_duplicate_step(self):
        run = {"id": 1, "name": "test", "workflow_id": 1, "head_sha": "a"*40,
               "event": "pull_request", "run_attempt": 1}
        for conclusions in ([], ["skipped"], ["success", "success"], ["success"]):
            jobs = [{"id": 1, "name": "unit (1.27.x)", "conclusion": "success",
                     "steps": [{"name": "contract", "conclusion": c} for c in conclusions]}]
            rows = summarize([run], lambda r, a: jobs, {"unit": ["contract"]})
            self.assertEqual(bool(rows[0]["unproven_steps"]), conclusions != ["success"])

    def test_classification_is_not_guessed_from_log_text(self):
        self.assertEqual(failure_boundary("apt error inside a product test"), "unclassified")
        self.assertEqual(failure_boundary("[infrastructure] Incus bootstrap"), "infrastructure")
        self.assertEqual(failure_boundary("[fixture] build"), "fixture")

    def test_pagination_and_missing_pages(self):
        api = Actions("owner/repo", "not-a-real-token")
        calls = []
        def get(path):
            calls.append(path)
            return {"total_count": 101, "jobs": list(range(100)) if path.endswith("&page=1") else [100]}
        api.get = get
        self.assertEqual(len(api.pages("runs/1/jobs", "jobs")), 101)
        self.assertEqual(len(calls), 2)
        api.get = lambda path: {"total_count": 10, "jobs": []}
        with self.assertRaises(ValueError):
            api.pages("runs/1/jobs", "jobs")

    def test_invalid_repository_or_path_never_sends_token(self):
        for repo in ("../repo", "https://attacker.invalid/repo", "owner/repo?x", "owner/repo\n"):
            with self.assertRaises(ValueError):
                Actions(repo, "private")
        api = Actions("owner/repo", "private")
        for path in ("../secrets", "https://attacker.invalid", "runs/1\n"):
            with self.assertRaises(ValueError):
                api.get(path)


if __name__ == "__main__":
    unittest.main()
