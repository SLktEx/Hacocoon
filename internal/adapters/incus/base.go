package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	defaultBaseName = core.BaseName("haco/ubuntu-26.04")
	baseConfigEnv   = "HACO_INCUS_BASES_JSON"
	maxBaseConfig   = 64 * 1024
)

var baseFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type BaseProvider struct {
	baseMu sync.RWMutex
	*Runtime
	sources map[core.BaseName]string
}

type resolvedBase struct {
	ref          core.BaseRef
	pinnedSource string
	built        bool
}

func NewBaseProvider(runtime *Runtime) (*BaseProvider, error) {
	if runtime == nil {
		return nil, core.ErrInvalidArgument
	}
	sources := map[core.BaseName]string{
		defaultBaseName:                    "images:ubuntu/26.04",
		core.BaseName("haco/ubuntu-24.04"): "images:ubuntu/24.04",
	}
	custom, err := customBaseSourcesFromEnv()
	if err != nil {
		return nil, err
	}
	for name, source := range custom {
		if strings.HasPrefix(string(name), "haco/") {
			return nil, fmt.Errorf("custom Base %q may not override reserved haco/ namespace: %w", name, core.ErrInvalidArgument)
		}
		sources[name] = source
	}
	provider := &BaseProvider{Runtime: runtime, sources: sources}
	return provider, nil
}

func customBaseSourcesFromEnv() (map[core.BaseName]string, error) {
	raw := strings.TrimSpace(os.Getenv(baseConfigEnv))
	if raw == "" {
		return nil, nil
	}
	if len(raw) > maxBaseConfig {
		return nil, fmt.Errorf("%s is too large: %w", baseConfigEnv, core.ErrInvalidArgument)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", baseConfigEnv, core.ErrInvalidArgument)
	}
	result := make(map[core.BaseName]string, len(decoded))
	for rawName, rawSource := range decoded {
		name := core.BaseName(rawName)
		if err := validateBaseName(name); err != nil {
			return nil, err
		}
		source := strings.TrimSpace(rawSource)
		if source != rawSource || source == "" || len(source) > 512 || strings.HasPrefix(source, "-") || hasControlString(source) {
			return nil, fmt.Errorf("invalid Incus source for Base %q: %w", name, core.ErrInvalidArgument)
		}
		result[name] = source
	}
	return result, nil
}

func validateBaseName(name core.BaseName) error {
	value := string(name)
	if value == "" || len(value) > 128 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || hasControlString(value) {
		return fmt.Errorf("invalid Base name %q: %w", value, core.ErrInvalidArgument)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("invalid Base name %q: %w", value, core.ErrInvalidArgument)
		}
		for _, r := range segment {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return fmt.Errorf("invalid Base name %q: %w", value, core.ErrInvalidArgument)
			}
		}
	}
	return nil
}

func hasControlString(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

func (p *BaseProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if spec.DNSMode.Effective() != core.DNSHost {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	if len(spec.Attachments) != 0 {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	// Retained Store startup is not wired yet. Never fall through to ordinary
	// creation, which may start daemons before maintenance preparation.
	if spec.ResourceMaintenance {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	p.baseMu.RLock()
	defer p.baseMu.RUnlock()
	resolved, err := p.resolveBase(ctx, spec.Base)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	clone := *p.Runtime
	clone.image = resolved.pinnedSource
	created, err := clone.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	base := resolved.ref
	created.Base = &base
	return created, nil
}

func (p *BaseProvider) ListBases(ctx context.Context) ([]core.BaseInfo, error) {
	if p == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	built, err := p.builtBases(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(p.sources)+len(built))
	for name := range built {
		names = append(names, string(name))
	}
	for name := range p.sources {
		names = append(names, string(name))
	}
	sort.Strings(names)
	result := make([]core.BaseInfo, 0, len(names))
	for _, name := range names {
		info := core.BaseInfo{Name: core.BaseName(name)}
		if found, ok := built[info.Name]; ok {
			info = found
		}
		result = append(result, info)
	}
	return result, nil
}

func (p *BaseProvider) InspectBase(ctx context.Context, name core.BaseName) (core.BaseInfo, error) {
	resolved, err := p.resolveBase(ctx, name)
	if err != nil {
		return core.BaseInfo{}, err
	}
	return core.BaseInfo{Name: resolved.ref.Name, Revision: resolved.ref.Revision}, nil
}

// resolveBase resolves only the explicitly selected Base. Retained Seed catalogs
// cannot replace its image or grant Environment nesting authority.
func (p *BaseProvider) resolveBase(ctx context.Context, requested core.BaseName) (resolvedBase, error) {
	if p == nil {
		return resolvedBase{}, core.ErrRuntimeUnavailable
	}
	name := requested
	if name == "" {
		name = defaultBaseName
	}
	if err := validateBaseName(name); err != nil {
		return resolvedBase{}, err
	}
	source, ok := p.sources[name]
	if !ok {
		built, err := p.builtBases(ctx)
		if err != nil {
			return resolvedBase{}, err
		}
		info, found := built[name]
		if !found {
			return resolvedBase{}, fmt.Errorf("Base %q: %w", name, core.ErrNotFound)
		}
		fingerprint, err := baseRevisionFingerprint(info.Revision)
		if err != nil {
			return resolvedBase{}, err
		}
		return resolvedBase{ref: core.BaseRef{Name: name, Revision: info.Revision}, pinnedSource: "local:" + fingerprint, built: true}, nil
	}
	project := ""
	if strings.HasPrefix(source, "local:") {
		project = p.project
	}
	fingerprint, err := p.imageFingerprint(ctx, source, project)
	if err != nil {
		return resolvedBase{}, fmt.Errorf("resolve Base %q from %q: %w", name, source, err)
	}
	return resolvedBase{
		ref: core.BaseRef{
			Name:     name,
			Revision: core.BaseRevision("sha256:" + fingerprint),
		},
		pinnedSource: pinImageSource(source, fingerprint),
	}, nil
}

func baseRevisionFingerprint(revision core.BaseRevision) (string, error) {
	value := string(revision)
	if !strings.HasPrefix(value, "sha256:") {
		return "", core.ErrIncompatibleState
	}
	fingerprint := strings.TrimPrefix(value, "sha256:")
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return "", core.ErrIncompatibleState
	}
	return fingerprint, nil
}

func pinImageSource(source, fingerprint string) string {
	if cut := strings.IndexByte(source, ':'); cut > 0 {
		return source[:cut+1] + fingerprint
	}
	return fingerprint
}
