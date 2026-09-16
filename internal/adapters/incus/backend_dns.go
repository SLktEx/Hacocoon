package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/network/dns"
	"github.com/SLktEx/Hacocoon/internal/network/egress"
	"net/netip"
)

const environmentDNSModeKey = "user.hacocoon.dns-mode"

// ResolveEnvironmentName uses only the owned backend tooling resolver. The
// ordinary host mode is implemented separately by Standard on the Physical Host.
func (r *Runtime) ResolveEnvironmentName(ctx context.Context, ref, instance, name string) ([]netip.Addr, error) {
	canonical, err := egress.CanonicalHost(name)
	if err != nil || canonical != name {
		return nil, core.ErrInvalidArgument
	}
	if err := r.VerifyEnvironmentIdentity(ctx, ref, instance); err != nil {
		return nil, err
	}
	input, err := json.Marshal(name)
	if err != nil {
		return nil, err
	}
	output, err := r.RunTrustedHostPython(ctx, backendDNSLookup, input)
	if err != nil {
		return nil, err
	}
	if err := r.VerifyEnvironmentIdentity(ctx, ref, instance); err != nil {
		return nil, err
	}
	return decodeBackendDNS(output)
}
func decodeBackendDNS(output []byte) ([]netip.Addr, error) {
	if len(output) > 4096 {
		return nil, core.ErrIncompatibleState
	}
	var addresses []netip.Addr
	if json.Unmarshal(output, &addresses) != nil || len(addresses) > nameresolution.MaxAddresses {
		return nil, core.ErrIncompatibleState
	}
	for _, ip := range addresses {
		if !ip.IsValid() || ip.Zone() != "" {
			return nil, core.ErrIncompatibleState
		}
	}
	return addresses, nil
}

const backendDNSLookup = `import json,socket,sys
name=json.load(sys.stdin)
answers=[]
for item in socket.getaddrinfo(name,None,socket.AF_UNSPEC,socket.SOCK_STREAM):
    address=item[4][0]
    if address not in answers: answers.append(address)
    if len(answers)>32: raise SystemExit(1)
print(json.dumps(answers))
`
