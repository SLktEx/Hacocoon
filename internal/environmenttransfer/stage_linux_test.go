//go:build linux

package environmenttransfer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func stageFixture(t *testing.T) []byte {
	t.Helper()
	m, parts := fixture(true)
	var out bytes.Buffer
	if err := Write(&out, m, parts, 1024); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func TestStageOwnsReadOnlyUnnamedBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	original := stageFixture(t)
	source := bytes.NewReader(original)
	staged, err := Stage(context.Background(), root, source, 1024)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Close()
	for i := range original {
		original[i] = 0
	}
	m := staged.Manifest()
	m.Components[0].Role = "changed"
	if staged.Manifest().Components[0].Role != "rootfs" {
		t.Fatal("manifest mutation leaked")
	}
	for i := 0; i < 2; i++ {
		got, err := Inspect(staged.Reader(), 1024)
		if err != nil || !got.HasOCI {
			t.Fatal(got, err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("staging exposed a path", err)
	}
	var st unix.Stat_t
	if err := unix.Fstat(int(staged.file.Fd()), &st); err != nil || st.Nlink != 0 {
		t.Fatal("file remained named", err)
	}
	if _, err := staged.file.WriteAt([]byte("x"), 0); err == nil {
		t.Fatal("writable descriptor escaped")
	}
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(moved, root)
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(root)
	if _, err := Inspect(staged.Reader(), 1024); err != nil {
		t.Fatal("directory replacement changed bytes", err)
	}
}
func TestStageRefusesUnsafeInputsWithoutNamedRemnants(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	good := stageFixture(t)
	for _, input := range [][]byte{nil, good[:len(good)-1024], bytes.Repeat([]byte("x"), envelopeOverhead+1025)} {
		if s, err := Stage(context.Background(), root, bytes.NewReader(input), 1024); err == nil || s != nil {
			t.Fatal("invalid staging accepted", err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 0 {
			t.Fatal("failed staging left named data", err)
		}
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if s, err := Stage(context.Background(), link, bytes.NewReader(good), 1024); err == nil || s != nil {
		t.Fatal("symlink accepted")
	}
	os.Chmod(root, 0755)
	if s, err := Stage(context.Background(), root, bytes.NewReader(good), 1024); err == nil || s != nil {
		t.Fatal("nonprivate directory accepted")
	}
	os.Chmod(root, 0700)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Stage(ctx, root, bytes.NewReader(good), 1024); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestStageClosedReaderFails(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	s, err := Stage(context.Background(), root, bytes.NewReader(stageFixture(t)), 1024)
	if err != nil {
		t.Fatal(err)
	}
	reader := s.Reader()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(reader); err == nil {
		t.Fatal("closed staging readable")
	}
}

func TestStageMaximumBudgetDoesNotOverflow(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	s, err := Stage(context.Background(), root, bytes.NewReader(stageFixture(t)), (1<<63-1)-envelopeOverhead)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

type stageErrorReader struct {
	data    *bytes.Reader
	failure error
}

func (r *stageErrorReader) Read(p []byte) (int, error) {
	n, err := r.data.Read(p)
	if err == io.EOF {
		return n, r.failure
	}
	return n, err
}
func TestStageRejectsTransportFailureAfterCompleteBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("transport failed")
	s, err := Stage(context.Background(), root, &stageErrorReader{bytes.NewReader(stageFixture(t)), failure}, 1024)
	if s != nil || !errors.Is(err, failure) {
		t.Fatal(s, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
