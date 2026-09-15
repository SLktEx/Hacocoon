package networkrelay

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Service struct {
	Capabilities  Requester
	Targets       Targets
	Authority     Authority
	DialTarget    func(context.Context, Target, netip.Addr) (net.Conn, error)
	Dial          func(context.Context, string, string) (net.Conn, error)
	CheckInterval time.Duration
	mu            sync.Mutex
	sessions      map[string]*active
}

type active struct {
	view     Session
	request  core.CapabilityRequest
	ctx      context.Context
	cancel   context.CancelFunc
	upstream net.Conn
	client   net.Conn
	once     sync.Once
}

type Connection struct {
	Conn    net.Conn
	Session Session
	service *Service
	active  *active
}

func (s *Service) reserve(ctx context.Context, source Source, spec Spec) (*active, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	expires := now.Add(time.Duration(spec.DurationSeconds) * time.Second)
	sessionCtx, cancel := context.WithDeadline(ctx, expires)
	a := &active{view: Session{ID: hex.EncodeToString(token[:]), Source: source, Target: Target{Kind: spec.Kind, Name: spec.Target, Protocol: spec.Protocol, Port: spec.Port}, State: "pending", Phase: "resolve", CreatedAt: now, ExpiresAt: expires}, ctx: sessionCtx, cancel: cancel}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = make(map[string]*active)
	}
	total, own := 0, 0
	for _, existing := range s.sessions {
		if existing.view.State == "closed" || existing.view.State == "failed" {
			continue
		}
		total++
		if existing.view.Source == source {
			own++
		}
	}
	if total >= 128 || own >= 16 {
		cancel()
		return nil, core.ErrStorageBusy
	}
	if len(s.sessions) >= 1024 {
		var oldest *active
		for _, existing := range s.sessions {
			if existing.view.State != "closed" && existing.view.State != "failed" {
				continue
			}
			if oldest == nil || existing.view.CreatedAt.Before(oldest.view.CreatedAt) {
				oldest = existing
			}
		}
		if oldest != nil {
			delete(s.sessions, oldest.view.ID)
		}
	}
	s.sessions[a.view.ID] = a
	return a, nil
}

