package desktopreview

import (
	"context"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
)

func toastAdvance(t *testing.T, f *ToastFlow, action string, inputs map[string]string) *ToastIntent {
	t.Helper()
	p, err := f.Page()
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(p.Body, "\n"); len(lines) > 4 {
		t.Fatal("body exceeded four lines")
	} else {
		for _, line := range lines {
			if len(wrapToastText(line, 34)) > 1 {
				t.Fatalf("line exceeded display budget: %q", line)
			}
		}
	}
	x, err := ToastXML(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(x, "activationType=\"protocol\"") || strings.Contains(x, f.view.Token) {
		t.Fatal("unsafe native payload")
	}
	if err := f.Displayed(p.Nonce); err != nil {
		t.Fatal(err)
	}
	intent, err := f.Activate(p.Nonce+":"+action, inputs)
	if err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestToastAllChoicesRequireAllPagesAndExplicitConsistentAnswer(t *testing.T) {
	for _, ja := range []bool{false, true} {
		for _, choice := range []capability.SavedChoice{"", capability.AllowEnvironment, capability.DenyEnvironment, capability.AskEnvironment, capability.AllowGlobal, capability.DenyGlobal, capability.AskGlobal} {
			s, backend, id := newReviewFixture()
			view := selectReview(t, s, id, 1)
			f, err := NewToastFlow(*view, ja)
			if err != nil {
				t.Fatal(err)
			}
			for f.phase == "current" {
				if toastAdvance(t, f, "next", nil) != nil {
					t.Fatal("navigation answered")
				}
			}
			level, policy := "once", "ask"
			if choice != "" {
				policy, level, _ = strings.Cut(string(choice), "-")
			}
			if toastAdvance(t, f, "choose", map[string]string{"scope": level, "policy": policy}) != nil {
				t.Fatal("scope selection answered")
			}
			for f.phase == "scope" {
				if toastAdvance(t, f, "next", nil) != nil {
					t.Fatal("scope navigation answered")
				}
			}
			if len(backend.decisions) != 0 {
				t.Fatal("presentation decided")
			}
			action := "allow"
			if strings.HasPrefix(string(choice), "deny-") {
				action = "deny"
			}
			intent := toastAdvance(t, f, action, nil)
			if intent == nil || intent.RequestID != id || intent.Save != choice || intent.Approved != (action == "allow") {
				t.Fatalf("%+v", intent)
			}
			if _, err := f.Page(); err == nil {
				t.Fatal("consumed answer replayed")
			}
			current := selectReview(t, s, id, 2)
			if !f.Matches(*current) {
				t.Fatal("fresh token changed identical content")
			}
			result := s.Handle(context.Background(), Message{Version: 1, Sequence: 3, Action: "decide", RequestID: id, Token: current.Token, Approved: &intent.Approved, Save: intent.Save})
			if result.Type != "result" || result.Error != "" || len(backend.decisions) != 1 {
				t.Fatalf("%+v", result)
			}
		}
	}
}

func TestToastRefusesUnshownStaleBodyAndChangedSelections(t *testing.T) {
	s, backend, id := newReviewFixture()
	view := selectReview(t, s, id, 1)
	f, err := NewToastFlow(*view, false)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := f.Page()
	if _, err := f.Activate(p.Nonce+":next", nil); err == nil {
		t.Fatal("unshown page accepted")
	}
	_ = f.Displayed(p.Nonce)
	for _, action := range []string{"", id + ":allow", strings.Repeat("0", 64) + ":allow"} {
		if _, err := f.Activate(action, nil); err == nil {
			t.Fatal("body/forged activation accepted")
		}
	}
	if _, err := f.Activate(p.Nonce+":next", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Activate(p.Nonce+":next", nil); err == nil {
		t.Fatal("duplicate navigation accepted")
	}
	backend.requests[0].CapabilityRequest.Resource = "changed.example:443"
	if f.Matches(*selectReview(t, s, id, 2)) {
		t.Fatal("changed target matched")
	}
	if len(backend.decisions) != 0 {
		t.Fatal("invalid activation answered")
	}
}

func TestToastPagesPreserveLongHostileValuesWithoutXMLOrBidiInterpretation(t *testing.T) {
	long := strings.Repeat("日本語-<&>\"-", 100) + "\u202e\x00ms-resource:secret"
	pages, err := toastPages(map[string]string{"target": long}, true)
	if err != nil {
		t.Fatal(err)
	}
	var collected strings.Builder
	for _, page := range pages {
		lines := strings.Split(page, "\n")
		if len(lines) > 4 {
			t.Fatal("page exceeded line budget")
		}
		for _, line := range lines {
			collected.WriteString(line)
		}
		p := ToastPage{Title: "Review", Body: page, Nonce: strings.Repeat("a", 64), Buttons: []ToastButton{{"Next", strings.Repeat("a", 64) + ":next"}}}
		encoded, err := ToastXML(p)
		if err != nil {
			t.Fatal(err)
		}
		var decoded struct {
			Visual struct {
				Binding struct {
					Texts []string `xml:"text"`
				} `xml:"binding"`
			} `xml:"visual"`
		}
		if xml.Unmarshal([]byte(encoded), &decoded) != nil || len(decoded.Visual.Binding.Texts) != 2 || decoded.Visual.Binding.Texts[1] != "· "+strings.ReplaceAll(page, "\n", "\n· ") {
			t.Fatal("text changed")
		}
		if strings.ContainsRune(encoded, '\u202e') || strings.ContainsRune(encoded, '\x00') {
			t.Fatal("invisible authority formatting")
		}
	}
	if collected.String() != "target: "+printableToast(long) {
		t.Fatal("target truncated")
	}
}
