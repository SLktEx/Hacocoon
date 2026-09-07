package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ConfigureEnvironmentDNS enables the installed Standard component before the
// runtime is served. The source is a trusted product companion, never guest input.
func (r *Runtime) ConfigureEnvironmentDNS(source string) error {
	if r == nil {
		return core.ErrInvalidArgument
	}
	canonical, _, err := trustedClientSource(source)
	if err != nil {
		return err
	}
	r.environmentDNS = canonical
	return nil
}

func (r *Runtime) provisionEnvironmentDNS(ctx context.Context, ref string) error {
	if r.environmentDNS == "" {
		return nil
	}
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	marker, err := r.runner.Run(ctx, "incus", "config", "get", ref, managedEnvironmentMarkerKey, "--project", r.project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(marker.Stdout) != managedEnvironmentMarkerValue {
		return core.ErrIncompatibleState
	}
	source, digest, err := trustedClientSource(r.environmentDNS)
	if err != nil {
		return err
	}
	if _, err = r.runner.Run(ctx, "incus", "file", "push", source, ref+"/usr/local/libexec/hacocoon-dns.next", "--project", r.project, "--create-dirs", "--uid", "0", "--gid", "0", "--mode", "0755"); err != nil {
		return fmt.Errorf("install Environment DNS companion: %w", err)
	}
	result, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "sha256sum", "/usr/local/libexec/hacocoon-dns.next")
	fields := strings.Fields(result.Stdout)
	if err != nil || len(fields) != 2 || fields[0] != digest {
		return fmt.Errorf("verify Environment DNS companion: %w", core.ErrIncompatibleState)
	}
	if _, err = r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", environmentDNSSetup); err != nil {
		return fmt.Errorf("configure Environment name resolution: %w", err)
	}
	return nil
}

const environmentDNSSetup = `mv -T /usr/local/libexec/hacocoon-dns.next /usr/local/libexec/hacocoon-dns
test -d /etc/systemd/system
test ! -L /etc/systemd/system/hacocoon-dns.service
cat > /etc/systemd/system/hacocoon-dns.service <<'HACO_DNS_UNIT'
[Unit]
Description=Hacocoon policy-bound name resolution
After=network.target
[Service]
Type=notify
NotifyAccess=main
TimeoutStartSec=15
ExecStart=/usr/local/libexec/hacocoon-dns _dns-agent
Restart=on-failure
RestartSec=1
NoNewPrivileges=true
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
RestrictAddressFamilies=AF_INET AF_UNIX
[Install]
WantedBy=multi-user.target
HACO_DNS_UNIT
systemctl daemon-reload
systemctl enable hacocoon-dns.service
systemctl restart hacocoon-dns.service
systemctl is-active --quiet hacocoon-dns.service
test ! -d /etc/resolv.conf
rm -f /etc/resolv.conf
printf 'nameserver 127.0.0.1\noptions timeout:2 attempts:2\n' > /etc/resolv.conf
`