// Open authorizes one association, never a reusable bearer grant. Lifetime
// begins at submission so an approval cannot silently extend the requested bound.
func (s *Service) Open(ctx context.Context, source Source, spec Spec) (connection *Connection, err error) {
	if s == nil || s.Capabilities == nil || s.Targets == nil || s.Authority == nil {
		return nil, core.ErrPolicyDenied
	}
	if err = validateSpec(spec); err != nil {
		return nil, err
	}
	if !validName(source.Environment) || !core.ValidEnvironmentInstanceID(source.Instance) {
		return nil, core.ErrInvalidArgument
	}
	instance, e := s.Authority.CurrentInstance(ctx, source.Environment)
	if e != nil || instance != source.Instance || !core.ValidEnvironmentInstanceID(instance) {
		return nil, core.ErrCapabilityStale
	}
	a, e := s.reserve(ctx, source, spec)
	if e != nil {
		return nil, e
	}
	context.AfterFunc(a.ctx, func() {
		reason := "canceled"
		if errors.Is(a.ctx.Err(), context.DeadlineExceeded) {
			reason = "expired"
		}
		s.finish(a, reason, false)
	})
	defer func() {
		if err != nil {
			s.finish(a, failureReason(err), true)
		}
	}()
	target, e := s.Targets.Resolve(a.ctx, source, spec)
	if e != nil {
		return nil, e
	}
	if target.Kind != spec.Kind || target.Name != spec.Target || target.Protocol != spec.Protocol || (spec.Port != 0 && target.Port != spec.Port) {
		return nil, core.ErrIncompatibleState
	}
	request, e := requestFor(source, target, spec.DurationSeconds)
	if e != nil {
		return nil, e
	}
	revision, e := s.Authority.PolicyRevision(a.ctx)
	if e != nil {
		return nil, core.ErrPolicyDenied
	}
	s.mu.Lock()
	a.view.Target = cloneTarget(target)
	a.view.PolicyRevision = revision
	a.view.Phase = "authorize"
	a.request = request
	s.mu.Unlock()
	outcome, e := s.Capabilities.Request(a.ctx, request)
	s.mu.Lock()
	a.view.RequestID = outcome.RequestID
	s.mu.Unlock()
	if e != nil {
		return nil, e
	}
	if !outcome.AuditComplete || outcome.ExecutionState != core.CapabilitySucceeded || outcome.RequestID == "" {
		return nil, core.ErrPolicyDenied
	}
	// A saved approval intentionally changes Policy while authorizing this
	// exact request. The Capability service already re-evaluated that saved
	// decision; bind this association to its post-save revision.
	if outcome.SavedChoice != "" {
		revision, e := s.Authority.PolicyRevision(a.ctx)
		if e != nil {
			return nil, core.ErrPolicyDenied
		}
		s.mu.Lock()
		a.view.PolicyRevision = revision
		s.mu.Unlock()
	}
	if e = s.verify(a); e != nil {
		return nil, e
	}
	s.mu.Lock()
	if a.ctx.Err() != nil {
		s.mu.Unlock()
		return nil, a.ctx.Err()
	}
	a.view.State = "connecting"
	a.view.Phase = "connect"
	s.mu.Unlock()
	dial := s.Dial
	if dial == nil {
		dial = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	}
	var upstream net.Conn
	var failures []error
	for _, address := range target.Addresses {
		if s.DialTarget != nil {
			upstream, e = s.DialTarget(a.ctx, cloneTarget(target), address)
		} else {
			upstream, e = dial(a.ctx, spec.Protocol, net.JoinHostPort(address.String(), strconv.Itoa(target.Port)))
		}
		if e == nil {
			break
		}
		failures = append(failures, e)
	}
	if upstream == nil {
		if len(failures) == 0 {
			return nil, core.ErrIncompatibleState
		}
		return nil, errors.Join(failures...)
	}
	s.mu.Lock()
	if a.ctx.Err() != nil {
		s.mu.Unlock()
		_ = upstream.Close()
		return nil, a.ctx.Err()
	}
	a.upstream = upstream
	a.view.Peer = upstream.RemoteAddr().String()
	s.mu.Unlock()
	_ = upstream.SetDeadline(a.view.ExpiresAt)
	if e = s.verify(a); e != nil {
		return nil, e
	}
	if e = s.record(a, "connection-opened", ""); e != nil {
		return nil, core.ErrIncompatibleState
	}
	s.mu.Lock()
	if a.ctx.Err() != nil {
		s.mu.Unlock()
		return nil, a.ctx.Err()
	}
	a.view.State = "active"
	a.view.Phase = "active"
	view := cloneSession(a.view)
	s.mu.Unlock()
	connection = &Connection{Conn: upstream, Session: view, service: s, active: a}
	go s.monitor(a)
	return connection, nil
}

func (s *Service) verify(a *active) error { return s.verifyContext(a.ctx, a) }

func (s *Service) verifyContext(ctx context.Context, a *active) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	instance, err := s.Authority.CurrentInstance(ctx, a.view.Source.Environment)
	if err != nil || instance != a.view.Source.Instance {
		return core.ErrCapabilityStale
	}
	revision, err := s.Authority.PolicyRevision(ctx)
	if err != nil {
		return core.ErrPolicyDenied
	}
	if revision != a.view.PolicyRevision {
		return errPolicyChanged
	}
	evaluation, err := s.Authority.Evaluate(ctx, a.request)
	if err != nil || evaluation.Decision == core.PolicyDeny || (evaluation.Decision != core.PolicyAllow && evaluation.Decision != core.PolicyRequireApproval) {
		return core.ErrPolicyDenied
	}
	return s.Targets.Verify(ctx, a.view.Target)
}

var errDNSResolution = errors.New("name resolution failed")

var errPolicyChanged = errors.New("connection Policy expired or changed")

func (s *Service) monitor(a *active) {
	interval := s.CheckInterval
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			reason := "canceled"
			if errors.Is(a.ctx.Err(), context.DeadlineExceeded) {
				reason = "expired"
			}
			s.finish(a, reason, false)
			return
		case <-ticker.C:
			checkCtx, cancel := context.WithTimeout(a.ctx, interval)
			checked := make(chan error, 1)
			go func() { checked <- s.verifyContext(checkCtx, a) }()
			var err error
			select {
			case err = <-checked:
			case <-checkCtx.Done():
				err = checkCtx.Err()
			}
			cancel()
			if err != nil {
				s.finish(a, failureReason(err), false)
				return
			}
		}
	}
}

