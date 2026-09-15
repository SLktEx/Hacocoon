package gitrepo

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

type recoveryPolicy struct{ denyRead, approveRead bool }

func (p *recoveryPolicy) Evaluate(_ context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	decision := core.PolicyAllow
	if p.denyRead && req.Action == "fetch" {
		decision = core.PolicyDeny
	}
	if p.approveRead && req.Action == "fetch" {
		decision = core.PolicyRequireApproval
	}
	return core.PolicyEvaluation{Decision: decision}, nil
}

type recoveryBackend struct {
	localBackend
	pushes, reads int
	loseAck       bool
	beforePush    func()
}

func (b *recoveryBackend) RunGit(ctx context.Context, req AgentRequest) (Response, error) {
	if req.Operation == "push" {
		b.pushes++
		if b.beforePush != nil {
			b.beforePush()
		}
	}
	if req.Operation == "observe" {
		b.reads++
	}
	result, err := b.localBackend.RunGit(ctx, req)
	if req.Operation == "push" && b.loseAck && err == nil {
		return Response{}, errors.New("injected lost response after actual remote write")
	}
	return result, err
}

type recoveryFixture struct {
	broker     *Broker
	backend    *recoveryBackend
	bound      binding
	proposal   Proposal
	agent      AgentRequest
	auditPath  string
	policy     *recoveryPolicy
	identities *identityEnvironmentStore
}

