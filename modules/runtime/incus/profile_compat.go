package incus

import (
	"context"
	"net/url"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// showProfileJSON keeps Hacocoon compatible with Incus releases whose
// `profile show` command does not expose --format=json (notably Incus 6.0 on
// Ubuntu 24.04). The raw profile API is stable and `incus query` prints the
// response metadata as JSON, which matches the shape consumed by callers.
func (r *Runtime) showProfileJSON(ctx context.Context, name, project string) (host.Result, error) {
	result, err := r.runner.Run(ctx, "incus", "profile", "show", name, "--project", project, "--format", "json")
	if err == nil {
		return result, nil
	}
	endpoint := "/1.0/profiles/" + url.PathEscape(name) + "?project=" + url.QueryEscape(project)
	return r.runner.Run(ctx, "incus", "query", endpoint)
}
