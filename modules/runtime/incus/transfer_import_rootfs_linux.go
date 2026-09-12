//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"math"
	"os"
	"path"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"gopkg.in/yaml.v2"
)

const importImageOwnerKey = "user.hacocoon.import-owner"

type rootfsImportMetadata struct {
	Architecture string            `yaml:"architecture"`
	CreationDate int64             `yaml:"creation_date"`
	ExpiryDate   int64             `yaml:"expiry_date"`
	Properties   map[string]string `yaml:"properties"`
	Templates    map[string]any    `yaml:"templates"`
}

// prepareRootfsImport preserves rootfs entries while replacing image metadata,
// not the saved archive. A fresh owner changes the transport fingerprint, so a
// concurrent/repeated import cannot relabel an existing content-addressed image.
// Templates are not replayed against the newly configured Environment.
func prepareRootfsImport(ctx context.Context, source io.ReadSeeker, root string, limit int64, owner string) (*NativeArchive, error) {
	if source == nil || limit <= 0 || limit == math.MaxInt64 ||
		!core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: owner}) {
		return nil, core.ErrInvalidArgument
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return captureNativeArchive(ctx, root, limit, func(output string) error {
		file, err := os.OpenFile(output, os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer file.Close()
		counted := &volumeImportCountReader{reader: &archiveContextReader{ctx, io.LimitReader(source, limit+1)}}
		input := tar.NewReader(counted)
		h, err := input.Next()
		if err != nil {
			return err
		}
		if h.Name != "metadata.yaml" || h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > 64<<10 {
			return core.ErrIncompatibleState
		}
		raw, err := io.ReadAll(input)
		if err != nil {
			return err
		}
		var old rootfsImportMetadata
		if yaml.UnmarshalStrict(raw, &old) != nil {
			return core.ErrIncompatibleState
		}
		if old.Architecture != "x86_64" && old.Architecture != "aarch64" {
			return core.ErrUnsupported
		}
		fresh := rootfsImportMetadata{Architecture: old.Architecture, CreationDate: old.CreationDate,
			Properties: map[string]string{importImageOwnerKey: owner}, Templates: map[string]any{}}
		raw, err = yaml.Marshal(fresh)
		if err != nil {
			return err
		}
		writer := tar.NewWriter(&boundedImageWriter{ctx: ctx, file: file, remaining: limit})
		if err := writer.WriteHeader(&tar.Header{Name: "metadata.yaml", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(raw))}); err != nil {
			return err
		}
		if _, err := writer.Write(raw); err != nil {
			return err
		}
		seen := map[string]byte{}
		previousSize := h.Size
		for {
			before := counted.n
			h, err = input.Next()
			if err == io.EOF {
				if counted.n-before != (512-previousSize%512)%512+1024 {
					return core.ErrIncompatibleState
				}
				tail, readErr := io.ReadAll(io.LimitReader(counted, 10241))
				if readErr != nil || counted.n > limit || len(tail) > 10240 || !bytes.Equal(tail, make([]byte, len(tail))) {
					return core.ErrIncompatibleState
				}
				break
			}
			if err != nil {
				return err
			}
			previousSize = h.Size
			name := strings.TrimSuffix(h.Name, "/")
			template := name == "templates" || strings.HasPrefix(name, "templates/")
			if len(name) > 4096 || path.Clean(name) != name || !(name == "rootfs" || strings.HasPrefix(name, "rootfs/") || template) || len(seen) >= 1000000 {
				return core.ErrIncompatibleState
			}
			if _, exists := seen[name]; exists {
				return core.ErrIncompatibleState
			}
			for parent := path.Dir(name); parent != "."; parent = path.Dir(parent) {
				if kind, ok := seen[parent]; !ok || kind != tar.TypeDir {
					return core.ErrIncompatibleState
				}
			}
			if (name == "rootfs" || name == "templates") && h.Typeflag != tar.TypeDir {
				return core.ErrIncompatibleState
			}
			switch h.Typeflag {
			case tar.TypeReg, tar.TypeDir, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			case tar.TypeLink:
				target := strings.TrimSuffix(h.Linkname, "/")
				kind, exists := seen[target]
				if !exists || (kind != tar.TypeReg && kind != tar.TypeLink) || strings.HasPrefix(target, "templates/") != template {
					return core.ErrIncompatibleState
				}
			default:
				return core.ErrUnsupported
			}
			seen[name] = h.Typeflag
			if template {
				// Image templates are metadata outside rootfs; retaining the source
				// bundle does not require executing/copying them into the new image.
				if _, err := io.Copy(io.Discard, input); err != nil {
					return err
				}
				continue
			}
			if err := writer.WriteHeader(h); err != nil {
				return err
			}
			if _, err := io.Copy(writer, input); err != nil {
				return err
			}
		}
		if seen["rootfs"] != tar.TypeDir {
			return core.ErrIncompatibleState
		}
		if err := writer.Close(); err != nil {
			return err
		}
		return file.Close()
	})
}
