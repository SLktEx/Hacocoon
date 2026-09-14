package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/desktopreview"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type nativeDisplayFixture struct{ request core.ApprovalRequest }

func TestNativeToastDiagnosticsDoNotExposeProcessOutput(t *testing.T) {
	for _, data := range []string{"SECRET", "HACO_TOAST_FAILURE:show:1\nSECRET", "HACO_TOAST_FAILURE:SECRET:1", "HACO_TOAST_OK"} {
		err := nativeToastResult([]byte(data), errors.New("private process SECRET"))
		if err == nil || strings.Contains(err.Error(), "SECRET") {
			t.Fatal("untrusted native output exposed")
		}
	}
	if err := nativeToastResult([]byte("HACO_TOAST_FAILURE:show:-2146233087"), errors.New("exit")); err == nil || !strings.Contains(err.Error(), "HRESULT -2146233087") {
		t.Fatal("stable native diagnostic lost")
	}
	if err := nativeToastResult([]byte("HACO_TOAST_DISABLED:3"), nil); err == nil {
		t.Fatal("disabled notification accepted")
	}
	if err := nativeToastResult([]byte("HACO_TOAST_OK"), nil); err != nil {
		t.Fatal(err)
	}
}

func (f nativeDisplayFixture) PendingApprovals(context.Context) ([]core.ApprovalRequest, error) {
	return []core.ApprovalRequest{f.request}, nil
}
func (f nativeDisplayFixture) DecideApproval(context.Context, string, capability.ApprovalDecision) (core.CapabilityResult, error) {
	return core.CapabilityResult{}, errors.New("display fixture cannot execute")
}