func newRecoveryFixture(t *testing.T) *recoveryFixture {
	t.Helper()
	if _, err := os.Stat("/usr/bin/git"); err != nil {
		t.Skip("Linux Git required")
	}
	root := t.TempDir()
	remote, repos := filepath.Join(root, "remote.git"), filepath.Join(root, "repos")
	for _, dir := range []string{remote, repos, filepath.Join(repos, "source")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	testGit(t, remote, "init", "--bare", "--initial-branch=main")
	dir := filepath.Join(repos, "source")
	testGit(t, dir, "init", "--initial-branch=main")
	old := testCommit(t, dir, "data", "before")
	testGit(t, dir, "push", "--", "file://"+remote, "HEAD:refs/heads/main")
	newOID := testCommit(t, dir, "data", "after")
	backend := &recoveryBackend{localBackend: localBackend{repos: repos}}
	service := NewRepositoryService(filepath.Join(root, "state"), backend)
	repo := Object{Kind: "repo", ID: "source", Repository: "source", Remote: "file://" + remote, Branch: "main", NativeRef: "test-volume", Owner: strings.Repeat("a", 32), State: "ready"}
	work := repo
	work.Kind, work.ID, work.Owner = "work", "work", strings.Repeat("b", 32)
	for _, obj := range []Object{repo, work} {
		if err := service.save(obj); err != nil {
			t.Fatal(err)
		}
	}
	env := core.Environment{Name: "dev", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:work"}, RuntimeRef: "test:dev"}
	identity, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	f := &recoveryFixture{backend: backend, bound: binding{Environment: env, Workspace: work, Repository: repo}, auditPath: filepath.Join(root, "audit", "capabilities.jsonl"), policy: &recoveryPolicy{}, identities: &identityEnvironmentStore{environment: env, identity: identity}}
	f.proposal = Proposal{Environment: "dev", Repository: repo.ID, Remote: repo.Remote, Ref: "refs/heads/main", OldOID: old, NewOID: newOID, Operation: "push"}
	f.agent = AgentRequest{Operation: "push", Repository: repo.ID, Remote: repo.Remote, Branch: repo.Branch, Ref: f.proposal.Ref, OldOID: old, NewOID: newOID}
	f.restart(t, service)
	return f
}

func (f *recoveryFixture) restart(t *testing.T, service *RepositoryService) {
	t.Helper()
	b := NewBroker(service, f.identities, "")
	audit := capabilityapp.NewJSONLAudit(f.auditPath)
	b.PushAudit, b.AuditHistory = audit, eventsapp.New(f.auditPath)
	var err error
	b.Capabilities, err = capabilityapp.New(f.policy, nil, audit, b)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the validated binding from retained records, without a guest
	// HTTP socket: the test calls the same broker operation directly.
	if err := b.validateBinding(context.Background(), f.bound); err != nil {
		t.Fatal(err)
	}
	b.servers["dev"] = boundServer{binding: f.bound}
	f.broker = b
}

func (f *recoveryFixture) push(ctx context.Context) error {
	_, err := f.broker.perform(ctx, f.bound, f.proposal, func(ctx context.Context) (Response, error) {
		return f.broker.Repositories.RunGit(ctx, f.bound.Repository, f.agent)
	})
	return err
}

func TestPushRecoveryLostAcknowledgmentSurvivesRestartWithoutReplay(t *testing.T) {
	f := newRecoveryFixture(t)
	f.backend.loseAck = true
	if err := f.push(context.Background()); err == nil {
		t.Fatal("lost receipt succeeded")
	}
	f.restart(t, f.broker.Repositories)
	status, err := f.broker.ReconcilePush(context.Background(), "dev", "")
	if err != nil || status.State != "unconfirmed" || status.Observation != "matches-new" || status.Completed == nil || *status.Completed || f.backend.pushes != 1 || f.backend.reads != 1 {
		t.Fatalf("status=%+v pushes=%d reads=%d err=%v", status, f.backend.pushes, f.backend.reads, err)
	}
	if _, err := f.broker.ReconcilePush(context.Background(), "dev", status.RequestID); err != nil || f.backend.pushes != 1 {
		t.Fatalf("replay: %v", err)
	}
	// A new deny applies even though the original push was allowed.
	f.policy.denyRead = true
	if _, err := f.broker.ReconcilePush(context.Background(), "dev", ""); !errors.Is(err, core.ErrPolicyDenied) || f.backend.reads != 2 {
		t.Fatalf("read bypassed deny: %v", err)
	}
}

func TestPushRecoveryConfirmAndGenerationFence(t *testing.T) {
	f := newRecoveryFixture(t)
	if err := f.push(context.Background()); err != nil {
		t.Fatal(err)
	}
	status, err := f.broker.PushStatus(context.Background(), "dev", "")
	if err != nil || status.State != "confirmed" || status.Completed == nil || !*status.Completed {
		t.Fatalf("%+v %v", status, err)
	}
	f.identities.identity, _ = core.NewEnvironmentInstanceID()
	if _, err := f.broker.ReconcilePush(context.Background(), "dev", ""); !errors.Is(err, core.ErrCapabilityStale) || f.backend.reads != 0 {
		t.Fatalf("new generation: %v", err)
	}
}

func TestPushRecoveryRechecksHistoricalGenerationAfterReadApproval(t *testing.T) {
	f := newRecoveryFixture(t)
	if err := f.push(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.policy.approveRead = true
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := f.broker.ReconcilePush(ctx, "dev", ""); done <- err }()
	var pending []Proposal
	for len(pending) == 0 {
		select {
		case err := <-done:
			t.Fatalf("finished before approval: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
		pending = f.broker.Pending()
	}
	f.identities.identity, _ = core.NewEnvironmentInstanceID()
	if err := f.broker.Decide(pending[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, core.ErrCapabilityStale) || f.backend.reads != 0 {
		t.Fatalf("stale read: %v", err)
	}
}

func TestPushRecoveryCompetingIdenticalCreationIsStillUnconfirmed(t *testing.T) {
	f := newRecoveryFixture(t)
	f.agent.Ref, f.proposal.Ref = "refs/heads/new", "refs/heads/new"
	f.agent.OldOID, f.proposal.OldOID = ZeroOID, ZeroOID
	f.backend.beforePush = func() {
		testGit(t, filepath.Join(f.backend.repos, "source"), "push", "--", f.agent.Remote, f.agent.NewOID+":"+f.agent.Ref)
	}
	if err := f.push(context.Background()); err == nil {
		t.Fatal("competing creation succeeded")
	}
	status, err := f.broker.ReconcilePush(context.Background(), "dev", "")
	if err != nil || status.State != "unconfirmed" || status.Observation != "matches-new" || f.backend.pushes != 1 {
		t.Fatalf("%+v %v", status, err)
	}
}

func TestPushRecoveryReportsCurrentOldAbsentAndDivergedWithoutWriting(t *testing.T) {
	for _, observation := range []string{"matches-old", "absent", "diverged"} {
		t.Run(observation, func(t *testing.T) {
			f := newRecoveryFixture(t)
			if err := f.push(context.Background()); err != nil {
				t.Fatal(err)
			}
			remote := strings.TrimPrefix(f.agent.Remote, "file://")
			switch observation {
			case "matches-old":
				testGit(t, remote, "update-ref", f.agent.Ref, f.agent.OldOID)
			case "absent":
				testGit(t, remote, "update-ref", "-d", f.agent.Ref)
			case "diverged":
				dir := filepath.Join(f.backend.repos, "source")
				testCommit(t, dir, "data", "later independent write")
				testGit(t, dir, "push", "--", f.agent.Remote, "HEAD:"+f.agent.Ref)
			}
			status, err := f.broker.ReconcilePush(context.Background(), "dev", "")
			if err != nil || status.Observation != observation || status.State != "confirmed" || f.backend.pushes != 1 || f.backend.reads != 1 {
				t.Fatalf("%+v %v", status, err)
			}
		})
	}
}

func TestObserveHeadRefusesMalformedTargetsAndResponses(t *testing.T) {
	remote := "https://github.com/SLktEx/Hacocoon.git"
	for _, ref := range []string{"--heads", "refs/heads/-option", "refs/heads/a*", "refs/heads/a\nother", "refs/tags/v1"} {
		// A dash inside the literal full ref is safe: it never starts an option.
		if ref == "refs/heads/-option" {
			continue
		}
		calls := 0
		_, err := observeHead(func([]byte, ...string) ([]byte, error) { calls++; return nil, nil }, remote, ref)
		if err == nil || calls != 0 {
			t.Fatalf("target %q reached Git", ref)
		}
	}
	for _, response := range []string{ZeroOID + "\trefs/heads/main\n", strings.Repeat("a", 40) + "\trefs/heads/other\n", strings.Repeat("a", 40) + "\trefs/heads/main\n\n"} {
		_, err := observeHead(func(_ []byte, args ...string) ([]byte, error) {
			if strings.Join(args, " ") != "ls-remote --heads -- "+remote+" refs/heads/main" {
				t.Fatal(args)
			}
			return []byte(response), nil
		}, remote, "refs/heads/main")
		if err == nil {
			t.Fatalf("malformed observation accepted: %q", response)
		}
	}
}

type failingPushAudit struct {
	sink     capabilityapp.AuditSink
	failType string
}

func (a failingPushAudit) Record(ctx context.Context, event core.CapabilityAuditEvent) error {
	if event.Type == a.failType {
		return errors.New("injected audit persistence failure")
	}
	return a.sink.Record(ctx, event)
}

func TestPushRecoveryAuditFailureBoundaries(t *testing.T) {
	for _, phase := range []string{pushStarted, pushConfirmed, "git-push-observed"} {
		t.Run(phase, func(t *testing.T) {
			f := newRecoveryFixture(t)
			f.broker.PushAudit = failingPushAudit{sink: f.broker.PushAudit, failType: phase}
			err := f.push(context.Background())
			if phase == "git-push-observed" {
				if err != nil {
					t.Fatal(err)
				}
				_, err = f.broker.ReconcilePush(context.Background(), "dev", "")
			}
			if !errors.Is(err, core.ErrAuditIncomplete) {
				t.Fatal(err)
			}
			if (phase == pushStarted && f.backend.pushes != 0) || (phase != pushStarted && f.backend.pushes != 1) {
				t.Fatalf("pushes=%d", f.backend.pushes)
			}
			status, err := f.broker.PushStatus(context.Background(), "dev", "")
			if err != nil || (phase != "git-push-observed" && status.State != "unconfirmed") {
				t.Fatalf("%+v %v", status, err)
			}
		})
	}
}

func TestPushRecoveryRefusesActiveAndChangedOwnership(t *testing.T) {
	f := newRecoveryFixture(t)
	f.backend.beforePush = func() {
		status, err := f.broker.PushStatus(context.Background(), "dev", "")
		if err != nil || !status.Active {
			t.Fatalf("active %+v %v", status, err)
		}
		if _, err := f.broker.ReconcilePush(context.Background(), "dev", ""); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal(err)
		}
	}
	if err := f.push(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.bound.Repository.Owner = strings.Repeat("c", 32)
	f.broker.servers["dev"] = boundServer{binding: f.bound}
	if _, err := f.broker.ReconcilePush(context.Background(), "dev", ""); !errors.Is(err, core.ErrCapabilityStale) || f.backend.reads != 0 {
		t.Fatal(err)
	}
}

func TestPushRecoveryRejectsTamperedAndPartialHistory(t *testing.T) {
	for _, kind := range []string{"owner", "oid", "duplicate", "partial"} {
		t.Run(kind, func(t *testing.T) {
			f := newRecoveryFixture(t)
			if err := f.push(context.Background()); err != nil {
				t.Fatal(err)
			}
			var event core.CapabilityAuditEvent
			_, err := f.broker.AuditHistory.StreamAudit(context.Background(), 0, func(e core.CapabilityAuditEvent, _ int64) error {
				if e.Type == pushConfirmed {
					event = e
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			event.Attributes = maps.Clone(event.Attributes)
			switch kind {
			case "owner":
				event.Attributes["repository_owner"] = "other"
			case "oid":
				event.Attributes["new_oid"] = ZeroOID
			case "partial":
				data, err := os.ReadFile(f.auditPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(f.auditPath, data[:len(data)-1], 0600); err != nil {
					t.Fatal(err)
				}
			}
			if kind != "partial" {
				if err := f.broker.PushAudit.Record(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.broker.ReconcilePush(context.Background(), "dev", ""); err == nil || f.backend.reads != 0 {
				t.Fatalf("tampered %s accepted: %v", kind, err)
			}
		})
	}
}
