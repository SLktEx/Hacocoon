//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func rootfsImportFixture(t *testing.T, metadata string, extra *tar.Header) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	put := func(h *tar.Header, data string) {
		t.Helper()
		h.Size = int64(len(data))
		if err := w.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, data); err != nil {
			t.Fatal(err)
		}
	}
	put(&tar.Header{Name: "metadata.yaml", Typeflag: tar.TypeReg, Mode: 0600}, metadata)
	put(&tar.Header{Name: "rootfs", Typeflag: tar.TypeDir, Mode: 0755}, "")
	put(&tar.Header{Name: "rootfs/data", Typeflag: tar.TypeReg, Mode: 0640, Uid: 123, Gid: 456, Xattrs: map[string]string{"user.test": "value"}}, "saved bytes")
	put(&tar.Header{Name: "rootfs/hard", Typeflag: tar.TypeLink, Linkname: "rootfs/data"}, "")
	put(&tar.Header{Name: "rootfs/link", Typeflag: tar.TypeSymlink, Linkname: "/data"}, "")
	put(&tar.Header{Name: "templates", Typeflag: tar.TypeDir, Mode: 0755}, "")
	put(&tar.Header{Name: "templates/unsafe.tpl", Typeflag: tar.TypeReg, Mode: 0600}, "must not replay")
	if extra != nil {
		put(extra, "")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const rootfsMetadataFixture = `architecture: x86_64
creation_date: 123
expiry_date: 456
properties:
  source: old
templates:
  /target:
    when: [create]
    template: unsafe.tpl
`

func TestRootfsImportPreparationPreservesDataAndReplacesImageAuthority(t *testing.T) {
	source := rootfsImportFixture(t, rootfsMetadataFixture, nil)
	original := append([]byte(nil), source...)
	var digests []string
	for _, owner := range []string{strings.Repeat("a", 32), strings.Repeat("b", 32)} {
		a, err := prepareRootfsImport(context.Background(), bytes.NewReader(source), privateRootfsImportDir(t), 1<<20, owner)
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		digests = append(digests, a.Digest())
		r := tar.NewReader(a.Reader())
		seen := map[string]bool{}
		for {
			h, err := r.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(r)
			if err != nil {
				t.Fatal(err)
			}
			seen[h.Name] = true
			switch h.Name {
			case "metadata.yaml":
				var m rootfsImportMetadata
				if yaml.UnmarshalStrict(data, &m) != nil || m.Architecture != "x86_64" || m.CreationDate != 123 || m.ExpiryDate != 0 || len(m.Properties) != 1 || m.Properties[importImageOwnerKey] != owner || len(m.Templates) != 0 {
					t.Fatal("metadata authority inherited", m)
				}
			case "rootfs/data":
				if string(data) != "saved bytes" || h.Mode != 0640 || h.Uid != 123 || h.Gid != 456 || h.Xattrs["user.test"] != "value" {
					t.Fatal("rootfs content/attributes lost")
				}
			case "rootfs/hard":
				if h.Linkname != "rootfs/data" || h.Typeflag != tar.TypeLink {
					t.Fatal("hardlink lost")
				}
			case "rootfs/link":
				if h.Linkname != "/data" || h.Typeflag != tar.TypeSymlink {
					t.Fatal("symlink lost")
				}
			case "rootfs":
			default:
				t.Fatal("non-rootfs payload copied", h.Name)
			}
		}
		if len(seen) != 5 {
			t.Fatal("missing rootfs entries", seen)
		}
	}
	if digests[0] == digests[1] || !bytes.Equal(source, original) {
		t.Fatal("transport identity reused or saved archive changed")
	}
}
func TestRootfsImportPreparationRejectsMalformedArchives(t *testing.T) {
	for _, mode := range []string{"duplicate", "traversal", "symlink-child", "hardlink-outside", "metadata-duplicate", "unsupported-arch", "tail", "truncated", "limit", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			var extra *tar.Header
			metadata := rootfsMetadataFixture
			switch mode {
			case "duplicate":
				extra = &tar.Header{Name: "rootfs/data", Typeflag: tar.TypeReg}
			case "traversal":
				extra = &tar.Header{Name: "rootfs/../outside", Typeflag: tar.TypeReg}
			case "symlink-child":
				extra = &tar.Header{Name: "rootfs/link/child", Typeflag: tar.TypeReg}
			case "hardlink-outside":
				extra = &tar.Header{Name: "rootfs/bad", Typeflag: tar.TypeLink, Linkname: "templates/unsafe.tpl"}
			case "metadata-duplicate":
				metadata += "architecture: aarch64" + string(byte(10))
			case "unsupported-arch":
				metadata = strings.Replace(metadata, "x86_64", "unsupported", 1)
			}
			input := rootfsImportFixture(t, metadata, extra)
			limit := int64(1 << 20)
			if mode == "tail" {
				input = append(input, 1)
			}
			if mode == "truncated" {
				input = input[:len(input)-512]
			}
			if mode == "limit" {
				limit = 1024
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			root := privateRootfsImportDir(t)
			a, err := prepareRootfsImport(ctx, bytes.NewReader(input), root, limit, strings.Repeat("a", 32))
			if err == nil {
				a.Close()
				t.Fatal("unsafe image accepted")
			}
			files, err := os.ReadDir(root)
			if err != nil || len(files) != 0 {
				t.Fatal("preparation left named files", err)
			}
		})
	}
}

func privateRootfsImportDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRootfsImportNormalizesIncusArchitectureAliases(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"x86_64", "x86_64"}, {"amd64", "x86_64"}, {"generic_64", "x86_64"},
		{"aarch64", "aarch64"}, {"arm64", "aarch64"}, {"arm64_generic", "aarch64"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			original := rootfsImportFixture(t, strings.Replace(rootfsMetadataFixture, "x86_64", tc.input, 1), nil)
			a, err := prepareRootfsImport(context.Background(), bytes.NewReader(original), privateRootfsImportDir(t), 1<<20, strings.Repeat("a", 32))
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			reader := tar.NewReader(a.Reader())
			h, err := reader.Next()
			if err != nil || h.Name != "metadata.yaml" {
				t.Fatal("missing metadata", err)
			}
			raw, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}
			var metadata rootfsImportMetadata
			if err := yaml.UnmarshalStrict(raw, &metadata); err != nil || metadata.Architecture != tc.want {
				t.Fatalf("metadata=%+v err=%v", metadata, err)
			}
		})
	}
}
