package cliui

import (
	"strings"
	"testing"

	"golang.org/x/text/width"
)

func TestHelpExplanationFitsNarrowAndNormalTerminals(t *testing.T) {
	for _, columns := range []int{60, 80} {
		for _, text := range []string{commandCatalog["command.env.create"].en, commandCatalog["command.env.create"].ja, strings.Repeat("日本語", 50)} {
			got := HelpLines("  create        ", text, columns)
			for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
				cells := 0
				for _, r := range line {
					n := 1
					if kind := width.LookupRune(r).Kind(); kind == width.EastAsianWide || kind == width.EastAsianFullwidth {
						n = 2
					}
					cells += n
				}
				if cells > columns {
					t.Fatalf("%d-column explanation overflows: %q", columns, line)
				}
			}
		}
	}
}
