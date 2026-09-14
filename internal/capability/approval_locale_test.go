package capability

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func localeApprovalRequest() core.ApprovalRequest {
	return core.ApprovalRequest{CapabilityRequest: core.CapabilityRequest{
		Capability: "local.echo", Action: "echo", Resource: "target", Environment: "dev",
		EnvironmentInstance: "env-11111111111111111111111111111111",
	}}
}

func TestLocalizedApprovalKeepsDecisionsAndScopes(t *testing.T) {
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		for _, tc := range []struct {
			input    string
			approved bool
			save     SavedChoice
		}{
			{"\n", false, ""}, {"", false, ""}, {"N\n", false, ""},
			{"y\n", true, ""}, {"YES\n", true, ""}, {"はい\n", false, ""},
			{"1\n", true, AllowEnvironment}, {"2\n", false, DenyEnvironment},
			{"3\n", true, AllowGlobal}, {"4\n", false, DenyGlobal},
			{"5\nyes\n", true, AskEnvironment}, {"5\nno\n", false, AskEnvironment},
			{"6\nyes\n", true, AskGlobal}, {"6\n", false, AskGlobal},
		} {
			t.Run(string(language)+"/"+tc.input, func(t *testing.T) {
				var out bytes.Buffer
				decision, err := NewLocalizedStdioApproval(strings.NewReader(tc.input), &out, language).Decide(context.Background(), localeApprovalRequest())
				if err != nil || decision.Approved != tc.approved || decision.Save != tc.save {
					t.Fatalf("decision=%+v error=%v", decision, err)
				}
				for _, value := range []string{"local.echo", "action=echo", "resource=target", "environment=dev", "1=", "2=", "3=", "4=", "5=", "6="} {
					if !strings.Contains(out.String(), value) {
						t.Errorf("missing %q in %q", value, out.String())
					}
				}
				if language == cliui.Japanese {
					if !strings.Contains(out.String(), "未入力は拒否") || !strings.Contains(out.String(), "すべてのEnvironment") {
						t.Fatal(out.String())
					}
					if strings.Contains(out.String(), "Approve capability") {
						t.Fatal("nested prompt lost Japanese language")
					}
				}
			})
		}
	}
}

func TestLocalizedLegacyApprovalCannotSave(t *testing.T) {
	approved, err := NewLocalizedStdioApproval(strings.NewReader("1\n"), io.Discard, cliui.Japanese).Approve(context.Background(), localeApprovalRequest())
	if err != nil || approved {
		t.Fatalf("numeric legacy input granted authority: %v %v", approved, err)
	}
}

func TestLocalizedApprovalKeepsTerminalEscaping(t *testing.T) {
	request := localeApprovalRequest()
	request.CapabilityRequest.Resource = "target\x1b[31m\n forged-prompt"
	request.CapabilityRequest.Attributes = map[string]string{"arg\n": "value\x00\x1b[2J"}
	request.Reason = "reason\r\n\x1b[2J"
	var out bytes.Buffer
	_, err := NewLocalizedStdioApproval(strings.NewReader("\n"), &out, cliui.Japanese).Decide(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"\x1b", "\x00", "\r", "\n"} {
		if strings.Contains(out.String(), unsafe) {
			t.Fatalf("unsafe terminal output: %q", out.String())
		}
	}
}

type localeBrokenApprovalWriter struct{}

func (localeBrokenApprovalWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestLocalizedApprovalDisplayFailureDoesNotGrant(t *testing.T) {
	decision, err := NewLocalizedStdioApproval(strings.NewReader("y\n"), localeBrokenApprovalWriter{}, cliui.Japanese).Decide(context.Background(), localeApprovalRequest())
	if !errors.Is(err, io.ErrClosedPipe) || decision.Approved || decision.Save != "" {
		t.Fatalf("decision=%+v error=%v", decision, err)
	}
}

func TestLocalizedApprovalInstancesDoNotShareLanguage(t *testing.T) {
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		t.Run(string(language), func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 20; i++ {
				var out bytes.Buffer
				decision, err := NewLocalizedStdioApproval(strings.NewReader("y\n"), &out, language).Decide(context.Background(), localeApprovalRequest())
				if err != nil || !decision.Approved {
					t.Fatalf("decision=%+v error=%v", decision, err)
				}
				want := "Approve capability"
				if language == cliui.Japanese {
					want = "次の操作の承認"
				}
				if !strings.HasPrefix(out.String(), want) {
					t.Fatalf("language leak: %q", out.String())
				}
			}
		})
	}
}
