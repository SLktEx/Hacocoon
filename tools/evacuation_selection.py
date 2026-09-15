"""Review named current-data selections using existing portable tree manifests."""
import argparse
import json
from pathlib import Path
import sys

from evacuation_compare import compare_manifests, read_manifest

CATEGORIES = ("managed-data", "host-settings", "manual-files", "external-data")
MAX_PLAN = 1024 * 1024
MAX_ITEMS = 4096
DECISIONS = ("pending", "retain", "recreate", "exclude")


def template():
    return {"format": 1,
            "categories": {name: {"status": "pending", "reason": ""} for name in CATEGORIES},
            "items": [{"name": "My workspace", "category": "managed-data",
                       "source": "Describe the current data and its location",
                       "decision": "pending", "reason": "",
                       "source_manifest": "", "restored_manifest": ""}]}


def _unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate field")
        result[key] = value
    return result


def _text(value, *, required=False):
    return isinstance(value, str) and len(value) <= 4096 and not any(ord(c) < 32 for c in value) and (not required or bool(value.strip()))


def read_plan(path):
    with open(path, "rb") as stream:
        raw = stream.read(MAX_PLAN + 1)
    if len(raw) > MAX_PLAN:
        raise ValueError("selection too large")
    plan = json.loads(raw, object_pairs_hook=_unique)
    if not isinstance(plan, dict) or set(plan) != {"format", "categories", "items"} or type(plan["format"]) is not int or plan["format"] != 1:
        raise ValueError("invalid selection")
    categories, items = plan["categories"], plan["items"]
    if not isinstance(categories, dict) or set(categories) != set(CATEGORIES):
        raise ValueError("all review categories required")
    for row in categories.values():
        if not isinstance(row, dict) or set(row) != {"status", "reason"} or row["status"] not in ("pending", "listed", "none") or not _text(row["reason"], required=row["status"] == "none"):
            raise ValueError("invalid category review")
    if not isinstance(items, list) or len(items) > MAX_ITEMS:
        raise ValueError("invalid selected items")
    names = set()
    for item in items:
        if not isinstance(item, dict) or set(item) != {"name", "category", "source", "decision", "reason", "source_manifest", "restored_manifest"}:
            raise ValueError("invalid selected item")
        if not all(_text(item[key], required=key in ("name", "source")) for key in item) or item["name"] in names:
            raise ValueError("invalid or duplicate name")
        names.add(item["name"])
        if item["category"] not in CATEGORIES or item["decision"] not in DECISIONS:
            raise ValueError("invalid selection decision")
        if item["decision"] in ("recreate", "exclude") and not item["reason"].strip():
            raise ValueError("recreation and exclusion need reasons")
    for category, row in categories.items():
        listed = any(item["category"] == category for item in items)
        if row["status"] == "listed" and not listed or row["status"] == "none" and listed:
            raise ValueError("category conflicts with item list")
    return plan


def review(plan, directory):
    """Read one pair at a time; an unreadable item must not hide other results."""
    rows = []
    for item in plan["items"]:
        row = {key: item[key] for key in ("name", "category", "source", "decision", "reason")}
        row["status"] = {"pending": "unreviewed", "recreate": "recreation-unverified",
                         "exclude": "excluded-by-operator", "retain": "missing-manifest"}[item["decision"]]
        if item["decision"] == "retain" and item["source_manifest"] and item["restored_manifest"]:
            source = directory / item["source_manifest"]
            restored = directory / item["restored_manifest"]
            try:
                if source.samefile(restored):
                    row["status"] = "same-manifest"
                else:
                    result = compare_manifests(read_manifest(source), read_manifest(restored))
                    row["status"] = "incomplete" if not result["comparison_complete"] else "matching" if result["matching"] else "different"
                    row["comparison"] = result
            except (OSError, ValueError, TypeError, KeyError, RecursionError):
                row["status"] = "manifest-unavailable"
        row["next"] = {
            "unreviewed": "Choose whether to retain, recreate or exclude this data.",
            "recreation-unverified": "Record and verify the recreated application's behavior separately.",
            "excluded-by-operator": "Keep the stated exclusion visible when reviewing the scope.",
            "missing-manifest": "Scan both selected trees and set their manifest paths.",
            "same-manifest": "Provide separately captured source and restored manifests.",
            "manifest-unavailable": "Inspect the two manifest paths and their scan results; retry this report.",
            "incomplete": "Resolve incomplete scans and capture new manifests after stopping writers.",
            "different": "Review the listed differences without modifying the source to force a match.",
            "matching": "Verify independent retention, owner namespaces and ordinary application use."
        }[row["status"]]
        rows.append(row)
    selection_reviewed = all(row["status"] != "pending" for row in plan["categories"].values()) and all(row["decision"] != "pending" for row in rows)
    retained = [row for row in rows if row["decision"] == "retain"]
    matching = bool(retained) and selection_reviewed and all(row["status"] == "matching" for row in retained)
    return {"format": 1, "selection_reviewed": selection_reviewed,
            "selected_retained_data_matching": matching,
            "categories": plan["categories"], "items": rows,
            "counts": {status: sum(row["status"] == status for row in rows) for status in sorted({row["status"] for row in rows})},
            "authority": False, "backup_complete": False,
            "unreviewed": ["operator selection completeness, including unregistered and external data",
                           "independent captures and retained storage; matching manifests can be copies",
                           "numeric owner namespaces and application consistency",
                           "recreated data, editor/build/OCI and authenticated Git operation"],
            "next": "Resolve named gaps and verify application use. This report never authorizes source deletion."}


def main(argv=None):
    parser = argparse.ArgumentParser(description="Review named required data and restored-tree comparisons; never changes data.")
    sub = parser.add_subparsers(dest="action", required=True)
    sub.add_parser("template", help="print an editable selection with every category unreviewed")
    report = sub.add_parser("report", help="compare the selected saved manifests on Linux or Windows")
    report.add_argument("selection")
    args = parser.parse_args(argv)
    try:
        result = template() if args.action == "template" else review(read_plan(args.selection), Path(args.selection).absolute().parent)
    except (OSError, ValueError, TypeError, KeyError, RecursionError):
        print("Selection unavailable. Check its format, unique names, category decisions and exclusion reasons. No data changed.", file=sys.stderr)
        return 2
    json.dump(result, sys.stdout, indent=2, ensure_ascii=True)
    print()
    return 0 if args.action == "template" or result["selected_retained_data_matching"] else 1


if __name__ == "__main__":
    sys.exit(main())
