package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEnvironmentStatusLocalizedWithoutChangingValues(t *testing.T) {
	status := core.EnvironmentStatus{Environment: core.Environment{Name: "dev", Workspace: core.Workspace{Path: "/work-日本語"}}, State: core.EnvironmentStopped}
	before, _ := json.Marshal(status)
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		var out bytes.Buffer
		if code := writeEnvironmentStatusLanguage(&out, status, language); code != 0 {
			t.Fatal(code)
		}
		if !strings.Contains(out.String(), "/work-日本語") || !strings.Contains(out.String(), string(core.EnvironmentStopped)) {
			t.Fatal("resource or stable state value changed", out.String())
		}
		if language == cliui.English {
			want := "Environment: dev\nState:       stopped\nWorkspace:   /work-日本語\nAccess:      \nWorkspace retained; this Environment is stopped.\n"
			if out.String() != want {
				t.Fatalf("English output changed: %q", out.String())
			}
		} else if !strings.Contains(out.String(), "開発環境: dev") || !strings.Contains(out.String(), "作業データは保持") {
			t.Fatal("Japanese status missing", out.String())
		}
	}
	after, _ := json.Marshal(status)
	if !bytes.Equal(before, after) {
		t.Fatal("rendering mutated the status")
	}
}

func TestEnvironmentListLocalesRetainInputOrderAndEscaping(t *testing.T) {
	environments := []core.Environment{{Name: "z-last", Workspace: core.Workspace{Path: "/repo\x1b[31m"}}, {Name: "a-first", Workspace: core.Workspace{Path: "/日本語"}}}
	before, _ := json.Marshal(environments)
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		var out bytes.Buffer
		if err := writeEnvironmentListLanguage(&out, environments, language); err != nil {
			t.Fatal(err)
		}
		text := out.String()
		if !strings.Contains(text, "a-first") || !strings.Contains(text, "z-last") || strings.Index(text, "a-first") > strings.Index(text, "z-last") || strings.Contains(text, "\x1b") || !strings.Contains(text, "/日本語") || !strings.Contains(text, "haco open <name>") {
			t.Fatal("sorting, escaping or command spelling changed", text)
		}
		if language == cliui.Japanese && !strings.Contains(text, "作業場所") {
			t.Fatal("Japanese list header missing")
		}
		out.Reset()
		if err := writeEnvironmentListLanguage(&out, nil, language); err != nil || !strings.Contains(out.String(), "haco env create --workspace <workspace> <name>") {
			t.Fatal("empty list lost the next command", err, out.String())
		}
	}
	after, _ := json.Marshal(environments)
	if !bytes.Equal(before, after) {
		t.Fatal("list rendering reordered or changed caller data")
	}
}

func TestEnvironmentDoctorLocaleKeepsJSONAndProbeContract(t *testing.T) {
	for _, tc := range []struct {
		name         string
		state        core.EnvironmentState
		fail         bool
		code, probes int
	}{
		{"running", core.EnvironmentRunning, false, 0, 3},
		{"stopped", core.EnvironmentStopped, false, 1, 0},
		{"failed probes", core.EnvironmentRunning, true, 1, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &doctorEnvironmentFixture{state: tc.state, fail: tc.fail}
			report, err := diagnoseEnvironment(context.Background(), fixture, "dev")
			if err != nil || fixture.calls != tc.probes {
				t.Fatal("diagnostic execution changed", err, fixture.calls)
			}
			before, _ := json.Marshal(report)
			for _, check := range report.Checks {
				if got := environmentDoctorAction(cliui.English, report, check); got != displayCell(check.Action) {
					t.Fatalf("English action changed: %q != %q", got, check.Action)
				}
			}
			for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
				var human, machine bytes.Buffer
				if writeEnvironmentDoctorLanguage(&human, report, false, language) != tc.code || writeEnvironmentDoctorLanguage(&machine, report, true, language) != tc.code {
					t.Fatal("locale changed the diagnostic exit code")
				}
				if !bytes.Equal(machine.Bytes(), append(append([]byte(nil), before...), '\n')) || strings.Contains(machine.String(), "actionMessage") || strings.Contains(machine.String(), "scopeMessage") {
					t.Fatal("presentation metadata leaked into JSON", machine.String())
				}
				if strings.Contains(human.String(), "do-not-render") || strings.Contains(human.String(), "private backend") || strings.Contains(human.String(), "env.doctor.") || strings.Contains(human.String(), "%!") {
					t.Fatal("redaction or message selection failed", human.String())
				}
				if language == cliui.Japanese {
					if !strings.Contains(human.String(), "未検証") || !strings.Contains(human.String(), "開発環境: dev") {
						t.Fatal("Japanese diagnostic scope missing", human.String())
					}
					if tc.state == core.EnvironmentStopped && !strings.Contains(human.String(), "haco env start dev") {
						t.Fatal("start command was translated or lost")
					}
					if tc.fail && (!strings.Contains(human.String(), "削除しないでください") || !strings.Contains(human.String(), "haco ssh setup dev")) {
						t.Fatal("recovery or data-retention warning missing", human.String())
					}
				} else if !strings.Contains(human.String(), report.Scope) {
					t.Fatal("English diagnostic scope changed")
				}
			}
			after, _ := json.Marshal(report)
			if !bytes.Equal(before, after) || fixture.calls != tc.probes {
				t.Fatal("rendering changed diagnostic data or reran probes")
			}
		})
	}
}

