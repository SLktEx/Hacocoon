package cliui

import (
	"errors"
	"strings"
	"testing"
)

func TestEnvironmentCatalogFallbackAndParallelLookup(t *testing.T) {
	for id, entry := range environmentCatalog {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 100; i++ {
				if English.Text(id) != entry.en || Japanese.Text(id) != entry.ja || Language("unknown").Text(id) != entry.en {
					t.Fatal("surface catalog lookup or English fallback changed")
				}
			}
		})
	}
}

func TestEnvironmentFailureMessagesKeepOriginalDetail(t *testing.T) {
	original := errors.New("SSH: exit status 108 / source-%s / unknown.message / 日本語")
	for _, id := range []string{"env.copy.failed", "env.import.failed", "env.export.failed"} {
		for _, language := range []Language{English, Japanese} {
			got := language.Format(id, original)
			if !strings.HasSuffix(got, original.Error()+"\n") || strings.Contains(got, "%!") {
				t.Fatalf("original error was translated or reinterpreted: %q", got)
			}
		}
	}
}
