//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (*SandboxProvider) SupportsEnvironmentResources() bool { return true }

// Incus performs Lstat through its own stopped-instance file service. Never ask
// guest-provided executables to attest to path safety, or open guessed daemon
// storage paths in the controller's potentially different mount namespace.
func (p *SandboxProvider) verifyEnvironmentDataPaths(ctx context.Context, ref string, areas []core.EnvironmentRuntimeAttachment, mounts []WorkspaceAttachment) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	connect, err := localDaemonConnect(p.project)
	if err != nil {
		return err
	}
	server, err := connect(ctx)
	if err != nil {
		return err
	}
	defer server.Disconnect()
	connection, err := server.GetConnectionInfo()
	if err != nil {
		return err
	}
	if connection.Project != p.project || !filepath.IsAbs(connection.SocketPath) {
		return core.ErrUnsupported
	}
	if len(mounts) != 0 && !server.HasExtension("file_storage_volume") {
		return fmt.Errorf("repository cache placement requires the Incus file_storage_volume API: %w", core.ErrUnsupported)
	}
	for _, m := range mounts {
		// The rootfs API checks only the mountpoint's ancestors. Its contents
		// are not evidence about the Workspace volume mounted at that point.
		if err := verifyNativeDataDirectory(ctx, connection.URL, "/1.0/instances/"+ref+"/files", p.project, m.Path, false, server.DoHTTP); err != nil {
			return err
		}
	}
	for _, area := range areas {
		target := area.Attachment.Target
		if repositoryDataTarget(target) {
			found := false
			for _, m := range mounts {
				if !strings.HasPrefix(target, m.Path+"/") {
					continue
				}
				found = true
				if err := verifyWorkspaceDataPath(ctx, connection.URL, p.project, m, target, server.DoHTTP); err != nil {
					return err
				}
			}
			if !found {
				return core.ErrIncompatibleState
			}
			continue
		}
		if err := verifyRootfsDataPath(ctx, connection.URL, p.project, ref, target, server.DoHTTP); err != nil {
			return err
		}
	}
	return nil
}

type environmentDataHTTP func(*http.Request) (*http.Response, error)

func verifyRootfsDataPath(ctx context.Context, endpoint, project, ref, target string, do environmentDataHTTP) error {
	if !validEnvironmentDataTarget(target) || repositoryDataTarget(target) || validateManagedInstanceRef(ref) != nil {
		return core.ErrInvalidArgument
	}
	return verifyNativeDataDirectory(ctx, endpoint, "/1.0/instances/"+ref+"/files", project, target, true, do)
}

func verifyWorkspaceDataPath(ctx context.Context, endpoint, project string, mount WorkspaceAttachment, target string, do environmentDataHTTP) error {
	if _, err := environmentWorkspaceObject(mount); err != nil {
		return err
	}
	if !validEnvironmentDataTarget(target) || !strings.HasPrefix(target, mount.Path+"/") {
		return core.ErrInvalidArgument
	}
	apiPath := "/1.0/storage-pools/" + mount.Pool + "/volumes/custom/" + mount.Volume + "/files"
	return verifyNativeDataDirectory(ctx, endpoint, apiPath, project, strings.TrimPrefix(target, mount.Path), true, do)
}

func verifyNativeDataDirectory(ctx context.Context, endpoint, apiPath, project, target string, empty bool, do environmentDataHTTP) error {
	if !safeIncusRef(project) || !utf8.ValidString(target) || len(target) > 1024 || !strings.HasPrefix(target, "/") || target == "/" || path.Clean(target) != target || strings.Contains(target, "\\") || strings.ContainsFunc(target, unicode.IsControl) {
		return core.ErrInvalidArgument
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return core.ErrInvalidArgument
	}
	base.Path, base.RawPath, base.Fragment = apiPath, "", ""
	request := func(method, path string) (*http.Response, error) {
		u := *base
		u.RawQuery = url.Values{"project": {project}, "path": {path}}.Encode()
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		if err != nil {
			return nil, err
		}
		return do(req)
	}
	current := ""
	for _, part := range strings.Split(strings.TrimPrefix(target, "/"), "/") {
		current += "/" + part
		response, err := request(http.MethodHead, current)
		if err != nil {
			return err
		}
		if response == nil || response.Body == nil {
			return core.ErrRuntimeUnavailable
		}
		status, kind := response.StatusCode, response.Header.Get("X-Incus-type")
		if err := response.Body.Close(); err != nil {
			return err
		}
		if status == http.StatusNotFound {
			return nil
		}
		if status != http.StatusOK {
			return core.ErrRuntimeUnavailable
		}
		if kind != "directory" {
			return fmt.Errorf("cache placement %q crosses a link or non-directory: %w", current, core.ErrIncompatibleState)
		}
	}
	if !empty {
		return nil
	}
	// Cover only an empty directory. Neither Base nor Workspace content may be
	// silently hidden, erased or adopted as cache data.
	response, err := request(http.MethodGet, target)
	if err != nil {
		return err
	}
	if response == nil || response.Body == nil {
		return core.ErrRuntimeUnavailable
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK || response.Header.Get("X-Incus-type") != "directory" {
		return core.ErrIncompatibleState
	}
	const limit = 32 << 10
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if len(data) > limit {
		return core.ErrIncompatibleState
	}
	var result struct {
		Type       string   `json:"type"`
		StatusCode int      `json:"status_code"`
		Metadata   []string `json:"metadata"`
	}
	if json.Unmarshal(data, &result) != nil || result.Type != "sync" || result.StatusCode != http.StatusOK || result.Metadata == nil {
		return core.ErrIncompatibleState
	}
	if len(result.Metadata) != 0 {
		return fmt.Errorf("cache placement %q already contains data: %w", target, core.ErrStorageBusy)
	}
	return nil
}
