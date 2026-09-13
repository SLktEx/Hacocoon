//go:build linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/cliconfig"
	"net/http"
	"strings"
)

type localIncusConnect func(context.Context) (incusclient.InstanceServer, error)

// localDaemonConnect resolves the same Incus CLI configuration once. These
// native local/WSL operations do not support a remote HTTPS daemon.
// Never silently fall back from a configured remote to the local daemon.
func localDaemonConnect(project string) (localIncusConnect, error) {
	config, err := cliconfig.LoadConfig("")
	if err != nil {
		return nil, err
	}
	remote, ok := config.Remotes[config.DefaultRemote]
	socket, unixRemote := strings.CutPrefix(remote.Addr, "unix:")
	if !ok || remote.Public || remote.Protocol != "incus" || !unixRemote {
		return nil, core.ErrUnsupported
	}
	socket = strings.TrimPrefix(socket, "//")
	return func(ctx context.Context) (incusclient.InstanceServer, error) {
		server, err := incusclient.ConnectIncusUnixWithContext(ctx, socket, &incusclient.ConnectionArgs{SkipGetEvents: true, TransportWrapper: func(base *http.Transport) incusclient.HTTPTransporter { return &localIncusTransport{base: base} }})
		if err != nil {
			return nil, err
		}
		if server.IsClustered() {
			server.Disconnect()
			return nil, core.ErrUnsupported
		}
		return server.UseProject(project), nil
	}, nil
}

// The native SDK handles metadata decoding. Bound its input; only the separate
// image export endpoint uses the caller's larger streaming archive budget.
type localIncusTransport struct{ base *http.Transport }

func (t *localIncusTransport) Transport() *http.Transport { return t.base }
func (t *localIncusTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(request)
	if err != nil {
		return response, err
	}
	if request.Method != http.MethodGet || !strings.HasSuffix(request.URL.Path, "/export") {
		response.Body = http.MaxBytesReader(nil, response.Body, 1<<20)
	}
	return response, nil
}
