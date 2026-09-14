package vscode

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/experimental"
)

var testNow = time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

func release(version string, days int, pre bool, platform string) Release {
	r := Release{Version: version, LastUpdated: testNow.Add(-time.Duration(days) * 24 * time.Hour).Format(time.RFC3339Nano), TargetPlatform: platform}
	if pre {
		r.Properties = append(r.Properties, struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}{"Microsoft.VisualStudio.Code.PreRelease", "true"})
	}
	return r
}

func TestSelectionPolicy(t *testing.T) {
	releases := []Release{release("1.9.0", 45, false, ""), release("1.12.0", 30, false, "linux-x64"), release("1.13.0", 10, false, ""), release("1.14.0", 31, true, ""), release("9.0.0", 60, false, "win32-x64")}
	for _, tc := range []struct{ name, age, global, override, pin, want string }{
		{"default", "", "", "", "", "1.12.0"},
		{"allow", "30d", "allow", "", "", "1.14.0"},
		{"override allow", "30d", "deny", "allow", "", "1.14.0"},
		{"override deny", "30d", "allow", "deny", "", "1.12.0"},
		{"young", "0d", "deny", "", "", "1.13.0"},
		{"pin young", "90d", "deny", "", "1.13.0", "1.13.0"},
		{"pin pre", "90d", "deny", "deny", "1.14.0", "1.14.0"},
		{"none", "90d", "deny", "", "", ""},
		{"missing pin", "", "", "", "2.0.0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Select(releases, experimental.Extension{ID: "a.b", Version: tc.pin, PreRelease: tc.override}, experimental.Extensions{MinReleaseAge: tc.age, PreRelease: tc.global}, "linux-x64", testNow)
			if tc.want == "" {
				if err == nil {
					t.Fatal("expected no candidate")
				}
				return
			}
			if err != nil || r.Version != tc.want {
				t.Fatalf("%+v %v", r, err)
			}
		})
	}
}

func TestSelectionPlatformDateAndMalformedMetadata(t *testing.T) {
	r, err := Select([]Release{release("2.0.0", 60, false, ""), release("2.0.0", 2, false, "linux-x64"), release("1.0.0", 30, false, "")}, experimental.Extension{ID: "a.b"}, experimental.Extensions{}, "linux-x64", testNow)
	if err != nil || r.Version != "1.0.0" {
		t.Fatal(r, err)
	}
	for _, releases := range [][]Release{
		{{Version: "1.0.0", LastUpdated: "not-a-date"}}, {release("1.0.0", -1, false, "")},
		{release("--evil", 40, false, "")}, {release("1.0.0", 40, false, ""), release("1.0.0", 40, false, "")},
	} {
		if _, err := Select(releases, experimental.Extension{ID: "a.b"}, experimental.Extensions{}, "linux-x64", testNow); err == nil {
			t.Fatal("accepted invalid metadata")
		}
	}
}

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func gallery(t *testing.T, f func(string) string) Gallery {
	t.Helper()
	return Gallery{Client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != galleryURL || r.Method != "POST" {
			t.Fatal("unexpected request")
		}
		var request struct {
			Flags   int
			Filters []struct{ Criteria []struct{ Value string } }
		}
		if json.NewDecoder(r.Body).Decode(&request) != nil || request.Flags != 17 {
			t.Fatal("latest-only query or invalid request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(f(request.Filters[0].Criteria[0].Value)))}, nil
	})}}
}

func payload(id string, versions []Release) string {
	parts := strings.Split(id, ".")
	b, _ := json.Marshal(map[string]any{"results": []any{map[string]any{"extensions": []any{map[string]any{"publisher": map[string]any{"publisherName": parts[0]}, "extensionName": parts[1], "versions": versions}}}}})
	return string(b)
}

func TestGalleryIdentityAndDependencyPolicy(t *testing.T) {
	g := gallery(t, func(id string) string {
		r := release("1.0.0", 30, false, "")
		if id == "a.parent" {
			r.Properties = append(r.Properties, struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{"Microsoft.VisualStudio.Code.ExtensionDependencies", "a.child"})
		}
		return payload(id, []Release{release("2.0.0", 1, false, ""), r})
	})
	plan, err := Resolve(context.Background(), g, experimental.VSCode{Extensions: experimental.Extensions{Install: []experimental.Extension{{ID: "a.parent"}}}}, "linux-x64", testNow)
	if err != nil || len(plan) != 2 || plan[0].ID != "a.child" || plan[0].Version != "1.0.0" || plan[1].Version != "1.0.0" {
		t.Fatal(plan, err)
	}
	for _, response := range []string{`{}`, `{"results":[]}`, payload("wrong.extension", []Release{release("1.0.0", 40, false, "")})} {
		if _, err := gallery(t, func(string) string { return response }).Releases(context.Background(), "a.b"); err == nil {
			t.Fatal("accepted gallery mismatch")
		}
	}
	g = gallery(t, func(id string) string { return payload(id, []Release{release("1.0.0", 1, false, "")}) })
	if _, err := Resolve(context.Background(), g, experimental.VSCode{Extensions: experimental.Extensions{Install: []experimental.Extension{{ID: "a.parent"}}}}, "linux-x64", testNow); err == nil {
		t.Fatal("young dependency accepted")
	}
}
