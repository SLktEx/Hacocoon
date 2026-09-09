package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBaseDefinitionIsBoundedAndStrict(t *testing.T) {
	for _, data := range []string{`{"name":"tools","run":"true"}`, `{"name":"tools","run":"true","privileged":true}`, `{"name":"tools","run":"true"} {}`, `{"name":"../tools","run":"true"}`, strings.Repeat("x", 300000)} {
		file := filepath.Join(t.TempDir(), "base.json")
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := readBaseDefinition(file)
		if (err == nil) != (data == `{"name":"tools","run":"true"}`) {
			t.Fatal(err)
		}
	}
	if _, err := readBaseDefinition(t.TempDir()); err == nil {
		t.Fatal("directory accepted")
	}
}
