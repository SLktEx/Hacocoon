package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCheckpointSource(t *testing.T) {
	source, err := parseCheckpointSource([]byte(`# checkpoint source
schema: 1
current: "v0.2"
milestones:
  "v0.1": "First Gate"
  "v0.2": "Second Gate"
`))
	if err != nil {
		t.Fatal(err)
	}
	if source.Current != "v0.2" || len(source.Milestones) != 2 || source.Milestones[1].Gate != "Second Gate" {
		t.Fatalf("unexpected source: %#v", source)
	}
}

func TestParseCheckpointSourceRejectsInvalidSequence(t *testing.T) {
	cases := map[string]string{
		"duplicate": `schema: 1
current: "v0.1"
milestones:
  "v0.1": "One"
  "v0.1": "Again"
`,
		"gap": `schema: 1
current: "v0.3"
milestones:
  "v0.1": "One"
  "v0.3": "Three"
`,
		"stale current": `schema: 1
current: "v0.1"
milestones:
  "v0.1": "One"
  "v0.2": "Two"
`,
		"unsupported yaml": `schema: 1
current: "v0.1"
milestones:
  - version: "v0.1"
    gate: "One"
`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseCheckpointSource([]byte(input)); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBumpAdvancesYAMLAndAllMirrors(t *testing.T) {
	root := t.TempDir()
	writeBumpFixture(t, root)

	if err := bump(root, "v0.3", "Build Identity"); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		checkpointSourcePath,
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
		if !strings.Contains(string(data), "v0.3") {
			t.Fatalf("%s was not advanced: %s", rel, data)
		}
	}
	for _, rel := range []string{"docs/status/versioning-and-release-status.md", "docs/status/versioning-and-release-status.ja.md"} {
		data, _ := os.ReadFile(filepath.Join(root, rel))
		if !strings.Contains(string(data), "| v0.3 | Build Identity |") {
			t.Fatalf("%s missing new table row: %s", rel, data)
		}
	}

	sourceData, _ := os.ReadFile(filepath.Join(root, checkpointSourcePath))
	source, err := parseCheckpointSource(sourceData)
	if err != nil {
		t.Fatal(err)
	}
	if source.Current != "v0.3" || source.Milestones[2].Gate != "Build Identity" {
		t.Fatalf("unexpected updated source: %#v", source)
	}
}

func TestBumpRejectsSkippedCheckpoint(t *testing.T) {
	root := t.TempDir()
	writeBumpFixture(t, root)

	if err := bump(root, "v0.4", "Skipped"); err == nil || !strings.Contains(err.Error(), "next checkpoint must be v0.3") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBumpRejectsMirrorMismatch(t *testing.T) {
	root := t.TempDir()
	writeBumpFixture(t, root)
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.ja.md", "現在のmilestone位置は **v0.1** です。\n")

	if err := bump(root, "v0.3", "Gate"); err == nil || !strings.Contains(err.Error(), "disagrees with") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBumpRejectsGateTableDrift(t *testing.T) {
	root := t.TempDir()
	writeBumpFixture(t, root)
	writeFixture(t, root, "docs/status/versioning-and-release-status.ja.md", versionTable("Wrong Gate")+"\n現在のmilestone位置は **v0.2** です。\n")

	if err := bump(root, "v0.3", "Gate"); err == nil || !strings.Contains(err.Error(), "expected v0.2 / \"Second Gate\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRepositoryBumpMilestoneBlackBox(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for repository black-box milestone test")
	}
	cmd := exec.Command(python, filepath.Join(root, "tools/test_bump_milestone.py"))
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("black-box milestone workflow failed: %v\n%s", err, output)
	}
}

func writeBumpFixture(t *testing.T, root string) {
	t.Helper()
	writeFixture(t, root, checkpointSourcePath, `schema: 1
current: "v0.2"
milestones:
  "v0.1": "First Gate"
  "v0.2": "Second Gate"
`)
	writeFixture(t, root, "docs/status/versioning-and-release-status.md", versionTable("Second Gate")+"\nThe current milestone position is **v0.2**.\n")
	writeFixture(t, root, "docs/status/versioning-and-release-status.ja.md", versionTable("Second Gate")+"\n現在のmilestone位置は **v0.2** です。\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.md", "The current milestone position is **v0.2**.\n")
	writeFixture(t, root, "docs/IMPLEMENTATION_STATUS.ja.md", "現在のmilestone位置は **v0.2** です。\n")
	writeFixture(t, root, "internal/buildinfo/checkpoint_generated.go", "// Code generated from docs/status/checkpoints.yaml by tools/bump-milestone; DO NOT EDIT.\npackage buildinfo\n\nconst GeneratedCheckpoint = \"v0.2\"\n")
}

func versionTable(secondGate string) string {
	return "| Version | Gate | status |\n|---|---|---|\n| v0.1 | First Gate | ok |\n| v0.2 | " + secondGate + " | ok |\n"
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
