package buildinfo

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

func TestGeneratedCheckpointMatchesAuthorities(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	checks := map[string]*regexp.Regexp{
		"docs/status/versioning-and-release-status.md":    regexp.MustCompile(`current milestone position is \*\*(v0\.\d+)\*\*`),
		"docs/status/versioning-and-release-status.ja.md": regexp.MustCompile(`現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*`),
		"docs/IMPLEMENTATION_STATUS.md":                  regexp.MustCompile(`current milestone position is \*\*(v0\.\d+)\*\*`),
		"docs/IMPLEMENTATION_STATUS.ja.md":               regexp.MustCompile(`現在のmilestone位置は\s*\*\*(v0\.\d+)\*\*`),
	}
	for rel, re := range checks {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		match := re.FindSubmatch(data)
		if len(match) != 2 {
			t.Fatalf("%s: current checkpoint declaration not found", rel)
		}
		if got := string(match[1]); got != GeneratedCheckpoint {
			t.Fatalf("%s: checkpoint=%s generated=%s", rel, got, GeneratedCheckpoint)
		}
	}
}

func TestShortCommit(t *testing.T) {
	if got := ShortCommit("1234567890abcdef"); got != "1234567890ab" {
		t.Fatalf("got %q", got)
	}
	if got := ShortCommit(""); got != "unknown" {
		t.Fatalf("got %q", got)
	}
}
