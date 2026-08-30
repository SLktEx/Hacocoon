package seedbuild

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const pinStateVersion = 1

type SeedPin struct {
	Base     core.BaseName `json:"base"`
	Image    ImageIdentity `json:"image"`
	PinnedAt time.Time     `json:"pinned_at"`
}

type pinFileState struct {
	Version int                            `json:"version"`
	Pins    map[string]map[string]SeedPin `json:"pins"`
}

type DeletionSource interface {
	IsImageDeleted(context.Context, string, string) (bool, error)
}

type MaintenanceProtection struct {
	Revisions []core.BaseRevision `json:"revisions"`
	Aliases   []string            `json:"aliases"`
}

type MaintenanceReport struct {
	DeletedBuilders []string          `json:"deleted_builders,omitempty"`
	DeletedImages   []core.BaseRevision `json:"deleted_images,omitempty"`
	RetainedImages  map[string]string `json:"retained_images,omitempty"`
	Failures        map[string]string `json:"failures,omitempty"`
}

type MaintenanceBackend interface {
	MaintainSeedArtifacts(context.Context, MaintenanceProtection, bool) (MaintenanceReport, error)
}

func (s *Service) Pin(ctx context.Context, base core.BaseName, raw string) (SeedPin, error) {
	if s == nil || s.backend == nil || s.stats == nil || s.store == nil {
		return SeedPin{}, core.ErrRuntimeUnavailable
	}
	image, err := parsePinnedImage(raw)
	if err != nil {
		return SeedPin{}, err
	}
	unlock, err := s.store.LockBuild()
	if err != nil {
		return SeedPin{}, err
	}
	defer unlock()
	parent, err := s.backend.ResolveParentBase(ctx, base)
	if err != nil {
		return SeedPin{}, err
	}
	deleted, err := s.imageDeleted(ctx, image)
	if err != nil {
		return SeedPin{}, err
	}
	if deleted {
		return SeedPin{}, fmt.Errorf("Seed image %s has an OCI deletion tombstone; re-enable it explicitly before pinning: %w", image.String(), core.ErrIncompatibleState)
	}
	pin := SeedPin{Base: parent.Name, Image: image, PinnedAt: s.now().UTC()}
	return s.store.PutPin(ctx, pin)
}

func (s *Service) Unpin(ctx context.Context, base core.BaseName, raw string) (bool, error) {
	if s == nil || s.backend == nil || s.store == nil {
		return false, core.ErrRuntimeUnavailable
	}
	image, err := parsePinnedImage(raw)
	if err != nil {
		return false, err
	}
	unlock, err := s.store.LockBuild()
	if err != nil {
		return false, err
	}
	defer unlock()
	parent, err := s.backend.ResolveParentBase(ctx, base)
	if err != nil {
		return false, err
	}
	return s.store.DeletePin(ctx, parent.Name, image)
}

func (s *Service) Pins(ctx context.Context, base core.BaseName) ([]SeedPin, error) {
	if s == nil || s.backend == nil || s.store == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	parent, err := s.backend.ResolveParentBase(ctx, base)
	if err != nil {
		return nil, err
	}
	return s.store.ListPins(ctx, parent.Name)
}

func (s *Service) GC(ctx context.Context) (MaintenanceReport, error) {
	if s == nil || s.backend == nil || s.store == nil {
		return MaintenanceReport{}, core.ErrRuntimeUnavailable
	}
	unlock, err := s.store.LockBuild()
	if err != nil {
		return MaintenanceReport{}, err
	}
	defer unlock()
	return s.maintainLocked(ctx, false)
}

func (s *Service) Recover(ctx context.Context) (MaintenanceReport, error) {
	if s == nil || s.backend == nil || s.store == nil {
		return MaintenanceReport{}, core.ErrRuntimeUnavailable
	}
	unlock, err := s.store.LockBuild()
	if err != nil {
		return MaintenanceReport{}, err
	}
	defer unlock()
	return s.maintainLocked(ctx, true)
}

func (s *Service) recoverLocked(ctx context.Context) error {
	if _, ok := s.backend.(MaintenanceBackend); !ok {
		return nil
	}
	_, err := s.maintainLocked(ctx, true)
	return err
}

func (s *Service) maintainLocked(ctx context.Context, recoverBuilders bool) (MaintenanceReport, error) {
	backend, ok := s.backend.(MaintenanceBackend)
	if !ok {
		return MaintenanceReport{}, fmt.Errorf("Seed maintenance is unsupported by this runtime provider: %w", core.ErrUnsupported)
	}
	protection, err := s.store.MaintenanceProtection(ctx)
	if err != nil {
		return MaintenanceReport{}, err
	}
	report, err := backend.MaintainSeedArtifacts(ctx, protection, recoverBuilders)
	if err != nil {
		return report, err
	}
	if len(report.Failures) > 0 {
		return report, errors.Join(fmt.Errorf("Seed maintenance reported partial failures"), core.ErrRecoveryRequired)
	}
	return report, nil
}

