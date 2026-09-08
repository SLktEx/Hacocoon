package aws

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type guestSourceFunc func(context.Context, net.IP) (string, string, error)

func (f guestSourceFunc) ResolveEnvironmentInstance(c context.Context, ip net.IP) (string, string, error) {
	return f(c, ip)
}

type guestOps struct{ calls int }

func (g *guestOps) ListFromGuest(_ context.Context, source GuestSource, s ListSpec) (core.CapabilityResult, error) {
	g.calls++
	if source.Environment != "dev" || source.Instance != "env-11111111111111111111111111111111" || s.Environment != "" {
		panic("source not bound")
	}
	return core.CapabilityResult{Output: "[]"}, nil
}
func (g *guestOps) DownloadFromGuest(ctx context.Context, source GuestSource, s GetSpec, w io.Writer) (core.CapabilityResult, error) {
	result, err := g.ListFromGuest(ctx, source, ListSpec(s))
	if err != nil {
		return result, err
	}
	_, err = w.Write(make([]byte, 130000))
	return result, err
}

type deadlineRecorder struct{ *httptest.ResponseRecorder }

func (deadlineRecorder) SetReadDeadline(time.Time) error  { return nil }
func (deadlineRecorder) SetWriteDeadline(time.Time) error { return nil }
func TestGuestHTTPRefusesCallerAuthorityAndUnmanagedSources(t *testing.T) {
	for _, scenario := range []string{"list", "get", "environment", "instance", "unknown", "query", "absolute", "loopback", "forwarded", "missing-identity", "wrong-method"} {
		t.Run(scenario, func(t *testing.T) {
			ops := &guestOps{}
			resolver := guestSourceFunc(func(_ context.Context, ip net.IP) (string, string, error) {
				if !ip.Equal(net.ParseIP("10.200.0.2")) {
					return "", "", core.ErrPolicyDenied
				}
				if scenario == "missing-identity" {
					return "dev", "", nil
				}
				return "dev", "env-11111111111111111111111111111111", nil
			})
			payload := `{"operation":"list","url":"s3://example-bucket/project/"}`
			switch scenario {
			case "get":
				payload = strings.ReplaceAll(payload, "list", "get")
			case "environment", "instance", "unknown":
				payload = strings.TrimSuffix(payload, "}") + `,"` + scenario + `":"other"}`
			}
			req := httptest.NewRequest("POST", GuestPath, strings.NewReader(payload))
			req.RemoteAddr = "10.200.0.2:1234"
			req.Header.Set("Content-Type", "application/json")
			switch scenario {
			case "query":
				req.RequestURI = GuestPath + "?x=1"
			case "absolute":
				req.URL.Scheme = "http"
				req.URL.Host = "evil.invalid"
			case "loopback":
				req.RemoteAddr = "127.0.0.1:1234"
			case "forwarded":
				req.RemoteAddr = "10.200.0.3:1234"
				req.Header.Set("X-Forwarded-For", "10.200.0.2")
			case "wrong-method":
				req.Method = "GET"
			}
			w := deadlineRecorder{httptest.NewRecorder()}
			NewGuestHandler(ops, resolver).ServeHTTP(w, req)
			if scenario != "list" && scenario != "get" {
				if ops.calls != 0 || w.Code < 400 {
					t.Fatal("authority bypass", ops.calls, w.Code)
				}
				return
			}
			if ops.calls != 1 || w.Code != 200 {
				t.Fatal(ops.calls, w.Code, w.Body.String())
			}
			decoder := json.NewDecoder(w.Body)
			total, final := 0, 0
			for {
				var frame GuestFrame
				err := decoder.Decode(&frame)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(frame.Data) > 64<<10 {
					t.Fatal("unbounded frame")
				}
				total += len(frame.Data)
				if frame.Result != nil {
					final++
				}
			}
			if final != 1 || scenario == "get" && total != 130000 {
				t.Fatal(total, final)
			}
		})
	}
}
