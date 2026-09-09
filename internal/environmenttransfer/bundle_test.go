package environmenttransfer

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"testing"
)

func fixture(oci bool) (Manifest, []io.Reader) {
	m := Manifest{Version: 1, Source: "dev", HasOCI: oci}
	roles := []string{"rootfs", "workspace"}
	if oci {
		roles = append(roles, "oci")
	}
	parts := []io.Reader{}
	for _, role := range roles {
		payload := []byte("native " + role + " archive")
		hash := sha256.Sum256(payload)
		m.Components = append(m.Components, Component{role, int64(len(payload)), hex.EncodeToString(hash[:])})
		parts = append(parts, bytes.NewReader(payload))
	}
	return m, parts
}
func TestBundleRoundTrip(t *testing.T) {
	for _, oci := range []bool{false, true} {
		m, parts := fixture(oci)
		var out bytes.Buffer
		if err := Write(&out, m, parts, 1024); err != nil {
			t.Fatal(err)
		}
		got, err := Inspect(bytes.NewReader(out.Bytes()), 1024)
		if err != nil || got.Source != m.Source || got.HasOCI != oci {
			t.Fatal(got, err)
		}
		if _, err := Inspect(bytes.NewReader(out.Bytes()), 1); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("budget ignored", err)
		}
		if _, err := Inspect(bytes.NewReader(append(append([]byte{}, out.Bytes()...), 0)), 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("trailing bytes accepted", err)
		}
	}
}
func TestBundleRejectsIncompleteAndChangedStreams(t *testing.T) {
	m, parts := fixture(true)
	var out bytes.Buffer
	if err := Write(&out, m, parts, 1024); err != nil {
		t.Fatal(err)
	}
	good := out.Bytes()
	for _, cut := range []int{0, 512, 1024, len(good) - 1, len(good) - 512, len(good) - 1024} {
		if _, err := Inspect(bytes.NewReader(good[:cut]), 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatalf("truncation %d accepted: %v", cut, err)
		}
	}
	bad := append([]byte{}, good...)
	i := bytes.Index(bad, []byte("native workspace archive"))
	if i < 0 {
		t.Fatal("fixture missing")
	}
	bad[i] ^= 1
	if _, err := Inspect(bytes.NewReader(bad), 1024); !errors.Is(err, ErrInvalidBundle) {
		t.Fatal("changed bytes accepted", err)
	}
}
func TestBundleRefusesManifestDriftAndUnsafeHeaders(t *testing.T) {
	m, _ := fixture(false)
	raw, _ := json.Marshal(m)
	for _, data := range [][]byte{append([]byte(" "), raw...), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1), bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"unknown":true`), 1)} {
		var out bytes.Buffer
		tw := tar.NewWriter(&out)
		tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0600, Size: int64(len(data)), Format: tar.FormatUSTAR})
		tw.Write(data)
		tw.Close()
		if _, err := Inspect(&out, 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("noncanonical metadata accepted", err)
		}
	}
	for _, name := range []string{"../manifest.json", "/manifest.json", "manifest.json/", "C:/manifest.json"} {
		var out bytes.Buffer
		tw := tar.NewWriter(&out)
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(raw)), Format: tar.FormatUSTAR})
		tw.Write(raw)
		tw.Close()
		if _, err := Inspect(&out, 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("path accepted", err)
		}
	}
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeDir} {
		var out bytes.Buffer
		tw := tar.NewWriter(&out)
		tw.WriteHeader(&tar.Header{Name: "manifest.json", Linkname: "outside", Typeflag: kind, Format: tar.FormatUSTAR})
		tw.Close()
		if _, err := Inspect(&out, 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("non-regular entry accepted", err)
		}
	}
}
func TestBundleWriterDoesNotCompletePartialPayload(t *testing.T) {
	for _, extra := range []bool{false, true} {
		m, parts := fixture(false)
		payload := "x"
		if extra {
			payload = "native rootfs archivex"
		}
		parts[0] = bytes.NewBufferString(payload)
		var out bytes.Buffer
		if err := Write(&out, m, parts, 1024); err == nil {
			t.Fatal("changed/extra source accepted")
		}
		if _, err := Inspect(bytes.NewReader(out.Bytes()), 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("partial output accepted", err)
		}
	}
	m, parts := fixture(false)
	m.Components[1].Role = "base"
	if err := Write(io.Discard, m, parts, 1024); !errors.Is(err, ErrInvalidBundle) {
		t.Fatal("Base component accepted", err)
	}
	m, parts = fixture(false)
	m.HasOCI = true
	if err := Write(io.Discard, m, parts, 1024); !errors.Is(err, ErrInvalidBundle) {
		t.Fatal("missing OCI accepted", err)
	}
}

func TestBundleRejectsWrongAndExtraComponents(t *testing.T) {
	m, parts := fixture(false)
	var valid bytes.Buffer
	if err := Write(&valid, m, parts, 1024); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"traversal", "order", "extra", "gnu"} {
		var bad bytes.Buffer
		tw := tar.NewWriter(&bad)
		tr := tar.NewReader(bytes.NewReader(valid.Bytes()))
		index := 0
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if index == 1 {
				switch change {
				case "traversal":
					h.Name = "../rootfs.tar"
				case "order":
					h.Name = "workspace.tar"
				case "gnu":
					h.Format = tar.FormatGNU
				}
			}
			if err := tw.WriteHeader(h); err != nil {
				t.Fatal(err)
			}
			if _, err := io.Copy(tw, tr); err != nil {
				t.Fatal(err)
			}
			index++
		}
		if change == "extra" {
			if err := tw.WriteHeader(&tar.Header{Name: "rootfs.tar", Size: 1, Mode: 0600, Format: tar.FormatUSTAR}); err != nil {
				t.Fatal(err)
			}
			tw.Write([]byte("x"))
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := Inspect(bytes.NewReader(bad.Bytes()), 1024); !errors.Is(err, ErrInvalidBundle) {
			t.Fatal("unexpected component accepted", change, err)
		}
	}
}

func TestBundleBoundsReadsBeforeManifest(t *testing.T) {
	var extended bytes.Buffer
	tw := tar.NewWriter(&extended)
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Format: tar.FormatPAX, PAXRecords: map[string]string{"comment": "x"}}); err != nil {
		t.Fatal(err)
	}
	// First two blocks are the short extended header and its payload. Repeating
	// them exercises tar.Reader's internal special-header loop, before Next returns.
	if extended.Len() < 1536 {
		t.Fatal("expected extended header fixture")
	}
	input := &countedReader{Reader: bytes.NewReader(bytes.Repeat(extended.Bytes()[:1024], 100))}
	if _, err := Inspect(input, 1024); !errors.Is(err, ErrInvalidBundle) {
		t.Fatal("extended metadata accepted", err)
	}
	if input.n > 1024+envelopeOverhead {
		t.Fatal("unbounded header reads", input.n)
	}
	if _, err := Inspect(bytes.NewReader(nil), 1<<63-1); !errors.Is(err, ErrInvalidBundle) {
		t.Fatal("overflowing budget accepted", err)
	}
}

type mutateReader struct {
	io.Reader
	mutate func()
}

func (r *mutateReader) Read(p []byte) (int, error) {
	if r.mutate != nil {
		r.mutate()
		r.mutate = nil
	}
	return r.Reader.Read(p)
}
func TestWriterSnapshotsDescriptorsBeforeReaderCallbacks(t *testing.T) {
	m, parts := fixture(false)
	parts[0] = &mutateReader{Reader: parts[0], mutate: func() { m.Components[1].Role = "../outside"; parts[1] = bytes.NewReader(nil) }}
	var out bytes.Buffer
	if err := Write(&out, m, parts, 1024); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(bytes.NewReader(out.Bytes()), 1024); err != nil {
		t.Fatal("callback changed validated descriptors", err)
	}
}

type deliveredErrorWriter struct {
	bytes.Buffer
	size int
}

func (w *deliveredErrorWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if w.Len() >= w.size {
		return n, io.ErrClosedPipe
	}
	return n, err
}
func TestWriterPreservesErrorAfterBytesWereDelivered(t *testing.T) {
	m, parts := fixture(false)
	var reference bytes.Buffer
	if err := Write(&reference, m, parts, 1024); err != nil {
		t.Fatal(err)
	}
	m, parts = fixture(false)
	out := &deliveredErrorWriter{size: reference.Len()}
	if err := Write(out, m, parts, 1024); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("writer failure reported as success", err)
	}
	// Complete-looking bytes do not overrule the failed producer operation.
	if _, err := Inspect(bytes.NewReader(out.Bytes()), 1024); err != nil {
		t.Fatal("fixture did not deliver all bytes", err)
	}
}
