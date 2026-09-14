package experimental

import (
	"strings"
	"testing"
	"time"
)

func TestVSCodeSchema(t *testing.T) {
	if _, err := VSCodeFromObject(nil); err == nil {
		t.Fatal("null subtree accepted")
	}
	valid := `settings:
  editor.formatOnSave: true
  search.exclude: {"**/generated": true}
  test.null: null
extensions:
  minReleaseAge: 30d
  preRelease: deny
  install:
    - id: ms-python.python
    - id: rust-lang.rust-analyzer
      preRelease: allow
    - id: some.extension
      version: 1.2.3
`
	v, err := ParseVSCode([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	if v.Settings["editor.formatOnSave"] != true || len(v.Extensions.Install) != 3 {
		t.Fatal(v)
	}
	for _, s := range []string{
		"", "null", "[]", "settings: {}\n---\nsettings: {}", "settings: {}\nsettings: {}",
		"settings: {a: 1, a: 2}", "settings: {1: true}", "settings: {a: .nan}",
		"settings: null", "extensions: null", "extensions: {install: null}",
		"extensions: {install: [null]}", "extensions: {force: true}", "repo: {}",
		"extensions: {preRelease: true}", "extensions: {preRelease: maybe}",
		"extensions: {install: [{id: '--option'}]}", "extensions: {install: [{id: a.b, version: latest}]}",
		"extensions: {install: [{id: a.b, version: 1.2}]}",
		"extensions: {install: [{id: a.b, version: null}]}",
		"extensions: {install: [{id: a.b}, {id: A.B}]}",
		"extensions: {install: [{id: a.b, minReleaseAge: 1d}]}",
	} {
		if _, err := ParseVSCode([]byte(s)); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	if _, err := ParseVSCode([]byte(strings.Repeat(" ", MaxBytes+1))); err == nil {
		t.Fatal("size limit")
	}
}

func TestReleaseAge(t *testing.T) {
	for s, want := range map[string]time.Duration{"": 30 * 24 * time.Hour, "30d": 30 * 24 * time.Hour, "0d": 0, "48h": 48 * time.Hour, "90m": 90 * time.Minute} {
		got, err := ReleaseAge(s)
		if err != nil || got != want {
			t.Fatalf("%q: %v %v", s, got, err)
		}
	}
	for _, s := range []string{"-1d", "1.5d", "tomorrow", "99999999999999999d", "106752d", "-1h"} {
		if _, err := ReleaseAge(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
