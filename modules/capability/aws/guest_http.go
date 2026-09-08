package aws

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"net/http"
	"time"
)

const GuestPath = "/_haco/operations/aws"

type GuestSources interface {
	ResolveEnvironmentInstance(context.Context, net.IP) (string, string, error)
}
type GuestOperations interface {
	ListFromGuest(context.Context, GuestSource, ListSpec) (core.CapabilityResult, error)
	DownloadFromGuest(context.Context, GuestSource, GetSpec, io.Writer) (core.CapabilityResult, error)
}
type GuestRequest struct {
	Operation string `json:"operation"`
	URL       string `json:"url"`
	Profile   string `json:"profile,omitempty"`
	Region    string `json:"region,omitempty"`
}
type GuestFrame struct {
	Data   []byte                 `json:"data,omitempty"`
	Result *core.CapabilityResult `json:"result,omitempty"`
	Error  string                 `json:"error,omitempty"`
}
type GuestHandler struct {
	operations GuestOperations
	sources    GuestSources
	slots      chan struct{}
}

func NewGuestHandler(operations GuestOperations, sources GuestSources) *GuestHandler {
	return &GuestHandler{operations: operations, sources: sources, slots: make(chan struct{}, 32)}
}
func (h *GuestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Connection", "close")
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.operations == nil || h.sources == nil {
		http.Error(w, "AWS unavailable", 503)
		return
	}
	if r.URL == nil || r.URL.IsAbs() || r.RequestURI != GuestPath || r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "invalid AWS request", 400)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		http.Error(w, "AWS busy", 503)
		return
	}
	ctl := http.NewResponseController(w)
	if ctl.SetReadDeadline(time.Now().Add(10*time.Second)) != nil || ctl.SetWriteDeadline(time.Now().Add(15*time.Minute)) != nil {
		http.Error(w, "AWS transport unavailable", 503)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Minute)
	defer cancel()
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(peer)
	if err != nil || ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		http.Error(w, "unmanaged AWS source", 403)
		return
	}
	name, instance, err := h.sources.ResolveEnvironmentInstance(ctx, ip)
	if err != nil || name == "" || !core.ValidEnvironmentInstanceID(instance) {
		http.Error(w, "unmanaged AWS source", 403)
		return
	}
	var request GuestRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&request) != nil || decoder.Decode(new(any)) != io.EOF || (request.Operation != "list" && request.Operation != "get") {
		http.Error(w, "invalid AWS request", 400)
		return
	}
	spec := ListSpec{URL: request.URL, Profile: request.Profile, Region: request.Region}
	w.Header().Set("Content-Type", "application/x-ndjson")
	encoder := json.NewEncoder(w)
	source := GuestSource{Environment: name, Instance: instance}
	var result core.CapabilityResult
	if request.Operation == "list" {
		result, err = h.operations.ListFromGuest(ctx, source, spec)
	} else {
		result, err = h.operations.DownloadFromGuest(ctx, source, GetSpec(spec), guestWriter{ctl, encoder})
	}
	failure := ""
	if err != nil {
		failure = "AWS operation did not succeed"
	}
	if ctl.SetWriteDeadline(time.Now().Add(5*time.Second)) != nil {
		return
	}
	_ = encoder.Encode(GuestFrame{Result: &result, Error: failure})
}

type guestWriter struct {
	ctl     *http.ResponseController
	encoder *json.Encoder
}

func (w guestWriter) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		n := len(data)
		if n > 64<<10 {
			n = 64 << 10
		}
		if err := w.ctl.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return total, err
		}
		if err := w.encoder.Encode(GuestFrame{Data: data[:n]}); err != nil {
			return total, err
		}
		if err := w.ctl.Flush(); err != nil {
			return total, err
		}
		total += n
		data = data[n:]
	}
	return total, nil
}
