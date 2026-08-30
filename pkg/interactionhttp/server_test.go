package interactionhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

type fakeReader struct {
	batch interaction.Batch
	err   error
}

func (f fakeReader) Batch(context.Context, int64, int) (interaction.Batch, error) {
	return f.batch, f.err
}

type recordingReader struct {
	batch  interaction.Batch
	err    error
	offset int64
	limit  int
	calls  int
}

func (r *recordingReader) Batch(_ context.Context, offset int64, limit int) (interaction.Batch, error) {
	r.calls++
	r.offset = offset
	r.limit = limit
	return r.batch, r.err
}

func TestEventsReturnsOnlyPublicInteractionFields(t *testing.T) {
	handler, err := NewHandler(fakeReader{batch: interaction.Batch{
		SchemaVersion: interaction.SchemaVersion,
		NextOffset:    42,
		Events: []interaction.Event{{
			SchemaVersion: interaction.SchemaVersion,
			EventID:       "req-1:approval-required",
			RequestID:     "req-1",
			Kind:          interaction.ApprovalRequired,
			Environment:   "dev",
			Capability:    "git.push",
			Action:        "push",
			NextOffset:    42,
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events?offset=0&limit=10", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"resource", "attributes", "approval_token", "credential", "provider_output"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("response exposed forbidden field %q: %s", forbidden, body)
		}
	}

	var response batchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.NextOffset != 42 || len(response.Events) != 1 {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestEventsForwardsResumeCursorAndBoundedLimit(t *testing.T) {
	reader := &recordingReader{batch: interaction.Batch{SchemaVersion: interaction.SchemaVersion, NextOffset: 99}}
	handler, err := NewHandler(reader)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/events?offset=17&limit=23", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if reader.calls != 1 || reader.offset != 17 || reader.limit != 23 {
		t.Fatalf("reader did not receive resume query: calls=%d offset=%d limit=%d", reader.calls, reader.offset, reader.limit)
	}
	var response batchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Events == nil || len(response.Events) != 0 || response.NextOffset != 99 {
		t.Fatalf("empty event batch was not stable JSON: %+v", response)
	}
}

func TestEventsPreservesTrustworthyPrefixOnCorruption(t *testing.T) {
	corruption := &interaction.CorruptionError{Line: 3, ByteOffset: 91, Kind: interaction.CorruptionMalformedJSON}
	handler, err := NewHandler(fakeReader{
		batch: interaction.Batch{SchemaVersion: interaction.SchemaVersion, Events: []interaction.Event{{EventID: "safe", Kind: interaction.OperationFailed}}, NextOffset: 80},
		err:   corruption,
	})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response batchResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Events) != 1 || response.NextOffset != 80 || response.Error == nil || response.Error.Code != "source-corruption" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response.Error.Line != 3 || response.Error.ByteOffset != 91 || response.Error.Kind != interaction.CorruptionMalformedJSON {
		t.Fatalf("corruption metadata was not minimized/stable: %+v", response.Error)
	}
}

func TestEventsRejectsBadArguments(t *testing.T) {
	handler, err := NewHandler(fakeReader{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/api/v1/events?offset=-1", "/api/v1/events?limit=0", "/api/v1/events?limit=1001"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("%s: status = %d", target, recorder.Code)
		}
	}
}

func TestEventsRejectMutationWithoutCallingReader(t *testing.T) {
	reader := &recordingReader{}
	handler, err := NewHandler(reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/api/v1/events", strings.NewReader("{}")))
		if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != "GET" {
			t.Fatalf("%s: status=%d allow=%q", method, recorder.Code, recorder.Header().Get("Allow"))
		}
	}
	if reader.calls != 0 {
		t.Fatalf("mutation requests reached interaction reader: %d", reader.calls)
	}
}

func TestBridgeSetsBrowserSecurityHeadersAndNoCORS(t *testing.T) {
	handler, err := NewHandler(fakeReader{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/", "/api/v1/events"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Referrer-Policy") != "no-referrer" {
			t.Fatalf("%s: required browser hardening headers missing: %v", target, recorder.Header())
		}
		csp := recorder.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "connect-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s: unexpected CSP: %q", target, csp)
		}
		if origin := recorder.Header().Get("Access-Control-Allow-Origin"); origin != "" {
			t.Fatalf("%s: bridge unexpectedly enabled CORS for %q", target, origin)
		}
	}
}

func TestEventsHidesUnexpectedReaderErrors(t *testing.T) {
	handler, err := NewHandler(fakeReader{err: errors.New("secret backend detail")})
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/events", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret backend detail") {
		t.Fatalf("unexpected internal error exposure: %s", recorder.Body.String())
	}
}

func TestIsLoopbackAddress(t *testing.T) {
	for _, address := range []string{"127.0.0.1:18081", "[::1]:18081", "localhost:18081"} {
		if !IsLoopbackAddress(address) {
			t.Fatalf("expected loopback: %s", address)
		}
	}
	for _, address := range []string{"0.0.0.0:18081", ":18081", "192.0.2.1:18081", "bad"} {
		if IsLoopbackAddress(address) {
			t.Fatalf("expected rejection: %s", address)
		}
	}
}
