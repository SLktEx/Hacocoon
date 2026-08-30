package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/egressproxy"
)

const (
	realEgressFakeIP   = "203.0.114.1"
	realEgressFakeHost = "haco-egress-e2e.test"
)

type e2eApprovalQueue struct {
	decisions chan bool
}

func (a *e2eApprovalQueue) Approve(ctx context.Context, _ core.ApprovalRequest) (bool, error) {
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case decision := <-a.decisions:
		return decision, nil
	}
}

func TestRealIncusEgressProxyE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS") != "1" {
		t.Skip("set HACO_E2E_INCUS=1 on a supported Incus host")
	}
	if goruntime.GOOS != "linux" {
		t.Skip("real Incus E2E requires Linux/WSL2")
	}
	if _, err := exec.LookPath("incus"); err != nil {
		t.Fatalf("incus CLI not found: %v", err)
	}

	ctx := context.Background()
	runner := e2eLoggingRunner{t: t, inner: host.ExecRunner{}}
	if result, err := runner.Run(ctx, "incus", "version"); err != nil {
		t.Fatalf("Incus daemon is not usable: %v\n%s", err, result.Stderr)
	}

	project := fmt.Sprintf("haco-e2e-egress-%d", time.Now().UnixNano())
	runtimeAdapter := New(runner)
	runtimeAdapter.project = project
	if err := runtimeAdapter.Prepare(ctx, core.RuntimePrepareSpec{StorageAttachment: map[string]string{"incus_pool": "default"}}); err != nil {
		t.Fatalf("prepare Incus runtime: %v", err)
	}

	workspaceDir := filepath.Join(t.TempDir(), "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	provider, err := NewSandboxProvider(runtimeAdapter)
	if err != nil {
		t.Fatalf("create production Incus sandbox provider: %v", err)
	}
	service := workspaceapp.New(provider, store)
	instanceRef := "haco-egress"
	defer func() {
		cleanupCtx := context.Background()
		_, _ = runner.Run(cleanupCtx, "incus", "delete", instanceRef, "--project", project, "--force")
		_, _ = runner.Run(cleanupCtx, "incus", "project", "delete", project)
	}()

	environment, err := service.Create(ctx, core.EnvironmentSpec{Name: "egress", WorkspacePath: workspaceDir})
	if err != nil {
		t.Fatalf("create egress environment: %v", err)
	}
	if environment.RuntimeRef != instanceRef {
		t.Fatalf("runtime ref = %q, want %q", environment.RuntimeRef, instanceRef)
	}

	hostsMarker := fmt.Sprintf("haco-egress-e2e-%d", time.Now().UnixNano())
	if _, err := runner.Run(ctx, "sudo", "ip", "address", "add", realEgressFakeIP+"/32", "dev", "lo"); err != nil {
		t.Fatalf("add local fake-public upstream address: %v", err)
	}
	defer func() {
		_, _ = runner.Run(context.Background(), "sudo", "ip", "address", "del", realEgressFakeIP+"/32", "dev", "lo")
	}()

	hostsLine := fmt.Sprintf("%s %s # %s", realEgressFakeIP, realEgressFakeHost, hostsMarker)
	addHosts := fmt.Sprintf("printf '%%s\\n' '%s' >> /etc/hosts", hostsLine)
	if _, err := runner.Run(ctx, "sudo", "sh", "-c", addHosts); err != nil {
		t.Fatalf("install fake-public upstream hostname: %v", err)
	}
	defer func() {
		removeHosts := fmt.Sprintf("tmp=$(mktemp); { grep -v -F -- '# %s' /etc/hosts || true; } > \"$tmp\"; cat \"$tmp\" > /etc/hosts; rm -f \"$tmp\"", hostsMarker)
		_, _ = runner.Run(context.Background(), "sudo", "sh", "-c", removeHosts)
	}()

	upstreamListener, err := net.Listen("tcp4", net.JoinHostPort(realEgressFakeIP, "0"))
	if err != nil {
		t.Fatalf("listen on fake-public upstream: %v", err)
	}
	upstreamPort := upstreamListener.Addr().(*net.TCPAddr).Port
	var upstreamHits atomic.Int32
	upstreamServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		if r.URL.Path != "/ok" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("haco-egress-ok\n"))
	})}
	upstreamDone := make(chan error, 1)
	go func() { upstreamDone <- upstreamServer.Serve(upstreamListener) }()
	defer func() {
		_ = upstreamServer.Close()
		if serveErr := <-upstreamDone; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Errorf("fake-public upstream server: %v", serveErr)
		}
	}()

	policyDir := t.TempDir()
	policyPath := filepath.Join(policyDir, "policy.json")
	auditPath := filepath.Join(policyDir, "audit", "capabilities.jsonl")
	approvals := &e2eApprovalQueue{decisions: make(chan bool, 2)}
	approvals.decisions <- false
	approvals.decisions <- true
	capabilities, err := capabilityapp.New(
		capabilityapp.NewFilePolicyEvaluator(policyPath),
		approvals,
		capabilityapp.NewJSONLAudit(auditPath),
		egressapp.Provider{},
	)
	if err != nil {
		t.Fatalf("compose egress capability boundary: %v", err)
	}
	egressBroker := egressapp.NewBroker(capabilities)
	sources, err := egressapp.NewPersistedSourceResolver(runtimeAdapter, store)
	if err != nil {
		t.Fatalf("compose persisted egress source resolver: %v", err)
	}
	proxy := egressproxy.New(egressBroker, sources)
	proxyAddress, err := runtimeAdapter.PrepareEgressProxy(ctx)
	if err != nil {
		t.Fatalf("prepare real Incus egress proxy listener: %v", err)
	}
	proxyListener, err := net.Listen("tcp", proxyAddress)
	if err != nil {
		t.Fatalf("listen on real Incus egress gateway %s: %v", proxyAddress, err)
	}
	proxyServer := &http.Server{Handler: proxy}
	proxyDone := make(chan error, 1)
	go func() { proxyDone <- proxyServer.Serve(proxyListener) }()
	defer func() {
		_ = proxyServer.Close()
		if serveErr := <-proxyDone; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			t.Errorf("real Incus egress proxy server: %v", serveErr)
		}
	}()

	proxyHost, proxyPort, err := net.SplitHostPort(proxyAddress)
	if err != nil {
		t.Fatalf("parse proxy address %q: %v", proxyAddress, err)
	}

	// The managed Incus ACL must reject raw bypass even though the fake upstream
	// is locally reachable from the Physical Host.
	bypassScript := fmt.Sprintf("exec 3<>/dev/tcp/%s/%d", realEgressFakeIP, upstreamPort)
	bypassResult, bypassErr := service.Exec(ctx, "egress", core.ExecutionRequest{Argv: []string{"timeout", "3", "bash", "-lc", bypassScript}})
	if bypassErr == nil || bypassResult.ExitCode == 0 {
		t.Fatalf("raw network bypass unexpectedly reached upstream: result=%#v err=%v", bypassResult, bypassErr)
	}
	if got := upstreamHits.Load(); got != 0 {
		t.Fatalf("raw bypass contacted upstream %d times", got)
	}

	writeRealEgressPolicy(t, policyPath, upstreamPort, core.PolicyDeny)
	denied := executeRealEgressHTTP(t, ctx, service, proxyHost, proxyPort, upstreamPort)
	if !strings.Contains(denied, " 403 Forbidden") {
		t.Fatalf("policy deny response did not contain 403:\n%s", denied)
	}
	if got := upstreamHits.Load(); got != 0 {
		t.Fatalf("policy-denied request contacted upstream %d times", got)
	}

	writeRealEgressPolicy(t, policyPath, upstreamPort, core.PolicyAllow)
	allowed := executeRealEgressHTTP(t, ctx, service, proxyHost, proxyPort, upstreamPort)
	if !strings.Contains(allowed, " 200 OK") || !strings.Contains(allowed, "haco-egress-ok") {
		t.Fatalf("policy allow response did not reach upstream:\n%s", allowed)
	}
	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("allowed request upstream hits = %d, want 1", got)
	}

	writeRealEgressPolicy(t, policyPath, upstreamPort, core.PolicyRequireApproval)
	approvalDenied := executeRealEgressHTTP(t, ctx, service, proxyHost, proxyPort, upstreamPort)
	if !strings.Contains(approvalDenied, " 403 Forbidden") {
		t.Fatalf("approval denial response did not contain 403:\n%s", approvalDenied)
	}
	if got := upstreamHits.Load(); got != 1 {
		t.Fatalf("approval-denied request contacted upstream; hits = %d", got)
	}

	approvalAllowed := executeRealEgressHTTP(t, ctx, service, proxyHost, proxyPort, upstreamPort)
	if !strings.Contains(approvalAllowed, " 200 OK") || !strings.Contains(approvalAllowed, "haco-egress-ok") {
		t.Fatalf("approved request did not reach upstream:\n%s", approvalAllowed)
	}
	if got := upstreamHits.Load(); got != 2 {
		t.Fatalf("approved request upstream hits = %d, want 2", got)
	}

	audit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read egress audit log: %v", err)
	}
	for _, marker := range []string{
		`"decision":"deny"`,
		`"decision":"allow"`,
		`"decision":"require-approval"`,
		`"approved":false`,
		`"approved":true`,
	} {
		if !strings.Contains(string(audit), marker) {
			t.Fatalf("egress audit missing %s:\n%s", marker, audit)
		}
	}

	if err := service.Delete(ctx, "egress"); err != nil {
		t.Fatalf("delete egress environment: %v", err)
	}
}

