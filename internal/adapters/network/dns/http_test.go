package dnsproxy

import (
	"bytes"
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/net/dns/dnsmessage"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

type lookupFunc func(context.Context, string, string) ([]netip.Addr, error)

func (f lookupFunc) Resolve(ctx context.Context, env, host string) ([]netip.Addr, error) {
	return f(ctx, env, host)
}

type sourceFunc func(context.Context, net.IP) (string, error)

func (f sourceFunc) ResolveEnvironment(ctx context.Context, ip net.IP) (string, error) {
	return f(ctx, ip)
}
func packet(t *testing.T, kind dnsmessage.Type) []byte {
	t.Helper()
	name, err := dnsmessage.NewName("Intranet.Example.")
	if err != nil {
		t.Fatal(err)
	}
	message := dnsmessage.Message{Header: dnsmessage.Header{ID: 42, RecursionDesired: true}, Questions: []dnsmessage.Question{{Name: name, Type: kind, Class: dnsmessage.ClassINET}}}
	data, err := message.Pack()
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestDNSHTTPRelayBindsSourceAndReturnsOnlyRequestedFamily(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "allowed", true: "denied"}[denied], func(t *testing.T) {
			lookups := 0
			handler := NewHandler(lookupFunc(func(_ context.Context, env, host string) ([]netip.Addr, error) {
				lookups++
				if env != "dev" || host != "intranet.example" {
					t.Fatal("guest supplied authority")
				}
				return []netip.Addr{netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("fd00::5")}, nil
			}), sourceFunc(func(_ context.Context, ip net.IP) (string, error) {
				if denied {
					return "", core.ErrPolicyDenied
				}
				return "dev", nil
			}))
			server := httptest.NewServer(handler)
			defer server.Close()
			request, err := http.NewRequest(http.MethodPost, server.URL+Path, bytes.NewReader(packet(t, dnsmessage.TypeA)))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", ContentType)
			request.Header.Set("X-Environment", "victim")
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if denied {
				if response.StatusCode != 403 || lookups != 0 {
					t.Fatal("unmanaged source resolved")
				}
				return
			}
			var message dnsmessage.Message
			if response.StatusCode != 200 || message.Unpack(data) != nil || message.ID != 42 || len(message.Answers) != 1 || message.Answers[0].Header.TTL != 0 {
				t.Fatalf("bad DNS response %d", response.StatusCode)
			}
			if message.Answers[0].Body.(*dnsmessage.AResource).A != [4]byte{10, 0, 0, 5} {
				t.Fatal("private answer changed")
			}
		})
	}
}
func TestDNSRejectsUnsupportedAndMalformedRequestsWithoutLookup(t *testing.T) {
	handler := NewHandler(lookupFunc(func(context.Context, string, string) ([]netip.Addr, error) {
		t.Fatal("unexpected lookup")
		return nil, nil
	}), nil)
	query := packet(t, dnsmessage.TypeTXT)
	answer := handler.answer(context.Background(), "dev", query)
	var message dnsmessage.Message
	if message.Unpack(answer) != nil || message.RCode != dnsmessage.RCodeNotImplemented {
		t.Fatal("unsupported DNS was relayed")
	}
	for _, data := range [][]byte{nil, bytes.Repeat([]byte{0xff}, 4097), append([]byte{0, 0, 0, 0, 0, 2}, make([]byte, 20)...)} {
		if handler.answer(context.Background(), "dev", data) != nil {
			t.Fatal("malformed DNS accepted")
		}
	}
}
func TestDNSPolicyDenialIsNotAnAddressAnswer(t *testing.T) {
	handler := NewHandler(lookupFunc(func(context.Context, string, string) ([]netip.Addr, error) { return nil, core.ErrPolicyDenied }), nil)
	var message dnsmessage.Message
	if message.Unpack(handler.answer(context.Background(), "dev", packet(t, dnsmessage.TypeAAAA))) != nil || message.RCode != dnsmessage.RCodeRefused || len(message.Answers) != 0 {
		t.Fatal("policy refusal lost")
	}
}
