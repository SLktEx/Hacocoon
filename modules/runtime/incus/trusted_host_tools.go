package incus

import (
	"context"
	"fmt"
)

const trustedHostToolsScript = `set -eu
if test -x /usr/bin/git && test -x /usr/bin/gh; then
  exit 0
fi
export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends git gh
test -x /usr/bin/git
test -x /usr/bin/gh
`

// ensureTrustedHostTools keeps the minimal Host-local developer tool contract
// independent from user customization. Authentication and user-specific tools
// are intentionally not configured here.
func (r *Runtime) ensureTrustedHostTools(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "incus", "exec", trustedHostName,
		"--project", r.project,
		"--", "env", "-i",
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"/bin/sh", "-ec", trustedHostToolsScript,
	); err != nil {
		return fmt.Errorf("ensure trusted host standard tools: %w", err)
	}
	return nil
}
