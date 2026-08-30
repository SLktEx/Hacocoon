package incus

import (
	"context"
	"net/url"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// showProfileJSON reads the stable Incus profile API through `incus query`.
// `profile show --format=json` is not consistently supported across the Incus
// releases Hacocoon runs on, while the raw API response is JSON and already
// matches the shape consumed by callers.
func (r *Runtime) showProfileJSON(ctx context.Context, name, project string) (host.Result, error) {
	endpoint := "/1.0/profiles/" + url.PathEscape(name) + "?project=" + url.QueryEscape(project)
	return r.runner.Run(ctx, "incus", "query", endpoint)
}
