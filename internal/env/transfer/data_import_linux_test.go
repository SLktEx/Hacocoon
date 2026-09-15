//go:build linux

package environmenttransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"strings"
	"testing"
)

type dataImportFlow struct {
	*importFlow
	dataRead bool
}

func (f *dataImportFlow) CreateFromArchiveWithData(ctx context.Context, spec core.EnvironmentSpec, rootfs io.ReadSeeker, root string, limit int64, inputs []core.EnvironmentResourceImport) (core.Environment, error) {
	if len(inputs) != 1 || inputs[0].Key != "compiler" || inputs[0].Target != "/root/.cache/compiler" || inputs[0].Kind != "build-cache" {
		f.t.Fatal("lost data descriptor")
	}
	content, err := io.ReadAll(inputs[0].Archive)
	sum := sha256.Sum256(content)
	if err != nil || string(content) != "uncollected compiler" || inputs[0].Digest != hex.EncodeToString(sum[:]) {
		f.t.Fatal("wrong data bytes", err)
	}
	f.dataRead = true
	env, err := f.CreateFromArchive(ctx, spec, rootfs, root, limit)
	if err != nil {
		return env, err
	}
	env.Attachments = []core.EnvironmentAttachment{{Key: inputs[0].Key, Target: inputs[0].Target}}
	return env, nil
}

func TestPortableDataDispatchAndUnsupportedRefusal(t *testing.T) {
	ctx := context.Background()
	privateRoot := func(t *testing.T) string {
		t.Helper()
		root := t.TempDir()
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		return root
	}
	base, err := Stage(ctx, privateRoot(t), bytes.NewReader(importBundle(t, 2, 2, true)), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := base.Close(); err != nil {
			t.Error(err)
		}
	}()
	m := base.Manifest()
	parts := []io.Reader{}
	for _, c := range m.Components {
		reader, err := base.ComponentReader(c.Role)
		if err != nil {
			t.Fatal(err)
		}
		parts = append(parts, reader)
	}
	payload := "uncollected compiler"
	sum := sha256.Sum256([]byte(payload))
	m.Data = []Data{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache"}}
	m.Components = append(m.Components, Component{Role: dataRole(0), Bytes: int64(len(payload)), SHA256: hex.EncodeToString(sum[:])})
	parts = append(parts, strings.NewReader(payload))
	var bundle bytes.Buffer
	if err := Write(&bundle, m, parts, 1<<20); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"ok", "unsupported", "held"} {
		t.Run(mode, func(t *testing.T) {
			flow := &importFlow{t: t, mode: mode}
			withData := &dataImportFlow{importFlow: flow}
			i := Importer{Catalog: flow, Environments: withData, Workspaces: flow, Stores: flow, Root: privateRoot(t), StoreKind: "oci-containerd"}
			if mode == "unsupported" {
				i.Environments = flow
			}
			result, err := i.Import(ctx, bytes.NewReader(bundle.Bytes()), "imported", 1<<20)
			if mode == "unsupported" {
				if !errors.Is(err, core.ErrUnsupported) || len(flow.inputs) != 0 || withData.dataRead {
					t.Fatal("unsupported route created data", err)
				}
			} else if mode == "held" {
				if !errors.Is(err, core.ErrRecoveryRequired) || result.State != "cleanup-required" || len(flow.deleted) != 0 || !withData.dataRead {
					t.Fatal("unknown import lost retained data", result, err)
				}
			} else if err != nil || result.State != "running" || !withData.dataRead || !flow.started {
				t.Fatal(result, err)
			}
		})
	}
}
