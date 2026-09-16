package incus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// EmptyEnvironmentResource uses the volume file API, never guest executables or
// guessed host mount paths. The stopped parent's persisted fence owns the work.
func (b *PersistentResourceBackend) EmptyEnvironmentResource(ctx context.Context, lease core.WorkspaceLease, area core.EnvironmentAttachment, resource core.PersistentResource) error {
	if b == nil || b.Runtime == nil || resource.State != "clearing" || resource.Kind != CacheResourceKind || resource.Ref() != area.Resource || resource.EnvironmentInstance != lease.InstanceID || lease.State != core.WorkspaceLeaseActive || lease.RuntimeAbsent || lease.AccessMode != core.WorkspaceReadWrite {
		return core.ErrInvalidArgument
	}
	found := false
	for _, a := range lease.Attachments {
		if a == area {
			found = true
		}
	}
	if !found {
		return core.ErrInvalidArgument
	}
	verify := func() error {
		observed, err := b.observe(ctx, resource)
		if err != nil {
			return err
		}
		return b.verifyStoppedEnvironmentDataConsumer(ctx, resource, observed, lease.RuntimeRef)
	}
	if err := verify(); err != nil {
		return err
	}
	pool, name, err := managedResourceVolume(resource)
	if err != nil {
		return err
	}
	connect, err := localDaemonConnect(b.Runtime.project)
	if err != nil {
		return err
	}
	server, err := connect(ctx)
	if err != nil {
		return err
	}
	defer server.Disconnect()
	info, err := server.GetConnectionInfo()
	if err != nil {
		return err
	}
	if info.Project != b.Runtime.project || !filepath.IsAbs(info.SocketPath) || !server.HasExtension("file_storage_volume") {
		return core.ErrUnsupported
	}
	apiPath := "/1.0/storage-pools/" + pool + "/volumes/custom/" + name + "/files"
	if err := emptyNativeDataVolume(ctx, info.URL, apiPath, b.Runtime.project, server.DoHTTP); err != nil {
		return err
	}
	return verify()
}

// Enumerate before deleting, never follow symlinks, and never remove the volume
// root. On any failure, the caller retains its persisted clearing state.
func emptyNativeDataVolume(ctx context.Context, endpoint, apiPath, project string, do environmentDataHTTP) error {
	if !safeIncusRef(project) || do == nil {
		return core.ErrInvalidArgument
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return core.ErrInvalidArgument
	}
	base.Path, base.RawPath, base.Fragment = apiPath, "", ""
	request := func(method, target string) (*http.Response, error) {
		u := *base
		u.RawQuery = url.Values{"project": {project}, "path": {target}}.Encode()
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
		response, err := do(req)
		if err != nil {
			return nil, err
		}
		if response == nil || response.Body == nil {
			return nil, core.ErrRuntimeUnavailable
		}
		return response, nil
	}
	list := func(target string) ([]string, error) {
		response, err := request(http.MethodGet, target)
		if err != nil {
			return nil, err
		}
		defer func() { _ = response.Body.Close() }()
		if response.StatusCode != http.StatusOK || response.Header.Get("X-Incus-type") != "directory" {
			return nil, core.ErrIncompatibleState
		}
		const limit = 4 << 20
		data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
		if err != nil {
			return nil, err
		}
		if len(data) > limit {
			return nil, core.ErrIncompatibleState
		}
		var result struct {
			Type       string   `json:"type"`
			StatusCode int      `json:"status_code"`
			Metadata   []string `json:"metadata"`
		}
		if json.Unmarshal(data, &result) != nil || result.Type != "sync" || result.StatusCode != http.StatusOK || result.Metadata == nil {
			return nil, core.ErrIncompatibleState
		}
		seen := map[string]bool{}
		for _, name := range result.Metadata {
			if name == "" || name == "." || name == ".." || len(name) > 255 || !utf8.ValidString(name) || strings.ContainsAny(name, "/\\") || strings.ContainsFunc(name, unicode.IsControl) || seen[name] {
				return nil, core.ErrIncompatibleState
			}
			seen[name] = true
		}
		return result.Metadata, nil
	}
	targets := []string{}
	count := 0
	var walk func(string, int) error
	walk = func(parent string, depth int) error {
		if depth > 64 {
			return core.ErrIncompatibleState
		}
		names, err := list(parent)
		if err != nil {
			return err
		}
		for _, name := range names {
			count++
			if count > 100000 {
				return core.ErrIncompatibleState
			}
			target := path.Join(parent, name)
			if len(target) > 4096 {
				return core.ErrIncompatibleState
			}
			response, err := request(http.MethodHead, target)
			if err != nil {
				return err
			}
			status, kind := response.StatusCode, response.Header.Get("X-Incus-type")
			if err := response.Body.Close(); err != nil {
				return err
			}
			if status != http.StatusOK {
				return core.ErrIncompatibleState
			}
			switch kind {
			case "directory":
				if err := walk(target, depth+1); err != nil {
					return err
				}
			case "file", "symlink":
			default:
				return core.ErrIncompatibleState
			}
			targets = append(targets, target)
		}
		return nil
	}
	if err := walk("/", 0); err != nil {
		return err
	}
	for _, target := range targets {
		response, err := request(http.MethodDelete, target)
		if err != nil {
			return err
		}
		status := response.StatusCode
		if err := response.Body.Close(); err != nil {
			return err
		}
		if status != http.StatusOK && status != http.StatusNotFound {
			return core.ErrRuntimeUnavailable
		}
	}
	remaining, err := list("/")
	if err != nil {
		return err
	}
	if len(remaining) != 0 {
		return core.ErrRecoveryRequired
	}
	return nil
}
