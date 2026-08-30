package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	defaultKubernetesBaseName = core.BaseName("haco/ubuntu-26.04")
	kubernetesBasesConfigEnv  = "HACO_KUBERNETES_BASES_JSON"
	maxKubernetesBasesConfig  = 64 * 1024
)

var kubeImageDigestPattern = regexp.MustCompile(`^(.+)@sha256:([0-9a-f]{64})$`)

type BaseProvider struct {
	*Provider
	sources map[core.BaseName]string
}

type resolvedKubernetesBase struct {
	ref   core.BaseRef
	image string
}

func NewBaseProvider(provider *Provider) (*BaseProvider, error) {
	if provider == nil {
		return nil, core.ErrInvalidArgument
	}
	sources := map[core.BaseName]string{
		defaultKubernetesBaseName: provider.image,
	}
	custom, err := customKubernetesBaseSourcesFromEnv()
	if err != nil {
		return nil, err
	}
	for name, source := range custom {
		if strings.HasPrefix(string(name), "haco/") {
			return nil, fmt.Errorf("custom Kubernetes Base %q may not override reserved haco/ namespace: %w", name, core.ErrInvalidArgument)
		}
		sources[name] = source
	}
	return &BaseProvider{Provider: provider, sources: sources}, nil
}

func (p *BaseProvider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	resolved, err := p.resolveBase(spec.Base)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	clone := *p.Provider
	clone.image = resolved.image
	spec.Base = ""
	created, err := clone.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	base := resolved.ref
	created.Base = &base
	return created, nil
}

func (p *BaseProvider) ListBases(context.Context) ([]core.BaseInfo, error) {
	if p == nil || p.Provider == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	names := make([]string, 0, len(p.sources))
	for name := range p.sources {
		names = append(names, string(name))
	}
	sort.Strings(names)
	bases := make([]core.BaseInfo, 0, len(names))
	for _, name := range names {
		bases = append(bases, core.BaseInfo{Name: core.BaseName(name)})
	}
	return bases, nil
}

func (p *BaseProvider) InspectBase(_ context.Context, name core.BaseName) (core.BaseInfo, error) {
	resolved, err := p.resolveBase(name)
	if err != nil {
		return core.BaseInfo{}, err
	}
	return core.BaseInfo{Name: resolved.ref.Name, Revision: resolved.ref.Revision}, nil
}

func (p *BaseProvider) resolveBase(requested core.BaseName) (resolvedKubernetesBase, error) {
	if p == nil || p.Provider == nil {
		return resolvedKubernetesBase{}, core.ErrRuntimeUnavailable
	}
	name := requested
	if name == "" {
		name = defaultKubernetesBaseName
	}
	if err := validateKubernetesBaseName(name); err != nil {
		return resolvedKubernetesBase{}, err
	}
	source, ok := p.sources[name]
	if !ok {
		return resolvedKubernetesBase{}, fmt.Errorf("Kubernetes Base %q: %w", name, core.ErrNotFound)
	}
	matches := kubeImageDigestPattern.FindStringSubmatch(source)
	if len(matches) != 3 {
		return resolvedKubernetesBase{}, fmt.Errorf("Kubernetes Base %q source %q is not digest-pinned; tag-to-digest resolution is not implemented without weakening Base identity: %w", name, source, core.ErrUnsupported)
	}
	return resolvedKubernetesBase{
		ref: core.BaseRef{
			Name:     name,
			Revision: core.BaseRevision("sha256:" + matches[2]),
		},
		image: source,
	}, nil
}

func customKubernetesBaseSourcesFromEnv() (map[core.BaseName]string, error) {
	raw := strings.TrimSpace(os.Getenv(kubernetesBasesConfigEnv))
	if raw == "" {
		return nil, nil
	}
	if len(raw) > maxKubernetesBasesConfig {
		return nil, fmt.Errorf("%s is too large: %w", kubernetesBasesConfigEnv, core.ErrInvalidArgument)
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, fmt.Errorf("decode %s: %w", kubernetesBasesConfigEnv, core.ErrInvalidArgument)
	}
	result := make(map[core.BaseName]string, len(decoded))
	for rawName, rawSource := range decoded {
		name := core.BaseName(rawName)
		if err := validateKubernetesBaseName(name); err != nil {
			return nil, err
		}
		source := strings.TrimSpace(rawSource)
		if source != rawSource || source == "" || len(source) > 1024 || strings.HasPrefix(source, "-") || strings.ContainsAny(source, "\r\n\x00") {
			return nil, fmt.Errorf("invalid OCI image source for Kubernetes Base %q: %w", name, core.ErrInvalidArgument)
		}
		result[name] = source
	}
	return result, nil
}

func validateKubernetesBaseName(name core.BaseName) error {
	value := string(name)
	if value == "" || len(value) > 128 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("invalid Kubernetes Base name %q: %w", value, core.ErrInvalidArgument)
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("invalid Kubernetes Base name %q: %w", value, core.ErrInvalidArgument)
		}
		for _, r := range segment {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.') {
				return fmt.Errorf("invalid Kubernetes Base name %q: %w", value, core.ErrInvalidArgument)
			}
		}
	}
	return nil
}
