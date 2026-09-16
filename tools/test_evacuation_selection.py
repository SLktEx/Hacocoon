import contextlib
import copy
import io
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest

import evacuation_compare
import evacuation_selection as subject


class SelectionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="haco-selection-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.plan = subject.template()
        self.plan["categories"] = {name: {"status": "none", "reason": "Operator reviewed; no current data here"} for name in subject.CATEGORIES}
        self.plan["categories"]["managed-data"] = {"status": "listed", "reason": ""}
        self.item = self.plan["items"][0]
        self.item.update(name="作業中のコード", source="selected workspace", decision="retain",
                         source_manifest="source.json", restored_manifest="restored.json")
        self.manifest = {"format": 1, "complete": True, "errors": [], "entries": {
            ".": {"kind": "directory", "mode": "0o700", "uid": 1000, "gid": 1000,
                  "content": None, "xattrs": "a" * 64, "hardlink": None}}}
        self.save("source.json", self.manifest)
        self.save("restored.json", self.manifest)

    def save(self, name, value):
        path = self.root / name
        path.write_text(json.dumps(value), encoding="utf-8")
        return path

    def report(self):
        return subject.review(subject.read_plan(self.save("selection.json", self.plan)), self.root)

    def test_template_cannot_claim_review_or_matching(self):
        self.plan = subject.template()
        result = self.report()
        self.assertFalse(result["selection_reviewed"])
        self.assertFalse(result["selected_retained_data_matching"])
        self.assertEqual(result["items"][0]["status"], "unreviewed")

    def test_matching_is_scoped_and_never_deletion_authority(self):
        result = self.report()
        self.assertTrue(result["selection_reviewed"])
        self.assertTrue(result["selected_retained_data_matching"])
        self.assertFalse(result["authority"])
        self.assertFalse(result["backup_complete"])
        self.assertEqual(result["counts"], {"matching": 1})
        self.plan["categories"]["external-data"]["status"] = "pending"
        self.assertFalse(self.report()["selected_retained_data_matching"])

    def test_differences_incomplete_missing_and_unreadable_are_independent(self):
        altered = copy.deepcopy(self.manifest)
        altered["entries"]["."]["mode"] = "0o755"
        self.save("altered.json", altered)
        partial = copy.deepcopy(self.manifest)
        partial.update(complete=False, errors=[{"path": ".", "reason": "scan incomplete"}])
        self.save("partial.json", partial)
        (self.root / "broken.json").write_text("{", encoding="utf-8")
        cases = [("difference", "altered.json", "different"),
                 ("partial", "partial.json", "incomplete"),
                 ("missing", "", "missing-manifest"),
                 ("unreadable", "absent.json", "manifest-unavailable"),
                 ("broken", "broken.json", "manifest-unavailable"),
                 ("same", "source.json", "same-manifest")]
        for name, path, _ in cases:
            self.plan["items"].append(dict(self.item, name=name, restored_manifest=path))
        result = self.report()
        self.assertEqual([row["status"] for row in result["items"]], ["matching"] + [case[2] for case in cases])
        self.assertFalse(result["selected_retained_data_matching"])
        self.assertEqual(result["items"][1]["comparison"]["differences"], [{"path": ".", "fields": ["mode"]}])
        self.assertEqual(json.loads((self.root / "source.json").read_text()), self.manifest)

    def test_empty_and_recreated_only_do_not_claim_retained_match(self):
        self.item.update(decision="recreate", reason="Reinstall from reviewed setup")
        self.assertEqual(self.report()["items"][0]["status"], "recreation-unverified")
        self.assertFalse(self.report()["selected_retained_data_matching"])
        self.item.update(decision="exclude", reason="Disposable generated output")
        self.assertEqual(self.report()["items"][0]["status"], "excluded-by-operator")
        self.assertFalse(self.report()["selected_retained_data_matching"])
        self.plan["items"] = []
        self.plan["categories"]["managed-data"] = {"status": "none", "reason": "No current data"}
        self.assertFalse(self.report()["selected_retained_data_matching"])

    def test_ambiguous_or_unexplained_selection_is_refused(self):
        original = copy.deepcopy(self.plan)
        for mutate in (
            lambda p: p["items"].append(copy.deepcopy(p["items"][0])),
            lambda p: p["items"][0].update(decision="exclude", reason=""),
            lambda p: p["categories"].pop("external-data"),
            lambda p: p["categories"]["managed-data"].update(status="none", reason="conflict"),
            lambda p: p["categories"]["external-data"].update(status="listed"),
            lambda p: p.update(format=True),
        ):
            self.plan = copy.deepcopy(original)
            mutate(self.plan)
            with self.assertRaises(ValueError):
                self.report()
        path = self.root / "duplicate.json"
        path.write_text('{"format":1,"format":1}', encoding="utf-8")
        with self.assertRaises(ValueError):
            subject.read_plan(path)

    def test_cli_resolves_paths_beside_plan_and_preserves_nonzero_results(self):
        path = self.save("selection.json", self.plan)
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = subject.main(["report", str(path)])
        self.assertEqual(code, 0)
        self.assertEqual(json.loads(output.getvalue())["items"][0]["name"], "作業中のコード")
        (self.root / "restored.json").unlink()
        with contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(subject.main(["report", str(path)]), 1)
        with contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(subject.main(["report", str(self.root / "absent-plan")]), 2)

    @unittest.skipUnless(hasattr(os, "O_NOFOLLOW") and Path("/proc/self/mountinfo").exists(), "Linux tree scan required")
    def test_two_real_restored_trees_and_one_unrestored_selection(self):
        self.plan["items"] = []
        for name in ("workspace", "manual"):
            source = self.root / (name + "-source")
            restored = self.root / (name + "-restored")
            source.mkdir(mode=0o700)
            (source / ".git").mkdir()
            (source / ".git" / "HEAD").write_text("ref: refs/heads/main\n")
            (source / "dirty").write_text("private data")
            (source / "link").symlink_to("dirty")
            shutil.copytree(source, restored, symlinks=True)
            self.save(name + "-source.json", evacuation_compare.scan_tree(str(source)))
            self.save(name + "-restored.json", evacuation_compare.scan_tree(str(restored)))
            self.plan["items"].append(dict(self.item, name=name, source=str(source),
                                           source_manifest=name + "-source.json", restored_manifest=name + "-restored.json"))
        self.assertTrue(self.report()["selected_retained_data_matching"])
        (self.root / "manual-restored" / "dirty").write_text("lost changes")
        self.save("manual-restored.json", evacuation_compare.scan_tree(str(self.root / "manual-restored")))
        self.plan["items"].append(dict(self.item, name="not restored", restored_manifest=""))
        result = self.report()
        self.assertEqual([row["status"] for row in result["items"]], ["matching", "different", "missing-manifest"])
        self.assertFalse(result["selected_retained_data_matching"])
        self.assertEqual((self.root / "manual-source" / "dirty").read_text(), "private data")


if __name__ == "__main__":
    unittest.main()
