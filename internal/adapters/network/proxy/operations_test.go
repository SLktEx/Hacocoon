package egressproxy

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOptionalOperationRouteCannotCaptureExternalProxyURLs(t *testing.T) {
	calls := 0
	auth := &fakeAuthorizer{err: core.ErrPolicyDenied}
	p := NewWithOperations(auth, fakeSources{environment: "dev"}, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(204) }))
	local := httptest.NewRequest("POST", "/_haco/operations/example", nil)
	local.RemoteAddr = "10.0.0.5:54321"
	w := httptest.NewRecorder()
	p.ServeHTTP(w, local)
	if w.Code != 204 || calls != 1 || len(auth.requests) != 0 {
		t.Fatal("local operation routing failed")
	}
	external := httptest.NewRequest("POST", "http://intranet.example/_haco/operations/example", nil)
	external.RemoteAddr = "10.0.0.5:54321"
	w = httptest.NewRecorder()
	p.ServeHTTP(w, external)
	if w.Code != 403 || calls != 1 || len(auth.requests) != 1 {
		t.Fatal("external request captured optional route")
	}
}