func executeRealEgressHTTP(t *testing.T, ctx context.Context, service *workspaceapp.Service, proxyHost, proxyPort string, upstreamPort int) string {
	t.Helper()
	requestScript := fmt.Sprintf(
		"exec 3<>/dev/tcp/%s/%s; printf 'GET http://%s:%d/ok HTTP/1.1\\r\\nHost: %s:%d\\r\\nConnection: close\\r\\n\\r\\n' >&3; cat <&3",
		proxyHost, proxyPort, realEgressFakeHost, upstreamPort, realEgressFakeHost, upstreamPort,
	)
	result, err := service.Exec(ctx, "egress", core.ExecutionRequest{Argv: []string{"timeout", "10", "bash", "-lc", requestScript}})
	if err != nil || result.ExitCode != 0 {
		t.Fatalf("execute Environment -> Hacocoon proxy request: result=%#v err=%v", result, err)
	}
	return result.Stdout
}

func writeRealEgressPolicy(t *testing.T, path string, upstreamPort int, decision core.PolicyDecision) {
	t.Helper()
	policy := capabilityapp.PolicyFile{
		Default: core.PolicyDeny,
		Rules: []capabilityapp.PolicyRule{
			{
				Capability:  egressapp.Capability,
				Action:      egressapp.ActionConnect,
				Resource:    realEgressFakeHost,
				Environment: "egress",
				Attributes: map[string]string{
					"protocol": string(core.EgressHTTP),
					"port":     strconv.Itoa(upstreamPort),
				},
				Decision: decision,
				Reason:   "real Incus egress E2E",
			},
		},
	}
	content, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("encode egress policy: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write egress policy: %v", err)
	}
}