func (s *Service) mergePinnedImages(ctx context.Context, base core.BaseName, automatic []ImageIdentity) ([]ImageIdentity, error) {
	pins, err := s.store.ListPins(ctx, base)
	if err != nil {
		return nil, err
	}
	if len(pins) == 0 {
		return automatic, nil
	}
	if _, ok := s.stats.(DeletionSource); !ok {
		return nil, fmt.Errorf("Seed pins require deletion-state validation from the OCI plugin: %w", core.ErrUnsupported)
	}
	seen := make(map[string]struct{}, len(automatic)+len(pins))
	result := make([]ImageIdentity, 0, len(automatic)+len(pins))
	for _, image := range automatic {
		seen[image.String()] = struct{}{}
		result = append(result, image)
	}
	for _, pin := range pins {
		deleted, err := s.imageDeleted(ctx, pin.Image)
		if err != nil {
			return nil, err
		}
		if deleted {
			return nil, fmt.Errorf("pinned Seed image %s is blocked by an OCI deletion tombstone; re-enable or unpin it before building: %w", pin.Image.String(), core.ErrIncompatibleState)
		}
		if _, ok := seen[pin.Image.String()]; ok {
			continue
		}
		seen[pin.Image.String()] = struct{}{}
		result = append(result, pin.Image)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Reference != result[j].Reference {
			return result[i].Reference < result[j].Reference
		}
		return result[i].Digest < result[j].Digest
	})
	return result, nil
}

func (s *Service) validateDeletionState(ctx context.Context, images []ImageIdentity) error {
	if _, ok := s.stats.(DeletionSource); !ok {
		return nil
	}
	for _, image := range images {
		deleted, err := s.imageDeleted(ctx, image)
		if err != nil {
			return err
		}
		if deleted {
			return fmt.Errorf("Seed image %s was deleted while the build was in progress: %w", image.String(), core.ErrIncompatibleState)
		}
	}
	return nil
}

func (s *Service) imageDeleted(ctx context.Context, image ImageIdentity) (bool, error) {
	source, ok := s.stats.(DeletionSource)
	if !ok {
		return false, fmt.Errorf("OCI plugin does not expose deletion state: %w", core.ErrUnsupported)
	}
	return source.IsImageDeleted(ctx, image.Reference, image.Digest)
}

func parsePinnedImage(raw string) (ImageIdentity, error) {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return ImageIdentity{}, core.ErrInvalidArgument
	}
	cut := strings.LastIndexByte(raw, '@')
	if cut <= 0 || cut == len(raw)-1 {
		return ImageIdentity{}, fmt.Errorf("Seed pin must use reference@sha256:... immutable identity: %w", core.ErrInvalidArgument)
	}
	image := ImageIdentity{Reference: raw[:cut], Digest: strings.ToLower(raw[cut+1:])}
	if err := validateImageIdentity(image); err != nil {
		return ImageIdentity{}, err
	}
	return image, nil
}

func (s *Store) PutPin(_ context.Context, pin SeedPin) (SeedPin, error) {
	if s == nil || !validPin(pin) {
		return SeedPin{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.pinPath() + ".lock")
	if err != nil {
		return SeedPin{}, err
	}
	defer unlock()
	state, err := s.readPins()
	if err != nil {
		return SeedPin{}, err
	}
	base := string(pin.Base)
	if state.Pins[base] == nil {
		state.Pins[base] = map[string]SeedPin{}
	}
	if existing, ok := state.Pins[base][pin.Image.String()]; ok {
		return existing, nil
	}
	state.Pins[base][pin.Image.String()] = pin
	if err := s.writePins(state); err != nil {
		return SeedPin{}, err
	}
	return pin, nil
}

func (s *Store) DeletePin(_ context.Context, base core.BaseName, image ImageIdentity) (bool, error) {
	if s == nil || !validPinBase(base) || validateImageIdentity(image) != nil {
		return false, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.pinPath() + ".lock")
	if err != nil {
		return false, err
	}
	defer unlock()
	state, err := s.readPins()
	if err != nil {
		return false, err
	}
	pins := state.Pins[string(base)]
	if pins == nil {
		return false, nil
	}
	if _, ok := pins[image.String()]; !ok {
		return false, nil
	}
	delete(pins, image.String())
	if len(pins) == 0 {
		delete(state.Pins, string(base))
	}
	return true, s.writePins(state)
}

func (s *Store) ListPins(_ context.Context, base core.BaseName) ([]SeedPin, error) {
	if s == nil || !validPinBase(base) {
		return nil, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.pinPath() + ".lock")
	if err != nil {
		return nil, err
	}
	defer unlock()
	state, err := s.readPins()
	if err != nil {
		return nil, err
	}
	pins := state.Pins[string(base)]
	result := make([]SeedPin, 0, len(pins))
	for _, pin := range pins {
		result = append(result, pin)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Image.String() < result[j].Image.String() })
	return result, nil
}

