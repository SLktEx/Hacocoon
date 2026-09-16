//go:build linux

package environmenttransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"testing"
)

func TestStagedComponentReadersStayInsideVerifiedArchives(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	m := Manifest{Version: 1, Source: "dev", HasOCI: true}
	roles := []string{"rootfs", "workspace", "workspace-002", "oci"}
	sizes := []int{513, 1024, 1, 777}
	parts := []io.Reader{}
	payloads := [][]byte{}
	for i, role := range roles {
		data := bytes.Repeat([]byte{byte('a' + i)}, sizes[i])
		sum := sha256.Sum256(data)
		m.Components = append(m.Components, Component{Role: role, Bytes: int64(len(data)), SHA256: hex.EncodeToString(sum[:])})
		payloads = append(payloads, data)
		parts = append(parts, bytes.NewReader(data))
	}
	var encoded bytes.Buffer
	if err := Write(&encoded, m, parts, 4096); err != nil {
		t.Fatal(err)
	}
	staged, err := Stage(context.Background(), root, bytes.NewReader(encoded.Bytes()), 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	// The caller's input and manifest cannot redirect a component view.
	for i := range encoded.Bytes() {
		encoded.Bytes()[i] = 0
	}
	reported := staged.Manifest()
	reported.Components[0].Role = "oci"
	for i, role := range roles {
		reader, err := staged.ComponentReader(role)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := reader.(io.Writer); ok {
			t.Fatal("writable archive exposed")
		}
		if _, ok := reader.(interface {
			Outer() (io.ReaderAt, int64, int64)
		}); ok {
			t.Fatal("outer archive exposed")
		}
		second, err := staged.ComponentReader(role)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := reader.Seek(-1, io.SeekEnd); err != nil {
			t.Fatal(err)
		}
		tail, err := io.ReadAll(reader)
		if err != nil || !bytes.Equal(tail, payloads[i][len(payloads[i])-1:]) {
			t.Fatal(role, err, tail)
		}
		data, err := io.ReadAll(second)
		if err != nil || !bytes.Equal(data, payloads[i]) {
			t.Fatal("independent complete read", role, err)
		}
		if _, err := reader.Seek(int64(sizes[i]+1024), io.SeekStart); err != nil {
			t.Fatal(err)
		}
		if data, err := io.ReadAll(reader); err != nil || len(data) != 0 {
			t.Fatal("crossed component boundary", role, err)
		}
		if _, err := reader.Seek(-1, io.SeekStart); err == nil {
			t.Fatal("negative seek accepted")
		}
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		data, err = io.ReadAll(reader)
		if err != nil || !bytes.Equal(data, payloads[i]) {
			t.Fatal("rewind changed bytes", role, err)
		}
	}
	for _, role := range []string{"", "manifest.json", "rootfs.tar", "../oci", "workspace-003"} {
		if reader, err := staged.ComponentReader(role); err == nil || reader != nil {
			t.Fatal("unknown component accepted", role)
		}
	}
	reader, err := staged.ComponentReader("rootfs")
	if err != nil {
		t.Fatal(err)
	}
	if err := staged.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Read(make([]byte, 1)); err == nil {
		t.Fatal("closed bundle remained readable")
	}
}

func TestUnverifiedStagingCannotExposeComponents(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	input := stageFixture(t)
	// All component payloads exist, but the closing blocks are incomplete.
	staged, err := Stage(context.Background(), root, bytes.NewReader(input[:len(input)-512]), 1024)
	if err == nil || staged != nil {
		t.Fatal("incomplete bundle published", err)
	}
	if r, err := staged.ComponentReader("rootfs"); err == nil || r != nil {
		t.Fatal("nil staging reader")
	}
	if r, err := new(Staged).ComponentReader("rootfs"); err == nil || r != nil {
		t.Fatal("zero staging reader")
	}
}