type environmentLocaleWriter struct{ writes, failAt int }

func (w *environmentLocaleWriter) Write(p []byte) (int, error) {
	position := w.writes
	w.writes++
	if position == w.failAt {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func TestEnvironmentDoctorFailsOnEveryOutputBoundary(t *testing.T) {
	fixture := &doctorEnvironmentFixture{state: core.EnvironmentRunning}
	report, err := diagnoseEnvironment(context.Background(), fixture, "dev")
	if err != nil {
		t.Fatal(err)
	}
	// Also exercise the next-action write and the fallback for older reports.
	report.Checks[0].Action = "Inspect local service before continuing"
	for _, language := range []cliui.Language{cliui.English, cliui.Japanese} {
		count := &environmentLocaleWriter{failAt: -1}
		if writeEnvironmentDoctorLanguage(count, report, false, language) != 0 {
			t.Fatal("healthy report failed")
		}
		for at := 0; at < count.writes; at++ {
			broken := &environmentLocaleWriter{failAt: at}
			if writeEnvironmentDoctorLanguage(broken, report, false, language) != 1 {
				t.Fatalf("ignored write failure for %s at write %d", language, at)
			}
		}
		if writeEnvironmentStatusLanguage(&environmentLocaleWriter{failAt: 0}, core.EnvironmentStatus{}, language) != 1 {
			t.Fatal("status output failure ignored")
		}
		if writeEnvironmentListLanguage(&environmentLocaleWriter{failAt: 0}, []core.Environment{{Name: "dev"}}, language) == nil {
			t.Fatal("list output failure ignored")
		}
	}
}

func TestEnvironmentLocaleRenderingIsParallel(t *testing.T) {
	status := core.EnvironmentStatus{Environment: core.Environment{Name: "dev"}, State: core.EnvironmentStopped}
	for i := 0; i < 24; i++ {
		language, want := cliui.English, "Environment: dev"
		if i%2 != 0 {
			language, want = cliui.Japanese, "開発環境: dev"
		}
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			for j := 0; j < 50; j++ {
				var out bytes.Buffer
				if writeEnvironmentStatusLanguage(&out, status, language) != 0 || !strings.HasPrefix(out.String(), want) {
					t.Fatal("language leaked between calls", out.String())
				}
			}
		})
	}
}

func TestEnvironmentTransferUsageLocales(t *testing.T) {
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "")
		for _, command := range []struct {
			name string
			run  func(context.Context, []string, io.Writer, io.Writer) int
		}{{"copy", copyEnvironment}, {"import", importEnvironment}, {"export", exportEnvironment}} {
			for _, args := range [][]string{nil, {"--help"}, {"--unknown-option"}} {
				var out, diagnostic bytes.Buffer
				wantCode := 2
				if len(args) > 0 && args[0] == "--help" {
					wantCode = 0
				}
				if got := command.run(context.Background(), args, &out, &diagnostic); got != wantCode || out.Len() != 0 {
					t.Fatal(command.name, locale, got, out.String())
				}
				prefix := "Usage: haco env "
				if locale != "C" {
					prefix = "使い方: haco env "
				}
				if !strings.Contains(diagnostic.String(), prefix+command.name) {
					t.Fatal("localized usage missing", diagnostic.String())
				}
				if len(args) > 0 && args[0] == "--unknown-option" && !strings.Contains(diagnostic.String(), "flag provided but not defined: -unknown-option") {
					t.Fatal("original flag detail lost", diagnostic.String())
				}
			}
		}
	}
}

func TestEnvironmentCopyLocalePreservesFailureJSONAndOriginalError(t *testing.T) {
	server := control.NewServer()
	if err := server.Register(controlapi.MethodEnvironmentCopy, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]any{
			"result": map[string]string{"environment": "dev-restored", "workspace": "restore-owned", "state": "running"},
			"error":  map[string]string{"code": "recovery_required", "message": "owned residue retained / original-%s"},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "locale.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	var previous string
	for _, locale := range []string{"C", "ja_JP.UTF-8"} {
		t.Setenv("LC_ALL", locale)
		t.Setenv("LC_MESSAGES", "")
		t.Setenv("LANG", "")
		var out, diagnostic bytes.Buffer
		if copyEnvironment(ctx, []string{"--json", "dev"}, &out, &diagnostic) != 1 {
			t.Fatal("failed copy became successful")
		}
		if !json.Valid(out.Bytes()) || !strings.Contains(diagnostic.String(), "original-%s") {
			t.Fatal("JSON or original error detail changed", out.String(), diagnostic.String())
		}
		if previous != "" && previous != out.String() {
			t.Fatal("language changed copy JSON")
		}
		previous = out.String()
		if locale != "C" && !strings.Contains(diagnostic.String(), "コピーに失敗") {
			t.Fatal("Japanese copy failure missing")
		}
	}
}
