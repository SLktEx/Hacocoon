package main

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"strings"
	"testing"
)

type baseManageFake struct {
	images  []basemanage.Image
	deleted []basemanage.Identity
}

func (f *baseManageFake) ListBaseImages(context.Context) ([]basemanage.Image, error) {
	return f.images, nil
}
func (f *baseManageFake) DeleteBaseImage(_ context.Context, id basemanage.Identity) error {
	f.deleted = append(f.deleted, id)
	return nil
}
func TestBaseCleanupCLIConfirmsExactRevision(t *testing.T) {
	id := basemanage.Identity{Name: "tools", Fingerprint: strings.Repeat("a", 64), BuildInstance: "env-" + strings.Repeat("b", 32)}
	for _, mode := range []string{"confirm", "decline", "busy", "prefix", "ambiguous", "json"} {
		t.Run(mode, func(t *testing.T) {
			f := &baseManageFake{images: []basemanage.Image{{Identity: id, Current: true, IndependentSnapshots: []string{"saved"}}}}
			args := []string{"delete", "tools"}
			answer := "yes\n"
			switch mode {
			case "decline":
				answer = "n\n"
			case "busy":
				f.images[0].Environments = []string{"dev"}
			case "prefix":
				args = []string{"delete", "--yes", id.Fingerprint[:12]}
			case "ambiguous":
				args = []string{"delete", id.Fingerprint[:8]}
				v := f.images[0]
				v.Fingerprint = id.Fingerprint[:8] + strings.Repeat("c", 56)
				f.images = append(f.images, v)
			case "json":
				args = []string{"list", "--all", "--json"}
			}
			var out, diag bytes.Buffer
			code := baseManageCommand(context.Background(), f, args, strings.NewReader(answer), &out, &diag)
			deleteExpected := mode == "confirm" || mode == "prefix"
			if deleteExpected {
				if code != 0 || len(f.deleted) != 1 || f.deleted[0] != id {
					t.Fatal(code, f.deleted, diag.String())
				}
			} else if len(f.deleted) != 0 {
				t.Fatal("unexpected delete", f.deleted)
			}
			if mode == "json" && (code != 0 || !strings.Contains(out.String(), "independent_snapshots")) {
				t.Fatal(out.String(), diag.String())
			}
			if (mode == "decline" || mode == "busy" || mode == "ambiguous") && code == 0 {
				t.Fatal("false success")
			}
		})
	}
}
