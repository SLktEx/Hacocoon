// Package workspaceinput defines provider-neutral input for independent work.
package workspaceinput

import (
	"archive/tar"
	"context"
	"errors"
	"io"
	"path"
	"strings"
)

var ErrInvalid = errors.New("invalid or unsupported Workspace input")

const Limit int64 = 64 << 30

// CopyTree validates the entire stream while visiting each entry. A consumer
// must keep its output private until this returns successfully. No extraction,
// executable, source path, provider configuration or credentials are involved.
func CopyTree(ctx context.Context, source io.Reader, visit func(*tar.Header, io.Reader) error) error {
	bounded := &io.LimitedReader{R: source, N: Limit + 1}
	r := tar.NewReader(bounded)
	seen := map[string]byte{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(h.Name, "/")
		if len(name) > 4096 || path.Clean(name) != name || (name != "tree" && !strings.HasPrefix(name, "tree/")) || len(seen) >= 1000000 {
			return ErrInvalid
		}
		if _, ok := seen[name]; ok {
			return ErrInvalid
		}
		if name != "tree" && seen[path.Dir(name)] != tar.TypeDir {
			return ErrInvalid
		}
		if h.Mode&^0777 != 0 || h.Size < 0 || h.Size > Limit {
			return ErrInvalid
		}
		for key := range h.PAXRecords {
			if key != "path" && key != "linkpath" {
				return ErrInvalid
			}
		}
		switch h.Typeflag {
		case tar.TypeDir, tar.TypeReg:
		case tar.TypeSymlink:
			if strings.HasPrefix(name, "tree/.git/") || path.IsAbs(h.Linkname) || !strings.HasPrefix(path.Clean(path.Join(path.Dir(name), h.Linkname)), "tree/") {
				return ErrInvalid
			}
		default:
			return ErrInvalid
		}
		seen[name] = h.Typeflag
		if err := visit(h, r); err != nil {
			return err
		}
	}
	if seen["tree"] != tar.TypeDir || seen["tree/.git"] != tar.TypeDir || seen["tree/.git/HEAD"] != tar.TypeReg || seen["tree/.git/config"] != tar.TypeReg {
		return ErrInvalid
	}
	// Require the transport's explicit end and reject concatenated payloads.
	tail, err := io.ReadAll(io.LimitReader(bounded, 10241))
	if err != nil {
		return err
	}
	if bounded.N <= 0 || len(tail) > 10240 {
		return ErrInvalid
	}
	for _, b := range tail {
		if b != 0 {
			return ErrInvalid
		}
	}
	return nil
}
