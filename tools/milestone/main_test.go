package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBumpAdvancesAllAuthoritiesAndGeneratedInput(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/status/versioning-and-release-status.md", "# Versioning\n| Version | Gate | status |\n|---|---|---|\n| v0.26 | Old Gate | ✅ implemented |\n\nThe current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/status/versioning-and-release-status.ja.md", "# バージョン\n| Version | Gate | status |\n|---|---|---|\n| v0.26 | Old Gate | 実装済み |\n\n現在のmilestone位置は **v0.26** です。\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.md", "The current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.ja.md", "現在のmilestone位置は **v0.26** です。\n")
	writeFixture(t, root, "internal/buildinfo/checkpoint_generated.go", "// Code generated.\npackage buildinfo\n\nconst GeneratedCheckpoint = \"v0.26\"\n")

	if err := bump(root, "v0.27", "Build Identity"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"docs/status/versioning-and-release-status.md",
		"docs/status/versioning-and-release-status.ja.md",
		"docs/IMPLEMENTATION_STATUS.md",
		"docs/IMPLEMENTATION_STATUS.ja.md",
		"internal/buildinfo/checkpoint_generated.go",
	} {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "v0.27") {
			t.Fatalf("%s was not advanced: %s", rel, data)
		}
	}
	for _, rel := range []string{"docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md"} {
		data, _ := os.ReadFile(filepath.Join(root, rel))
		if !strings.Contains(string(data), "| v0.27 | Build Identity |") {
			t.Fatalf("%s missing new table row: %s", rel, data)
		}
	}
}

func TestBumpRejectsSkippedCheckpoint(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/status/versioning-and-release-status.md", "| v0.26 | Old | ok |\nThe current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/status/versioning-and-release-status.ja.md", "| v0.26 | Old | ok |\n現在のmilestone位置は **v0.26** です。\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.md", "The current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.ja.md", "現在のmilestone位置は **v0.26** です。\n")
	writeFixture(t, root, "internal/buildinfo/checkpoint_generated.go", "package buildinfo\nconst GeneratedCheckpoint = \"v0.26\"\n")

	if err := bump(root, "v0.28", "Skipped"); err == nil || !strings.Contains(err.Error(), "next checkpoint must be v0.27") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBumpRejectsAuthorityMismatch(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "docs/status/versioning-and-release-status.md", "| v0.26 | Old | ok |\nThe current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/status/versioning-and-release-status.ja.md", "| v0.25 | Old | ok |\n現在のmilestone位置は **v0.25** です。\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.md", "The current milestone position is **v0.26**.\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.ja.md", "現在のmilestone位置は **v0.26** です。\n")
	writeFixture(t, root, "internal/buildinfo/checkpoint_generated.go", "package buildinfo\nconst GeneratedCheckpoint = \"v0.26\"\n")

	if err := bump(root, "v0.27", "Gate"); err == nil || !strings.Contains(err.Error(), "authorities disagree") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