func (s *Store) MaintenanceProtection(_ context.Context) (MaintenanceProtection, error) {
	if s == nil {
		return MaintenanceProtection{}, core.ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := lockFile(s.path + ".lock")
	if err != nil {
		return MaintenanceProtection{}, err
	}
	defer unlock()
	state, err := s.read()
	if err != nil {
		return MaintenanceProtection{}, err
	}
	revisions := map[core.BaseRevision]struct{}{}
	aliases := map[string]struct{}{}
	for _, manifest := range state.Current {
		revisions[manifest.SeedRevision] = struct{}{}
		revisions[manifest.ToolingRevision] = struct{}{}
		if manifest.SeedAlias != "" {
			aliases[manifest.SeedAlias] = struct{}{}
		}
	}
	for _, manifest := range state.Tooling {
		revisions[manifest.ToolingRevision] = struct{}{}
		if manifest.ToolingAlias != "" {
			aliases[manifest.ToolingAlias] = struct{}{}
		}
	}
	result := MaintenanceProtection{
		Revisions: make([]core.BaseRevision, 0, len(revisions)),
		Aliases:   make([]string, 0, len(aliases)),
	}
	for revision := range revisions {
		result.Revisions = append(result.Revisions, revision)
	}
	for alias := range aliases {
		result.Aliases = append(result.Aliases, alias)
	}
	sort.Slice(result.Revisions, func(i, j int) bool { return result.Revisions[i] < result.Revisions[j] })
	sort.Strings(result.Aliases)
	return result, nil
}

func (s *Store) pinPath() string { return s.path + ".pins.json" }

func (s *Store) readPins() (pinFileState, error) {
	contents, err := os.ReadFile(s.pinPath())
	if errors.Is(err, os.ErrNotExist) {
		return pinFileState{Version: pinStateVersion, Pins: map[string]map[string]SeedPin{}}, nil
	}
	if err != nil {
		return pinFileState{}, fmt.Errorf("read Seed pin state: %w", err)
	}
	var state pinFileState
	if err := json.Unmarshal(contents, &state); err != nil {
		return pinFileState{}, fmt.Errorf("decode Seed pin state: %w", err)
	}
	if state.Version != pinStateVersion {
		return pinFileState{}, fmt.Errorf("Seed pin state version %d is unsupported: %w", state.Version, core.ErrIncompatibleState)
	}
	if state.Pins == nil {
		state.Pins = map[string]map[string]SeedPin{}
	}
	for base, pins := range state.Pins {
		if !validPinBase(core.BaseName(base)) || pins == nil {
			return pinFileState{}, fmt.Errorf("invalid Seed pin base %q: %w", base, core.ErrIncompatibleState)
		}
		for key, pin := range pins {
			if !validPin(pin) || pin.Base != core.BaseName(base) || key != pin.Image.String() {
				return pinFileState{}, fmt.Errorf("invalid Seed pin %q for Base %q: %w", key, base, core.ErrIncompatibleState)
			}
		}
	}
	return state, nil
}

func (s *Store) writePins(state pinFileState) error {
	dir := filepath.Dir(s.pinPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Seed pin state directory: %w", err)
	}
	state.Version = pinStateVersion
	if state.Pins == nil {
		state.Pins = map[string]map[string]SeedPin{}
	}
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode Seed pin state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".seed-pins-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary Seed pin state: %w", err)
	}
	name := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}
	if err := tmp.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		cleanup()
		return fmt.Errorf("write temporary Seed pin state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temporary Seed pin state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Rename(name, s.pinPath()); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("commit Seed pin state: %w", err)
	}
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func validPin(pin SeedPin) bool {
	return validPinBase(pin.Base) && pin.PinnedAt.IsZero() == false && validateImageIdentity(pin.Image) == nil
}

func validPinBase(base core.BaseName) bool {
	value := string(base)
	if value == "" || strings.TrimSpace(value) != value || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || len(value) > 128 {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, r := range segment {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}
