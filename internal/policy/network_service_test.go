package capability

import (
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestNetworkServiceRequiresCanonicalSingleEndpoint(t *testing.T) {
	base := NetworkService{Name: "database", Instance: "svc-" + strings.Repeat("a", 32), Protocol: "tcp", Address: "127.0.0.1", Port: 5432}
	for _, protocol := range []string{"tcp", "udp"} {
		for _, address := range []string{"127.0.0.1", "192.0.2.1", "::1", "2001:db8::1"} {
			for _, port := range []int{1, 65535} {
				service := base
				service.Protocol, service.Address, service.Port = protocol, address, port
				if err := ValidateNetworkService(service); err != nil {
					t.Fatal("valid explicit endpoint rejected", service, err)
				}
			}
		}
	}
	for _, address := range []string{"", "localhost", "127.0.0.1:5432", "0.0.0.0", "::", "224.0.0.1", "ff02::1", "169.254.1.1", "fe80::1", "fe80::1%eth0", "::ffff:127.0.0.1", "2001:DB8::1", "2001:db8:0:0:0:0:0:1"} {
		service := base
		service.Address = address
		if err := ValidateNetworkService(service); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal("ambiguous or non-unicast address accepted", address, err)
		}
	}
	for _, change := range []func(*NetworkService){
		func(s *NetworkService) { s.Name = "-option" },
		func(s *NetworkService) { s.Name = strings.Repeat("a", 64) },
		func(s *NetworkService) { s.Instance = "database" },
		func(s *NetworkService) { s.Protocol = "unix" },
		func(s *NetworkService) { s.Port = 0 },
		func(s *NetworkService) { s.Port = 65536 },
	} {
		service := base
		change(&service)
		if err := ValidateNetworkService(service); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal("invalid endpoint identity accepted", service, err)
		}
	}
}
