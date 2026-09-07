package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurationIsBoundedAndStrict(t *testing.T) {
	p := filepath.Join(t.TempDir(), "review.json")
	for _, s := range []string{`{}`, `{"distribution":"-x"}`, `{"distribution":"Hacocoon","command":"sh"}`, `{"distribution":"Hacocoon"} {}`, strings.Repeat(" ", 4097)} {
		if err := os.WriteFile(p, []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadConfiguration(p); err == nil {
			t.Errorf("accepted invalid config")
		}
	}
	if err := os.WriteFile(p, []byte(`{"distribution":"Hacocoon"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if c, err := loadConfiguration(p); err != nil || c.Distribution != "Hacocoon" {
		t.Fatal(c, err)
	}
}