func (s *Service) finish(a *active, reason string, failed bool) {
	a.once.Do(func() {
		a.cancel()
		s.mu.Lock()
		upstream, client := a.upstream, a.client
		if !time.Now().Before(a.view.ExpiresAt) {
			reason, failed = "expired", false
		}
		a.view.State = "closed"
		if failed {
			a.view.State = "failed"
		}
		a.view.Reason = reason
		s.mu.Unlock()
		if upstream != nil {
			_ = upstream.Close()
		}
		if client != nil {
			_ = client.Close()
		}
		if err := s.record(a, "connection-closed", reason); err != nil {
			s.mu.Lock()
			a.view.State = "failed"
			a.view.Reason = "audit_failed"
			s.mu.Unlock()
		}
	})
}
func (s *Service) record(a *active, kind, reason string) error {
	s.mu.Lock()
	view := cloneSession(a.view)
	request := a.request
	s.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	id := view.RequestID
	if id == "" {
		id = view.ID
	}
	return s.Authority.Record(ctx, core.CapabilityAuditEvent{
		Time: time.Now().UTC(), RequestID: id, Type: kind, Capability: Capability, Action: Action,
		Environment: view.Source.Environment, EnvironmentInstance: view.Source.Instance,
		Resource: view.Target.Name, Attributes: connectionAuditAttributes(request.Attributes, view.ID), Reason: reason,
	})
}
func (c *Connection) BindClient(client net.Conn) error {
	c.service.mu.Lock()
	if c.active.ctx.Err() != nil {
		c.service.mu.Unlock()
		_ = client.Close()
		return core.ErrCapabilityStale
	}
	_ = client.SetDeadline(c.active.view.ExpiresAt)
	c.active.client = client
	c.service.mu.Unlock()
	return nil
}
func (c *Connection) Close()             { c.service.finish(c.active, "completed", false) }
func (c *Connection) Fail(reason string) { c.service.finish(c.active, reason, true) }
func (c *Connection) Validate() error    { return c.service.verify(c.active) }

func (s *Service) Revoke(id string) error {
	s.mu.Lock()
	a := s.sessions[id]
	s.mu.Unlock()
	if a == nil {
		return core.ErrNotFound
	}
	s.finish(a, "revoked", false)
	return nil
}
func (s *Service) List() []Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Session, 0, len(s.sessions))
	for _, a := range s.sessions {
		out = append(out, cloneSession(a.view))
	}
	return out
}
func (s *Service) Close() {
	s.mu.Lock()
	sessions := make([]*active, 0, len(s.sessions))
	for _, a := range s.sessions {
		sessions = append(sessions, a)
	}
	s.mu.Unlock()
	for _, a := range sessions {
		s.finish(a, "controller_stopped", false)
	}
}
func cloneTarget(t Target) Target    { t.Addresses = append(t.Addresses[:0:0], t.Addresses...); return t }
func cloneSession(s Session) Session { s.Target = cloneTarget(s.Target); return s }

func failureReason(err error) string {
	switch {
	case errors.Is(err, core.ErrNotFound):
		return "target_not_found"
	case errors.Is(err, errDNSResolution):
		return "dns_failed"
	case errors.Is(err, core.ErrStorageBusy):
		return "limit_reached"
	case errors.Is(err, core.ErrInvalidArgument):
		return "invalid_request"
	case errors.Is(err, errPolicyChanged):
		return "policy_expired_or_changed"
	case errors.Is(err, core.ErrApprovalDenied):
		return "approval_denied"
	case errors.Is(err, core.ErrPolicyDenied):
		return "policy_denied"
	case errors.Is(err, core.ErrCapabilityStale):
		return "identity_changed"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "connection_refused"
	case errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EHOSTUNREACH):
		return "unreachable"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns_failed"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "timeout"
	}
	return "connection_failed"
}

// A correlation identifier is observation metadata, never connection authority.
func connectionAuditAttributes(attributes map[string]string, id string) map[string]string {
	out := make(map[string]string, len(attributes)+1)
	for k, v := range attributes {
		out[k] = v
	}
	out["connection_id"] = id
	return out
}