func TestNativeToastShowHistoryAndRemoval(t *testing.T) {
	if os.Getenv("HACO_TEST_TOAST_DISPLAY") != "1" {
		t.Skip("requires an interactive Windows notification session; no human click is tested")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	appID := "Hacocoon.NotificationTest." + hex.EncodeToString(random[:])
	classText, _ := desktopreview.ClassID("Hacocoon-Native-Test")
	class, _ := windows.GUIDFromString(classText)
	stop, err := startToastCOM(class, appID, make(chan nativeActivation, 4))
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	path := `Software\Classes\AppUserModelId\` + appID
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("test registration already exists")
	}
	defer func() {
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	if err := key.SetStringValue("DisplayName", "Hacocoon notification test"); err != nil {
		key.Close()
		t.Fatal(err)
	}
	if err := key.SetStringValue("CustomActivator", classText); err != nil {
		key.Close()
		t.Fatal(err)
	}
	key.Close()
	plan, err := desktopreview.SessionPlan("Hacocoon-Native-Test", os.Getenv("SystemRoot"))
	if err != nil {
		t.Fatal(err)
	}
	surface := &nativeToastSurface{plan: plan, appID: appID}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 8*time.Second)
		defer done()
		if err := surface.Clear(cleanup); err != nil {
			t.Error(err)
		}
	}()
	// Match ordinary nativeReview startup, including an empty history before
	// the first display. No warm-up or changed Windows settings are supplied.
	if err := surface.Clear(ctx); err != nil {
		t.Fatal("initial owned-history clear", err)
	}
	id := strings.Repeat("a", 32)
	fixture := nativeDisplayFixture{request: core.ApprovalRequest{RequestID: id, CapabilityRequest: core.CapabilityRequest{Environment: "表示確認用Env", EnvironmentInstance: "env-" + strings.Repeat("b", 32), Capability: "network.egress", Action: "connect", Resource: "example.invalid:443", Attributes: map[string]string{"hostname": "example.invalid", "protocol": "tcp", "port": "443"}}}}
	session := desktopreview.Session{Client: fixture}
	reply := session.Handle(ctx, desktopreview.Message{Version: 1, Sequence: 1, Action: "select", RequestID: id})
	if reply.View == nil {
		t.Fatal(reply.Error)
	}
	for _, ja := range []bool{false, true} {
		flow, err := desktopreview.NewToastFlow(*reply.View, ja)
		if err != nil {
			t.Fatal(err)
		}
		var page desktopreview.ToastPage
		for count := 0; count < 100; count++ {
			page, err = flow.Page()
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Inputs) > 0 {
				break
			}
			if err = flow.Displayed(page.Nonce); err != nil {
				t.Fatal(err)
			}
			if _, err = flow.Activate(page.Nonce+":next", nil); err != nil {
				t.Fatal(err)
			}
		}
		if len(page.Inputs) != 2 {
			t.Fatal("missing native selection controls")
		}
		if err := surface.Show(ctx, id, page); err != nil {
			t.Fatal(err)
		}
		// History proves the OS accepted the exact ToastGeneric XML and controls.
		// It does not prove visual wrapping or a human's button activation.
		script := `$ErrorActionPreference='Stop';[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]>$null;$items=@([Windows.UI.Notifications.ToastNotificationManager]::History.GetHistory('` + appID + `'));if($items.Count -ne 1){exit 1};[Console]::Out.Write($items[0].Content.GetXml())`
		command := exec.CommandContext(ctx, os.Getenv("SystemRoot")+`\System32\WindowsPowerShell\v1.0\powershell.exe`, "-NoProfile", "-NonInteractive", "-EncodedCommand", encodeNativeScript(script))
		command.Env = plan.Env
		command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		output, err := command.Output()
		if err != nil {
			t.Fatal("notification history unavailable", err)
		}
		if !bytes.Contains(output, []byte(page.Title)) || !bytes.Contains(output, []byte(`id="scope"`)) || !bytes.Contains(output, []byte(`id="policy"`)) || bytes.Contains(output, []byte(reply.View.Token)) {
			t.Fatal("native history differs from safe displayed choices")
		}
		if err := surface.Remove(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("Windows ToastGeneric English/Japanese selection XML accepted in native history; human activation/visual layout not tested")
}

func TestNativeToastResultPreservesCancellation(t *testing.T) {
	for _, reason := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, output := range []string{"", "HACO_TOAST_OK", "HACO_TOAST_FAILURE:history:-2146233087"} {
			err := nativeToastResult([]byte(output), errors.Join(errors.New("private process SECRET"), reason))
			if !errors.Is(err, reason) || strings.Contains(err.Error(), "SECRET") {
				t.Fatalf("reason lost or private error exposed: %v", err)
			}
		}
	}
}

func TestNativeToastProcessFixture(t *testing.T) {
	switch os.Getenv("HACO_TEST_TOAST_PROCESS") {
	case "deadline":
		os.Stdout.WriteString("HACO_TOAST_OK")
		time.Sleep(30 * time.Second)
		os.Exit(0)
	case "failure":
		_, _ = os.Stderr.WriteString("HACO_TOAST_STAGE:runtime\r\nHACO_TOAST_STAGE:history\r\n")
		_, _ = os.Stdout.WriteString("HACO_TOAST_FAILURE:history:-2146233087")
		os.Exit(7)
	case "private":
		os.Stdout.WriteString("private-page-token")
		os.Stderr.WriteString("private-controller-output")
		os.Exit(9)
	}
}

func TestNativeToastProcessFailureAndReaping(t *testing.T) {
	t.Setenv("HACO_LOG_FORMAT", "text")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"deadline", "failure", "private"} {
		t.Run(kind, func(t *testing.T) {
			timeout := 5 * time.Second
			if kind == "deadline" {
				timeout = 300 * time.Millisecond
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, exe, "-test.run=^TestNativeToastProcessFixture$")
			cmd.Env = append(os.Environ(), "HACO_TEST_TOAST_PROCESS="+kind)
			cmd.WaitDelay = time.Second
			err := runNativeToastCommand(ctx, cmd)
			if err == nil {
				t.Fatal("failed child accepted")
			}
			var process *nativeToastProcessFailure
			if !errors.As(err, &process) || process.durationMS < 0 {
				t.Fatal("numeric process observations missing", err)
			}
			if cmd.ProcessState == nil {
				t.Fatal("child was not reaped")
			}
			if kind == "deadline" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("deadline lost", err)
			}
			if kind == "failure" {
				var native *nativeDisplayFailure
				if process.exitCode != 7 || !errors.As(err, &native) || native.stage != "history" || process.progress != "history" {
					t.Fatal("native failure lost", err)
				}
			}
			if kind == "private" && process.exitCode != 9 {
				t.Fatal("exit lost", process.exitCode)
			}
			var output bytes.Buffer
			reportReviewFailure(&output, &nativeReviewFailure{stage: "clear", cause: err})
			if strings.Contains(output.String(), "private") || !strings.Contains(output.String(), "duration_ms=") || !strings.Contains(output.String(), "exit_code=") {
				t.Fatal("unsafe or missing process observations", output.String())
			}
		})
	}
}

func TestNativeProgressBoundsAndPartialReads(t *testing.T) {
	for _, tc := range []struct {
		chunks []string
		want string
	}{
		{[]string{"HACO_TOAST_STA", "GE:runtime\r\nHACO_TOAST_STAGE:history\n"}, "history"},
		{[]string{"HACO_TOAST_STAGE:input\n"}, "input"},
		{[]string{"HACO_TOAST_STAGE:input"}, "unobserved"},
		{[]string{"HACO_TOAST_STAGE:private-token\n"}, "unobserved"},
		{[]string{"private-output\nHACO_TOAST_STAGE:history\n"}, "unobserved"},
		{[]string{strings.Repeat("x", 513), "HACO_TOAST_STAGE:history\n"}, "unobserved"},
	} {
		var progress nativeProgress
		for _, chunk := range tc.chunks {
			if n, err := progress.Write([]byte(chunk)); err != nil || n != len(chunk) {
				t.Fatal("diagnostic draining blocked")
			}
		}
		if len(progress.data) > 512 || progress.stage() != tc.want {
			t.Fatal("unsafe progress classification")
		}
	}
}
