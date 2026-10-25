// Package reclaimnative is the thin public Windows-native reclamation frontend.
// Native enrollment and all mutation authority remain in the installed helper.
package reclaimnative

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
)

const usage = "haco-reclaim.exe --distribution <name> [--yes | --status]"

type options struct {
	distribution      string
	yes, status, help bool
}

func parseOptions(args []string) (options, error) {
	var o options
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		o.help = true
		return o, nil
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--distribution":
			if o.distribution != "" || i+1 == len(args) {
				return o, errors.New("distribution required exactly once")
			}
			i++
			o.distribution = args[i]
			if !validDistribution(o.distribution) {
				return o, errors.New("invalid distribution name")
			}
		case "--yes":
			if o.yes || o.status {
				return o, errors.New("conflicting mode")
			}
			o.yes = true
		case "--status":
			if o.status || o.yes {
				return o, errors.New("conflicting mode")
			}
			o.status = true
		default:
			return o, errors.New("unknown argument")
		}
	}
	if o.distribution == "" {
		return o, errors.New("explicit distribution required")
	}
	return o, nil
}

// Same narrow name shape as the managed Windows installer. A name only selects
// a candidate; the native resolver must independently establish saved enrollment.
func validDistribution(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for i, c := range []byte(name) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && (c == '-' || c == '_' || c == '.') {
			continue
		}
		return false
	}
	return true
}

type savedStatus struct {
	operation, state string
	document         []byte
}
type helper interface {
	prepare(context.Context, string) ([]byte, error)
	launch(context.Context, string, string) ([]byte, error)
	close() error
}
type actions struct {
	resolve func(string) (string, error)
	latest  func(context.Context, string) (savedStatus, error)
	open    func(string) (helper, error)
	confirm func(context.Context, io.Reader, io.Writer, cliui.Language) (bool, error)
}

func run(ctx context.Context, args []string, in io.Reader, out, diagnostic io.Writer, language cliui.Language, a actions) int {
	o, err := parseOptions(args)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, usage)
		return 2
	}
	if o.help {
		if _, err := fmt.Fprintln(out, usage); err != nil {
			return 1
		}
		if _, err := fmt.Fprintln(out, nativeText(language, "help")); err != nil {
			return 1
		}
		return 0
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, language.Text("reclaim.text.canceled_before"))
		return 1
	}
	registration, err := a.resolve(o.distribution)
	if err != nil || !validGUID(registration) {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "unavailable"))
		return 1
	}
	if o.status {
		result, err := a.latest(ctx, registration)
		if err != nil || !validSavedStatus(result) {
			_, _ = fmt.Fprintln(diagnostic, nativeText(language, "status_failed"))
			return 1
		}
		if _, err = out.Write(append(result.document, '\n')); err != nil {
			return 1
		}
		if result.state == "failed" || result.state == "interrupted" {
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintln(out, language.Text("reclaim.text.stop_warning")); err != nil {
		return 1
	}
	if !o.yes {
		confirmed, err := a.confirm(ctx, in, out, language)
		if err != nil {
			_, _ = fmt.Fprintln(diagnostic, nativeText(language, "confirmation_unavailable"))
			return 1
		}
		if !confirmed {
			if _, err := fmt.Fprintln(out, language.Text("reclaim.text.canceled")); err != nil {
				return 1
			}
			return 0
		}
	}
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, language.Text("reclaim.text.canceled_before"))
		return 1
	}
	h, err := a.open(registration)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "unavailable"))
		return 1
	}
	// Close only releases native observation/file handles, never signals helpers.
	// Handle-release failure must not turn an uncertain dispatch into success.
	code := startFresh(ctx, registration, h, out, diagnostic, language)
	if err := h.close(); err != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	return code
}

func startFresh(ctx context.Context, registration string, h helper, out, diagnostic io.Writer, language cliui.Language) int {
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, language.Text("reclaim.text.canceled_before"))
		return 1
	}
	raw, err := h.prepare(ctx, registration)
	if err != nil || ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	operation, err := decodePrepared(raw)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	// Never substitute an old/current ID after a missing reply. Preparation may
	// have persisted intent even when canceled or unable to publish its receipt.
	if ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	raw, err = h.launch(ctx, registration, operation)
	if err != nil || ctx.Err() != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	if _, err = decodeLaunch(raw); err != nil {
		_, _ = fmt.Fprintln(diagnostic, nativeText(language, "uncertain"))
		return 1
	}
	if _, err = fmt.Fprintf(out, "%s\n%s\n", operation, nativeText(language, "dispatched")); err != nil {
		return 1
	}
	return 0
}

func validSavedStatus(s savedStatus) bool {
	if len(s.document) == 0 || len(s.document) > 16384 || !json.Valid(s.document) {
		return false
	}
	switch s.state {
	case "none":
		return s.operation == ""
	case "pending", "failed", "complete", "interrupted":
		return validGUID(s.operation)
	default:
		return false
	}
}

func nativeText(language cliui.Language, key string) string {
	messages := map[string][2]string{
		"confirmation_unavailable": {"Confirmation unavailable; no Windows operation was invoked. Use an interactive Windows console, or --yes to explicitly confirm this fresh request.", "確認を受け取れなかったため、Windowsの操作は開始していません。対話可能なWindowsコンソールを使用するか、今回の新しい要求を明示的に確認する場合は --yes を指定してください。"},
		"help":                     {"Starts one fresh operation after confirmation. --yes confirms this request; --status reads the saved native result without starting WSL. Interactive input requires a Windows console; redirected input requires --yes. No automatic retry or review.", "確認後に新しい処理を1回だけ開始します。--yes は今回の確認を省略し、--status はWSLを起動せず保存結果を読み取ります。対話入力にはWindowsコンソールが必要です。入力をリダイレクトする場合は --yes を指定してください。自動再試行やレビューは行いません。"},
		"unavailable":              {"The enrolled distribution or installed helper could not be verified. No new operation was requested.", "登録済みディストリビューションまたはインストール済みヘルパーを確認できません。新しい処理は要求していません。"},
		"status_failed":            {"The saved native result could not be read. No operation was started or acknowledged.", "保存されたWindows側の結果を読み取れません。処理の開始や確認済みへの変更は行っていません。"},
		"uncertain":                {"The request outcome is unconfirmed; a prepared or running operation may remain. Inspect haco-reclaim.exe --distribution <name> --status. Do not retry or clear records automatically.", "要求の結果は未確認です。準備済みまたは実行中の処理が残っている可能性があります。haco-reclaim.exe --distribution <name> --status で確認してください。自動再試行や記録の削除はしないでください。"},
		"dispatched":               {"Worker dispatched; reclamation is not yet confirmed. Inspect haco-reclaim.exe --distribution <name> --status without restarting WSL.", "処理を起動しました。容量回収の完了は未確認です。WSLを再起動せず、haco-reclaim.exe --distribution <name> --status で確認してください。"},
	}
	v := messages[key]
	if language == cliui.Japanese {
		return v[1]
	}
	return v[0]
}
