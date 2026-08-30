package incus

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ResolveSourceIP asks the trusted Incus daemon which instance currently owns
// one IPv4 address inside the Hacocoon project. NIC source filtering on the
// managed profile prevents an untrusted Environment from freely spoofing a
// different managed source address.
func (r *Runtime) ResolveSourceIP(ctx context.Context, ip net.IP) (string, error) {
	if r == nil || r.runner == nil || ip == nil {
		return "", core.ErrInvalidArgument
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return "", core.ErrInvalidArgument
	}
	addr = addr.Unmap()
	if !addr.Is4() || addr.IsUnspecified() || addr.IsLoopback() {
		return "", core.ErrInvalidArgument
	}
	result, err := r.runner.Run(ctx, "incus", "list", "ipv4="+addr.String(), "--project", r.project, "-c", "n", "--format", "csv,noheader")
	if err != nil {
		return "", fmt.Errorf("resolve Incus source address %s: %w", addr, err)
	}
	reader := csv.NewReader(strings.NewReader(result.Stdout))
	reader.FieldsPerRecord = -1
	var names []string
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("decode Incus source lookup: %w", core.ErrIncompatibleState)
		}
		if len(record) != 1 {
			return "", fmt.Errorf("Incus source lookup returned malformed row: %w", core.ErrIncompatibleState)
		}
		name := strings.TrimSpace(record[0])
		if name == "" || strings.ContainsAny(name, "\r\n\x00") {
			return "", fmt.Errorf("Incus source lookup returned invalid instance name: %w", core.ErrIncompatibleState)
		}
		names = append(names, name)
	}
	switch len(names) {
	case 0:
		return "", fmt.Errorf("Incus source address %s: %w", addr, core.ErrNotFound)
	case 1:
		return names[0], nil
	default:
		return "", fmt.Errorf("Incus source address %s belongs to multiple instances: %w", addr, core.ErrIncompatibleState)
	}
}
