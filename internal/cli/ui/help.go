package cliui

import (
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

// HelpLines wraps human explanations without interpreting terminal settings or
// changing the copyable command syntax supplied separately by the client.
func HelpLines(prefix, explanation string, columns int) string {
	if columns < 40 {
		columns = 40
	}
	indent := strings.Repeat(" ", len(prefix))
	available := columns - len(prefix)
	if available < 2 {
		return prefix + explanation + "\n"
	}
	var out strings.Builder
	remaining := []rune(explanation)
	for len(remaining) > 0 {
		cells, end, space := 0, 0, -1
		for end < len(remaining) {
			r := remaining[end]
			n := 1
			if unicode.Is(unicode.Mn, r) {
				n = 0
			} else if kind := width.LookupRune(r).Kind(); kind == width.EastAsianWide || kind == width.EastAsianFullwidth {
				n = 2
			}
			if cells+n > available {
				break
			}
			if r == ' ' {
				space = end
			}
			cells += n
			end++
		}
		if end < len(remaining) && space > 0 {
			end = space
		}
		out.WriteString(prefix)
		out.WriteString(strings.TrimSpace(string(remaining[:end])))
		out.WriteByte('\n')
		remaining = []rune(strings.TrimLeft(string(remaining[end:]), " "))
		prefix = indent
	}
	return out.String()
}
