//go:build linux

package incus

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/workspace/input"
	"gopkg.in/yaml.v2"
)

// Portable tree input becomes an ordinary Incus volume archive only inside this
// adapter. The existing importer validates it and supplies fresh native ownership.
func (b *RepositoryBackend) ImportWorkspaceTreeVolume(ctx context.Context, object gitrepo.Object, source io.Reader) error {
	pool, name, err := volumeRef(object)
	if err != nil {
		return err
	}
	archive, err := captureNativeArchive(ctx, b.ImportRoot, b.ImportLimit, func(output string) (err error) {
		f, err := os.OpenFile(output, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		w := tar.NewWriter(&boundedImageWriter{ctx: ctx, file: f, remaining: b.ImportLimit})
		index := volumeImportIndex{Name: name, Pool: pool, Backend: "btrfs", Type: "custom"}
		index.Config.Volume = &volumeImportMetadata{Name: name, Type: "custom", ContentType: "filesystem", Config: map[string]string{}}
		raw, err := yaml.Marshal(index)
		if err != nil {
			return err
		}
		if err = w.WriteHeader(&tar.Header{Name: "backup/index.yaml", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(raw))}); err != nil {
			return err
		}
		if _, err = w.Write(raw); err != nil {
			return err
		}
		if err = workspaceinput.CopyTree(ctx, source, func(h *tar.Header, r io.Reader) error {
			copy := *h
			copy.Name = "backup/volume" + strings.TrimPrefix(h.Name, "tree")
			copy.Uid = 0
			copy.Gid = 0
			copy.Uname = ""
			copy.Gname = ""
			copy.PAXRecords = nil
			if err := w.WriteHeader(&copy); err != nil {
				return err
			}
			_, err := io.Copy(w, r)
			return err
		}); err != nil {
			return err
		}
		if err = w.Close(); err != nil {
			return err
		}
		return f.Close()
	})
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()
	return b.ImportWorkspaceVolume(ctx, object, io.NewSectionReader(archive.file, 0, archive.size))
}
