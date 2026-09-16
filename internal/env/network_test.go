package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"testing"
)

type networkProvider struct {
	fakeProvider
	native, instance, protocol string
	port                       int
}

func (p *networkProvider) DialEnvironmentNetwork(_ context.Context, ref, instance, protocol string, port int) (net.Conn, error) {
	p.native, p.instance, p.protocol, p.port = ref, instance, protocol, port
	return nil, core.ErrRuntimeUnavailable
}
func TestNetworkConnectionResolvesStoredProviderRoute(t *testing.T) {
	p := &networkProvider{}
	router, err := NewRouter(testProvider, Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.DialEnvironmentNetwork(context.Background(), encodeRouteRef(testProvider, "native-peer"), "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "udp", 9000)
	if !errors.Is(err, core.ErrRuntimeUnavailable) || p.native != "native-peer" || p.protocol != "udp" || p.port != 9000 || p.instance != "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatal(p, err)
	}
	_, err = router.DialEnvironmentNetwork(context.Background(), encodeRouteRef("runtime.absent", "native-peer"), "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "udp", 9000)
	if err == nil {
		t.Fatal("unknown provider accepted")
	}
}
