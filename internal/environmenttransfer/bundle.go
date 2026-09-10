// Package environmenttransfer binds native Incus archives without extracting
// files or restoring source authority. Public lifecycle integration is separate.
package environmenttransfer

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
)

const manifestLimit = 64 * 1024
const maxWorkspaces = 253 // Same supported attachment bound as the Incus snapshot planner.
const maxComponents = maxWorkspaces + 2
const envelopeOverhead = 512 * 1024

func validLimit(n int64) bool { return n > 0 && n <= (1<<63-1)-envelopeOverhead }

var sourceName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
var ErrInvalidBundle = errors.New("invalid or incomplete Environment transfer bundle")

// Manifest is an internal transport format, not a catalog, lease or approval.
// It deliberately has no provider paths, credentials or source management IDs.
type Manifest struct {
	Version    int         `json:"version"`
	Source     string      `json:"source"`
	HasOCI     bool        `json:"has_oci"`
	Components []Component `json:"components"`
}
type Component struct {
	Role   string `json:"role"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func (m Manifest) validate(limit int64) error {
	if m.Version != 1 || !sourceName.MatchString(m.Source) || !validLimit(limit) {
		return ErrInvalidBundle
	}
	count := len(m.Components) - 1
	if m.HasOCI {
		count--
	}
	if count < 1 || count > maxWorkspaces {
		return ErrInvalidBundle
	}
	roles := []string{"rootfs"}
	for i := 0; i < count; i++ {
		roles = append(roles, workspaceRole(i))
	}
	if m.HasOCI {
		roles = append(roles, "oci")
	}
	for i, c := range m.Components {
		if c.Role != roles[i] || c.Bytes <= 0 || c.Bytes > limit || !digestPattern.MatchString(c.SHA256) {
			return ErrInvalidBundle
		}
		limit -= c.Bytes // Checked subtraction avoids attacker-controlled size overflow.
	}
	return nil
}

// Numbering identifies transport order, never a destination path or authority.
func workspaceRole(index int) string {
	if index == 0 {
		return "workspace"
	}
	return fmt.Sprintf("workspace-%03d", index+1)
}

func copyComponent(dst io.Writer, src io.Reader, c Component) error {
	hash := sha256.New()
	if _, err := io.CopyN(io.MultiWriter(dst, hash), src, c.Bytes); err != nil {
		return fmt.Errorf("%s: %w", c.Role, errors.Join(ErrInvalidBundle, err))
	}
	if hex.EncodeToString(hash.Sum(nil)) != c.SHA256 {
		return ErrInvalidBundle
	}
	return nil
}

// Write streams a staged export. The caller must publish the output only after
// success. Any output from a failed write must remain unpublished, even if
// the underlying writer delivered bytes before reporting its error.
// Readers must end at their declared sizes. The limit is a caller-owned budget,
// not an additional user CLI argument. Native payloads remain opaque here.
func Write(dst io.Writer, m Manifest, parts []io.Reader, limit int64) error {
	if len(m.Components) < 2 || len(m.Components) > maxComponents || len(parts) != len(m.Components) {
		return ErrInvalidBundle
	}
	// Reader/writer callbacks must not change descriptors after validation.
	m.Components = append([]Component(nil), m.Components...)
	parts = append([]io.Reader(nil), parts...)
	if err := m.validate(limit); err != nil {
		return err
	}
	if dst == nil || len(parts) != len(m.Components) {
		return ErrInvalidBundle
	}
	for _, r := range parts {
		if r == nil {
			return ErrInvalidBundle
		}
	}
	raw, err := json.Marshal(m)
	if err != nil || len(raw) > manifestLimit {
		return ErrInvalidBundle
	}
	archive := tar.NewWriter(dst)
	header := func(name string, size int64) error {
		return archive.WriteHeader(&tar.Header{Name: name, Size: size, Mode: 0600, Typeflag: tar.TypeReg, Format: tar.FormatUSTAR})
	}
	if err := header("manifest.json", int64(len(raw))); err != nil {
		return err
	}
	if _, err := archive.Write(raw); err != nil {
		return err
	}
	for i, c := range m.Components {
		if err := header(c.Role+".tar", c.Bytes); err != nil {
			return err
		}
		if err := copyComponent(archive, parts[i], c); err != nil {
			return err
		}
		if err := requireEOF(parts[i]); err != nil {
			return err
		}
	}
	return archive.Close()
}

func requireEOF(r io.Reader) error {
	var one [1]byte
	n, err := io.ReadFull(r, one[:])
	if n != 0 || err != io.EOF {
		return errors.Join(ErrInvalidBundle, err)
	}
	return nil
}

type countedReader struct {
	io.Reader
	n uint64
}

func (r *countedReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.n += uint64(n)
	return n, err
}

func regularHeader(h *tar.Header, name string, size int64) bool {
	return h != nil && h.Name == name && h.Typeflag == tar.TypeReg && h.Linkname == "" && h.Size == size && h.Format == tar.FormatUSTAR
}

// Inspect verifies the whole stream without extracting or invoking a callback.
// A returned manifest proves structure and bytes only, not source authenticity,
// inner archive safety, lifecycle ownership or permission to restore settings.
// Later use must retain immutable staged bytes or reverify them; paths alone do
// not bind this observation to a subsequent import.
func Inspect(src io.Reader, limit int64) (Manifest, error) {
	return inspect(src, limit, nil)
}

// Component locations are recorded by the same parser that verifies all bytes.
// They are private until the complete envelope has passed validation.
type componentSpan struct {
	component Component
	offset    int64
}

func inspect(src io.Reader, limit int64, spans *[]componentSpan) (Manifest, error) {
	fail := func(err error) (Manifest, error) { return Manifest{}, errors.Join(ErrInvalidBundle, err) }
	if src == nil || !validLimit(limit) {
		return fail(nil)
	}
	// Bound reads even before a valid manifest, including hidden PAX headers.
	// Every valid fixed-name envelope fits within this payload-plus-header bound.
	counted := &countedReader{Reader: io.LimitReader(src, limit+envelopeOverhead)}
	archive := tar.NewReader(counted)
	h, err := archive.Next()
	if err != nil {
		return fail(err)
	}
	if h.Size <= 0 || h.Size > manifestLimit || !regularHeader(h, "manifest.json", h.Size) {
		return fail(nil)
	}
	raw, err := io.ReadAll(archive)
	if err != nil {
		return fail(err)
	}
	var m Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return fail(err)
	}
	canonical, err := json.Marshal(m)
	if err != nil || !bytes.Equal(canonical, raw) {
		return fail(nil)
	} // Includes duplicate keys and trailing JSON.
	if err := m.validate(limit); err != nil {
		return fail(err)
	}
	for _, c := range m.Components {
		h, err := archive.Next()
		if err != nil {
			return fail(err)
		}
		if !regularHeader(h, c.Role+".tar", c.Bytes) {
			return fail(nil)
		}
		if spans != nil {
			*spans = append(*spans, componentSpan{component: c, offset: int64(counted.n)})
		}
		if err := copyComponent(io.Discard, archive, c); err != nil {
			return fail(err)
		}
	}
	// archive/tar also reports EOF for a stream missing its closing blocks.
	// Require the writer's final padding and two complete zero blocks explicitly.
	before := counted.n
	if _, err := archive.Next(); err != io.EOF {
		return fail(err)
	}
	padding := (512 - m.Components[len(m.Components)-1].Bytes%512) % 512
	if counted.n-before != uint64(padding+1024) {
		return fail(nil)
	}
	if err := requireEOF(counted); err != nil {
		return fail(err)
	}
	return m, nil
}
