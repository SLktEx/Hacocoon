//go:build linux

package incus

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"gopkg.in/yaml.v2"
)

const volumeImportIndexLimit = 64 << 10

type volumeImportIndex struct {
	Name            string   `yaml:"name"`
	Backend         string   `yaml:"backend"`
	Pool            string   `yaml:"pool"`
	Optimized       bool     `yaml:"optimized"`
	OptimizedHeader bool     `yaml:"optimized_header"`
	Type            string   `yaml:"type"`
	Snapshots       []string `yaml:"snapshots,omitempty"`
	Config          struct {
		Volume *volumeImportMetadata `yaml:"volume"`
	} `yaml:"config"`
}
type volumeImportMetadata struct {
	Config      map[string]string `yaml:"config"`
	Description string            `yaml:"description"`
	Name        string            `yaml:"name"`
	Type        string            `yaml:"type"`
	UsedBy      []string          `yaml:"used_by"`
	Location    string            `yaml:"location"`
	ContentType string            `yaml:"content_type"`
	Project     string            `yaml:"project"`
	CreatedAt   string            `yaml:"created_at"`
}

// Import requires the caller's existing creating record to precede this call.
// New ownership is placed in native metadata before Incus creates the volume;
// there is no interval where saved ownership is adopted as destination authority.
func (b *PersistentResourceBackend) Import(ctx context.Context, r core.PersistentResource, source io.ReadSeeker) (resultErr error) {
	pool, name, err := persistentVolume(r)
	if err != nil {
		return err
	}
	if r.SourceOnly || r.State != "creating" || source == nil {
		return core.ErrInvalidArgument
	}
	observed, err := b.observe(ctx, r)
	if err != nil {
		return err
	}
	if observed != nil {
		return core.ErrAlreadyExists
	}
	config := map[string]string{"user.hacocoon.owner": r.Owner, "user.hacocoon.resource": r.ID, "user.hacocoon.kind": r.Kind, "user.hacocoon.source-only": strconv.FormatBool(r.SourceOnly)}
	archive, err := prepareVolumeImport(ctx, source, b.ImportRoot, b.ImportLimit, b.Runtime.project, pool, name, config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, archive.Close()) }()
	output, err := b.Runtime.runner.Run(ctx, "incus", "storage", "volume", "import", pool, fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), archive.file.Fd()), name, "--project", b.Runtime.project)
	if err != nil || output.ExitCode != 0 {
		return errors.Join(core.ErrRecoveryRequired, err)
	}
	// The canonical service verifies and commits the already recorded identity.
	return nil
}

func prepareVolumeImport(ctx context.Context, source io.ReadSeeker, root string, limit int64, project, pool, name string, config map[string]string) (*NativeArchive, error) {
	if source == nil || !safeIncusRef(project) || !safeIncusRef(pool) || !safeIncusRef(name) || limit <= 0 || limit == math.MaxInt64 {
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
		if h.Name != "backup/index.yaml" || h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > volumeImportIndexLimit {
			return core.ErrIncompatibleState
		}
		raw, err := io.ReadAll(input)
		if err != nil {
			return err
		}
		var index volumeImportIndex
		if yaml.UnmarshalStrict(raw, &index) != nil || index.Type != "custom" || index.Backend != "btrfs" || index.Optimized || index.OptimizedHeader || len(index.Snapshots) != 0 || index.Config.Volume == nil {
			return core.ErrUnsupported
		}
		old := index.Config.Volume
		if old.Type != "custom" || old.ContentType != "filesystem" || old.Name != index.Name {
			return core.ErrIncompatibleState
		}
		fresh := make(map[string]string, len(config)+2)
		for k, v := range config {
			fresh[k] = v
		}
		for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
			if value := old.Config[key]; value != "" {
				if !validImportedIDMap(value) {
					return core.ErrIncompatibleState
				}
				fresh[key] = value
			}
		}
		// Incus requires the mapping of the archived numeric IDs, not old ownership,
		// permissions, source-only settings or snapshot identities.
		index.Name = name
		index.Pool = pool
		index.Backend = "btrfs"
		index.Config.Volume = &volumeImportMetadata{Name: name, Type: "custom", ContentType: "filesystem", Project: project, Config: fresh, UsedBy: []string{}, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
		raw, err = yaml.Marshal(index)
		if err != nil || len(raw) > volumeImportIndexLimit {
			return core.ErrInvalidArgument
		}
		writer := tar.NewWriter(&boundedImageWriter{ctx: ctx, file: file, remaining: limit})
		if err := writer.WriteHeader(&tar.Header{Name: "backup/index.yaml", Typeflag: tar.TypeReg, Mode: 0600, Size: int64(len(raw))}); err != nil {
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
			if len(name) > 4096 || path.Clean(name) != name || !(name == "backup/volume" || strings.HasPrefix(name, "backup/volume/")) || len(seen) >= 1000000 {
				return core.ErrIncompatibleState
			}
			if _, ok := seen[name]; ok {
				return core.ErrIncompatibleState
			}
			for parent := path.Dir(name); parent != "backup" && parent != "."; parent = path.Dir(parent) {
				if kind, ok := seen[parent]; !ok || kind != tar.TypeDir {
					return core.ErrIncompatibleState
				}
			}
			if name == "backup/volume" && h.Typeflag != tar.TypeDir {
				return core.ErrIncompatibleState
			}
			switch h.Typeflag {
			case tar.TypeReg, tar.TypeDir, tar.TypeSymlink, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
			case tar.TypeLink:
				target := strings.TrimSuffix(h.Linkname, "/")
				kind, ok := seen[target]
				if !ok || (kind != tar.TypeReg && kind != tar.TypeLink) {
					return core.ErrIncompatibleState
				}
			default:
				return core.ErrUnsupported
			}
			seen[name] = h.Typeflag
			if err := writer.WriteHeader(h); err != nil {
				return err
			}
			if _, err := io.Copy(writer, input); err != nil {
				return err
			}
		}
		if seen["backup/volume"] != tar.TypeDir {
			return core.ErrIncompatibleState
		}
		if err := writer.Close(); err != nil {
			return err
		}
		return file.Close()
	})
}

func validImportedIDMap(value string) bool {
	if len(value) > volumeImportIndexLimit {
		return false
	}
	var entries []struct {
		Isuid, Isgid           bool
		Hostid, Nsid, Maprange int64
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&entries) != nil || len(entries) == 0 || len(entries) > 128 {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	for _, entry := range entries {
		if (!entry.Isuid && !entry.Isgid) || entry.Hostid < 0 || entry.Nsid < 0 || entry.Maprange <= 0 || entry.Hostid > 4294967295-entry.Maprange || entry.Nsid > 4294967295-entry.Maprange {
			return false
		}
	}
	return true
}

type volumeImportCountReader struct {
	reader io.Reader
	n      int64
}

func (r *volumeImportCountReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}
