//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (*SandboxProvider) SupportsEnvironmentResources() bool { return true }

// Incus performs Lstat through its own stopped-instance file service. Never ask
// guest-provided executables to attest to path safety, or open guessed daemon
// storage paths in the controller's potentially different mount namespace.
func (p *SandboxProvider) verifyEnvironmentDataPaths(ctx context.Context, ref string, areas []core.EnvironmentRuntimeAttachment) error {
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
	for _, area := range areas {
		if err := verifyRootfsDataPath(ctx, connection.URL, p.project, ref, area.Attachment.Target, server.DoHTTP); err != nil {
			return err
		}
	}
	return nil
}

type environmentDataHTTP func(*http.Request) (*http.Response, error)

func verifyRootfsDataPath(ctx context.Context, endpoint, project, ref, target string, do environmentDataHTTP) error {
	if !validEnvironmentDataTarget(target) || validateManagedInstanceRef(ref) != nil || !safeIncusRef(project) {
		return core.ErrInvalidArgument
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Host == "" || base.User != nil || (base.Scheme != "http" && base.Scheme != "https") {
		return core.ErrInvalidArgument
	}
	base.Path, base.RawPath, base.Fragment = "/1.0/instances/"+ref+"/files", "", ""
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
		response.Body.Close()
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
	// Only an empty existing rootfs directory may be covered. In particular a
	// Base's existing cache is not silently hidden, erased or adopted as a source.
	response, err := request(http.MethodGet, target)
	if err != nil {
		return err
	}
	if response == nil || response.Body == nil {
		return core.ErrRuntimeUnavailable
	}
	defer response.Body.Close()
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
