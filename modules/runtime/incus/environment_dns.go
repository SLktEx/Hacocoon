package incus

import (
	"context"
	"fmt"
	"strconv"
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
	result, err = r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "/bin/sh", "-ec", environmentDNSSetup)
	if err != nil {
		return fmt.Errorf("configure Environment name resolution (%s): %w", dnsSetupFailureStage(result.Stderr), err)
	}
	return nil
}

func dnsSetupFailureStage(output string) string {
	stage := "unknown"
	unitExit := -1
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "HACO_DNS_UNIT_EXIT=") {
			value, err := strconv.Atoi(strings.TrimPrefix(line, "HACO_DNS_UNIT_EXIT="))
			if err == nil && value >= 0 && value <= 255 {
				unitExit = value
			}
		}
		switch line {
		case "HACO_DNS_STAGE=manager", "HACO_DNS_STAGE=install", "HACO_DNS_STAGE=unit", "HACO_DNS_STAGE=reload", "HACO_DNS_STAGE=enable", "HACO_DNS_STAGE=restart", "HACO_DNS_STAGE=active", "HACO_DNS_STAGE=resolver":
			stage = strings.TrimPrefix(line, "HACO_DNS_STAGE=")
		}
	}
	if (stage == "restart" || stage == "active") && unitExit >= 0 {
		return fmt.Sprintf("%s; service_exit=%d", stage, unitExit)
	}
	return stage
}

const environmentDNSSetup = `stage=install
trap 'code=$?; if [ "$code" -ne 0 ]; then printf "HACO_DNS_STAGE=%s\n" "$stage" >&2; if [ "$stage" = restart ] || [ "$stage" = active ]; then unit_exit=$(systemctl show -p ExecMainStatus --value hacocoon-dns.service 2>/dev/null || :); printf "HACO_DNS_UNIT_EXIT=%s\n" "$unit_exit" >&2; fi; fi' EXIT
mv -T /usr/local/libexec/hacocoon-dns.next /usr/local/libexec/hacocoon-dns
stage=unit
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
stage=manager
attempt=0
until systemctl show --property=Version --value >/dev/null 2>&1; do
  attempt=$((attempt + 1))
  if [ "$attempt" -ge 60 ]; then exit 1; fi
  sleep 0.5
done
stage=reload
systemctl daemon-reload
stage=enable
systemctl enable hacocoon-dns.service
stage=restart
systemctl restart hacocoon-dns.service
stage=active
systemctl is-active --quiet hacocoon-dns.service
stage=resolver
test ! -d /etc/resolv.conf
rm -f /etc/resolv.conf
printf 'nameserver 127.0.0.1\noptions timeout:2 attempts:2\n' > /etc/resolv.conf
`
