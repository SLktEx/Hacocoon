package workspaceinput

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"
)

func TestTreeInputRefusesAliasesPrivilegesAndExtraPayload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header *tar.Header
		tail   string
	}{
		{"traversal", &tar.Header{Name: "tree/../outside", Typeflag: tar.TypeReg, Mode: 0600}, ""},
		{"missing-parent", &tar.Header{Name: "tree/missing/child", Typeflag: tar.TypeReg, Mode: 0600}, ""},
		{"hardlink", &tar.Header{Name: "tree/alias", Typeflag: tar.TypeLink, Linkname: "tree/.git/config", Mode: 0600}, ""},
		{"absolute-link", &tar.Header{Name: "tree/alias", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0777}, ""},
		{"outside-link", &tar.Header{Name: "tree/alias", Typeflag: tar.TypeSymlink, Linkname: "../outside", Mode: 0777}, ""},
		{"git-link", &tar.Header{Name: "tree/.git/alias", Typeflag: tar.TypeSymlink, Linkname: "config", Mode: 0777}, ""},
		{"duplicate", &tar.Header{Name: "tree/.git/config", Typeflag: tar.TypeReg, Mode: 0600}, ""},
		{"setuid", &tar.Header{Name: "tree/tool", Typeflag: tar.TypeReg, Mode: 04755}, ""},
		{"device", &tar.Header{Name: "tree/device", Typeflag: tar.TypeChar, Mode: 0600}, ""},
		{"xattr", &tar.Header{Name: "tree/xattr", Typeflag: tar.TypeReg, Mode: 0600, PAXRecords: map[string]string{"SCHILY.xattr.security.capability": "forged"}}, ""},
		{"trailing", nil, "unexpected"},
		{"confined-link", &tar.Header{Name: "tree/alias", Typeflag: tar.TypeSymlink, Linkname: ".git/HEAD", Mode: 0777}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			w := tar.NewWriter(&b)
			for _, h := range []*tar.Header{
				{Name: "tree", Typeflag: tar.TypeDir, Mode: 0700},
				{Name: "tree/.git", Typeflag: tar.TypeDir, Mode: 0700},
				{Name: "tree/.git/HEAD", Typeflag: tar.TypeReg, Mode: 0600},
				{Name: "tree/.git/config", Typeflag: tar.TypeReg, Mode: 0600},
				tc.header,
			} {
				if h != nil {
					if err := w.WriteHeader(h); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := w.Close(); err != nil {
				t.Fatal(err)
			}
			b.WriteString(tc.tail)
			err := CopyTree(context.Background(), &b, func(_ *tar.Header, r io.Reader) error { _, err := io.Copy(io.Discard, r); return err })
			if (err == nil) != (tc.name == "confined-link") {
				t.Fatal("unexpected input acceptance", err)
			}
		})
	}
}
