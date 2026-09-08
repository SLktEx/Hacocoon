package dnsproxy

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/egress"
	"golang.org/x/net/dns/dnsmessage"
)

const Path = "/_haco/dns-query"
const Endpoint = "http://169.254.254.1:18080" + Path
const MaxMessageBytes = 4096
const ContentType = "application/dns-message"

type Lookup interface {
	Resolve(context.Context, string, string) ([]netip.Addr, error)
}
type Sources interface {
	ResolveEnvironment(context.Context, net.IP) (string, error)
}
type Handler struct {
	lookup  Lookup
	sources Sources
	slots   chan struct{}
}

func NewHandler(lookup Lookup, sources Sources) *Handler {
	return &Handler{lookup: lookup, sources: sources, slots: make(chan struct{}, 32)}
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A bounded per-query connection cannot become an alternate persistent tunnel.
	w.Header().Set("Connection", "close")
	if r.Method != http.MethodPost || r.URL.IsAbs() || r.RequestURI != Path || r.Header.Get("Content-Type") != ContentType {
		http.Error(w, "invalid DNS relay request", http.StatusBadRequest)
		return
	}
	if h == nil || h.lookup == nil || h.sources == nil {
		http.Error(w, "DNS relay unavailable", http.StatusServiceUnavailable)
		return
	}
	select {
	case h.slots <- struct{}{}:
		defer func() { <-h.slots }()
	default:
		http.Error(w, "DNS relay busy", http.StatusServiceUnavailable)
		return
	}
	ctl := http.NewResponseController(w)
	deadline := time.Now().Add(6 * time.Second)
	if ctl.SetReadDeadline(deadline) != nil || ctl.SetWriteDeadline(deadline) != nil {
		http.Error(w, "DNS relay deadline unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "unmanaged DNS source", http.StatusForbidden)
		return
	}
	ip := net.ParseIP(peer)
	if ip == nil {
		http.Error(w, "unmanaged DNS source", http.StatusForbidden)
		return
	}
	environment, err := h.sources.ResolveEnvironment(ctx, ip)
	if err != nil || environment == "" {
		http.Error(w, "unmanaged DNS source", http.StatusForbidden)
		return
	}
	input, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxMessageBytes))
	if err != nil {
		http.Error(w, "invalid DNS message", http.StatusBadRequest)
		return
	}
	response := h.answer(ctx, environment, input)
	if response == nil {
		http.Error(w, "invalid DNS message", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", ContentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(response)
}

func question(input []byte) (dnsmessage.Message, error) {
	var message dnsmessage.Message
	if len(input) < 12 || len(input) > MaxMessageBytes || binary.BigEndian.Uint16(input[4:6]) != 1 || binary.BigEndian.Uint16(input[6:8]) != 0 || binary.BigEndian.Uint16(input[8:10]) != 0 || binary.BigEndian.Uint16(input[10:12]) > 1 {
		return message, core.ErrInvalidArgument
	}
	if err := message.Unpack(input); err != nil {
		return message, err
	}
	if message.Response || message.OpCode != 0 || message.RCode != 0 || message.Truncated || len(message.Questions) != 1 {
		return message, core.ErrInvalidArgument
	}
	if len(message.Additionals) > 0 && message.Additionals[0].Header.Type != dnsmessage.TypeOPT {
		return message, core.ErrInvalidArgument
	}
	return message, nil
}
func reply(query dnsmessage.Message, code dnsmessage.RCode, addresses []netip.Addr) []byte {
	out := dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true, RecursionDesired: query.RecursionDesired, RecursionAvailable: true, RCode: code}, Questions: query.Questions}
	if code == dnsmessage.RCodeSuccess && len(query.Questions) == 1 {
		q := query.Questions[0]
		for _, ip := range addresses {
			ip = ip.Unmap()
			header := dnsmessage.ResourceHeader{Name: q.Name, Type: q.Type, Class: dnsmessage.ClassINET, TTL: 0}
			if q.Type == dnsmessage.TypeA && ip.Is4() {
				out.Answers = append(out.Answers, dnsmessage.Resource{Header: header, Body: &dnsmessage.AResource{A: ip.As4()}})
			}
			if q.Type == dnsmessage.TypeAAAA && ip.Is6() {
				out.Answers = append(out.Answers, dnsmessage.Resource{Header: header, Body: &dnsmessage.AAAAResource{AAAA: ip.As16()}})
			}
		}
	}
	packed, err := out.Pack()
	if err != nil || len(packed) > MaxMessageBytes {
		return nil
	}
	return packed
}
func failure(input []byte) []byte {
	q, err := question(input)
	if err != nil {
		return nil
	}
	return reply(q, dnsmessage.RCodeServerFailure, nil)
}
func (h *Handler) answer(ctx context.Context, environment string, input []byte) []byte {
	query, err := question(input)
	if err != nil {
		return nil
	}
	q := query.Questions[0]
	if q.Class != dnsmessage.ClassINET || (q.Type != dnsmessage.TypeA && q.Type != dnsmessage.TypeAAAA) {
		return reply(query, dnsmessage.RCodeNotImplemented, nil)
	}
	host, err := egress.CanonicalHost(q.Name.String())
	if err != nil {
		return reply(query, dnsmessage.RCodeFormatError, nil)
	}
	addresses, err := h.lookup.Resolve(ctx, environment, host)
	if err != nil {
		code := dnsmessage.RCodeServerFailure
		if errors.Is(err, core.ErrPolicyDenied) || errors.Is(err, core.ErrApprovalDenied) {
			code = dnsmessage.RCodeRefused
		}
		if errors.Is(err, core.ErrNotFound) {
			code = dnsmessage.RCodeNameError
		}
		return reply(query, code, nil)
	}
	return reply(query, dnsmessage.RCodeSuccess, addresses)
}
