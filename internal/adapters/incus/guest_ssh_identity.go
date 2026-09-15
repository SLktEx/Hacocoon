package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (p *SandboxProvider) renewGuestSSHIdentity(ctx context.Context, ref string) error {
	out, err := p.runner.Run(ctx, "incus", "exec", ref, "--project", p.project, "--", "/bin/sh", "-ec", freshGuestSSHIdentity)
	if err != nil || out.ExitCode != 0 {
		return fmt.Errorf("renew Environment SSH identity: %w", core.ErrRuntimeUnavailable)
	}
	return nil
}

// Guest-local reset runs before publication/proxy exposure. Custom user keys
// remain, while managed authorization entries and server host identity are fresh.
const freshGuestSSHIdentity = `set -eu
for dir in /root /root/.ssh /etc /etc/ssh; do
 test ! -L "$dir"
 if test -e "$dir"; then test -d "$dir"; fi
done
file=/root/.ssh/authorized_keys
test ! -L "$file"
if test -e "$file"; then
 test -f "$file"
 tmp="$(mktemp /root/.ssh/authorized_keys.restore.XXXXXX)"
 trap 'rm -f -- "$tmp"' EXIT
 awk 'index($0, " haco:ssh-") == 0 { print }' "$file" > "$tmp"
 chmod 600 "$tmp"
 mv -- "$tmp" "$file"
 trap - EXIT
fi
if command -v ssh-keygen >/dev/null 2>&1; then
 for key in /etc/ssh/ssh_host_*_key /etc/ssh/ssh_host_*_key.pub; do
  test ! -L "$key"
  if test -e "$key"; then test -f "$key"; rm -f -- "$key"; fi
 done
 ssh-keygen -A
 if command -v sshd >/dev/null 2>&1; then
  attempt=0
  until systemctl show --property=Version --value >/dev/null 2>&1; do
   attempt=$((attempt + 1)); test "$attempt" -lt 60; sleep 0.5
  done
  systemctl restart ssh.service
 fi
fi
`
