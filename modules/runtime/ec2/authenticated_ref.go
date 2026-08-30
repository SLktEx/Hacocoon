package ec2

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	authenticatedRefPrefix = "ec2authv1."
	authenticatedRefDomain = "hacocoon-ec2-runtime-ref-auth-v1\x00"
	refKeyBytes            = 32
	maxAuthenticatedRefLen = 8192
)

type authenticatedInner interface {
	CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error)
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	ShellEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
	InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
}

// AuthenticatedRuntime is the production trust boundary around runtime.ec2.
// The inner EC2 ref still carries provider details, but the persisted ref is an
// authenticated capability: authority-sensitive selectors cannot be changed
// without the host-owned key.
type AuthenticatedRuntime struct {
	inner authenticatedInner
	key   []byte
}

func NewAuthenticated(inner authenticatedInner, key []byte) (*AuthenticatedRuntime, error) {
	if inner == nil || !validRefKey(key) {
		return nil, fmt.Errorf("EC2 authenticated runtime is not configured: %w", core.ErrRuntimeUnavailable)
	}
	return &AuthenticatedRuntime{inner: inner, key: append([]byte(nil), key...)}, nil
}

func (r *AuthenticatedRuntime) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if r == nil || r.inner == nil || !validRefKey(r.key) {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	created, err := r.inner.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	signed, err := encodeAuthenticatedRef(created.Ref, r.key)
	if err != nil {
		// The inner environment may already exist at this point. Never return an
		// unauthenticated ref that could later be treated as host authority.
		return core.EnvironmentRuntime{}, errors.Join(
			fmt.Errorf("authenticate EC2 runtime ref after create: %w", err),
			core.ErrRecoveryRequired,
		)
	}
	created.Ref = signed
	return created, nil
}

func (r *AuthenticatedRuntime) ExecEnvironment(ctx context.Context, rawRef string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	inner, err := r.verify(rawRef)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return r.inner.ExecEnvironment(ctx, inner, req)
}

func (r *AuthenticatedRuntime) ShellEnvironment(ctx context.Context, rawRef string) error {
	inner, err := r.verify(rawRef)
	if err != nil {
		return err
	}
	return r.inner.ShellEnvironment(ctx, inner)
}

func (r *AuthenticatedRuntime) DeleteEnvironment(ctx context.Context, rawRef string) error {
	inner, err := r.verify(rawRef)
	if err != nil {
		return err
	}
	return r.inner.DeleteEnvironment(ctx, inner)
}

func (r *AuthenticatedRuntime) InspectEnvironment(ctx context.Context, rawRef string) (core.EnvironmentRuntimeStatus, error) {
	inner, err := r.verify(rawRef)
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	return r.inner.InspectEnvironment(ctx, inner)
}

func (r *AuthenticatedRuntime) verify(rawRef string) (string, error) {
	if r == nil || r.inner == nil || !validRefKey(r.key) {
		return "", core.ErrRuntimeUnavailable
	}
	return decodeAuthenticatedRef(rawRef, r.key)
}

func encodeAuthenticatedRef(inner string, key []byte) (string, error) {
	if !validRefKey(key) || strings.TrimSpace(inner) == "" || len(inner) > maxRuntimeRefLength {
		return "", core.ErrInvalidArgument
	}
	payload := []byte(inner)
	signature := authenticatedRefMAC(payload, key)
	return authenticatedRefPrefix +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(signature), nil
}

func decodeAuthenticatedRef(raw string, key []byte) (string, error) {
	if !validRefKey(key) {
		return "", core.ErrRuntimeUnavailable
	}
	if strings.HasPrefix(raw, "ec2v1.") {
		return "", errors.Join(
			fmt.Errorf("legacy unsigned EC2 runtime ref cannot prove host authority: %w", core.ErrIncompatibleState),
			core.ErrRecoveryRequired,
		)
	}
	if len(raw) == 0 || len(raw) > maxAuthenticatedRefLen || !strings.HasPrefix(raw, authenticatedRefPrefix) {
		return "", fmt.Errorf("authenticated EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	rest := strings.TrimPrefix(raw, authenticatedRefPrefix)
	parts := strings.Split(rest, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("authenticated EC2 runtime ref shape: %w", core.ErrIncompatibleState)
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(payload) == 0 || len(payload) > maxRuntimeRefLength {
		return "", fmt.Errorf("authenticated EC2 runtime ref payload: %w", core.ErrIncompatibleState)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size {
		return "", fmt.Errorf("authenticated EC2 runtime ref signature: %w", core.ErrIncompatibleState)
	}
	expected := authenticatedRefMAC(payload, key)
	if !hmac.Equal(signature, expected) {
		return "", fmt.Errorf("EC2 runtime ref authentication failed: %w", core.ErrIncompatibleState)
	}
	inner := string(payload)
	// Validate the authenticated inner ref before returning it to a provider.
	// This preserves the existing strict JSON/authority syntax checks too.
	if _, err := decodeRef(inner); err != nil {
		return "", err
	}
	return inner, nil
}

func authenticatedRefMAC(payload, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = io.WriteString(mac, authenticatedRefDomain)
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}

func validRefKey(key []byte) bool { return len(key) >= refKeyBytes }

// LoadOrCreateRefKey loads the durable host-owned EC2 ref key, creating it on
// first use. The key is deliberately not sourced from environment variables:
// replacing or losing this file invalidates persisted EC2 authority and
// requires manual recovery instead of silently trusting unsigned state.
func LoadOrCreateRefKey(path string) ([]byte, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || strings.ContainsRune(path, '\x00') {
		return nil, core.ErrInvalidArgument
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("create EC2 ref-key directory: %w", err)
	}
	if err := validateRefKeyDirectory(parent); err != nil {
		return nil, err
	}
	if key, err := readRefKey(path); err == nil {
		return key, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key := make([]byte, refKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate EC2 ref key: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return readRefKey(path)
	}
	if err != nil {
		return nil, fmt.Errorf("create EC2 ref key: %w", err)
	}
	created := true
	defer func() {
		_ = file.Close()
		if created {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(key); err != nil {
		return nil, fmt.Errorf("write EC2 ref key: %w", err)
	}
	if err := file.Sync(); err != nil {
		return nil, fmt.Errorf("sync EC2 ref key: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, fmt.Errorf("close EC2 ref key: %w", err)
	}
	created = false
	return append([]byte(nil), key...), nil
}

func readRefKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("EC2 ref key must be a regular non-symlink file: %w", core.ErrIncompatibleState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("EC2 ref key permissions %o expose host authority: %w", info.Mode().Perm(), core.ErrIncompatibleState)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read EC2 ref key: %w", err)
	}
	if len(contents) != refKeyBytes {
		return nil, fmt.Errorf("EC2 ref key length %d: %w", len(contents), core.ErrIncompatibleState)
	}
	return contents, nil
}

func validateRefKeyDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect EC2 ref-key directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("EC2 ref-key parent is not a trusted directory: %w", core.ErrIncompatibleState)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("EC2 ref-key directory permissions %o are writable by group/other: %w", info.Mode().Perm(), core.ErrIncompatibleState)
	}
	return nil
}
