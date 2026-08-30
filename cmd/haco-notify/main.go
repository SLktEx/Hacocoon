package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
	"github.com/SLktEx/Hacocoon/pkg/interactionhttp"
)

const maxSeenEventIDs = 512

type notifier interface {
	Notify(context.Context, string, string) error
}

type batchReader interface {
	Batch(context.Context, int64, int) (interaction.Batch, error)
}

type commandNotifier struct {
	command func(context.Context, string, string) *exec.Cmd
}

func (n commandNotifier) Notify(ctx context.Context, title, body string) error {
	cmd := n.command(ctx, title, body)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("native notification command failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

type notifyState struct {
	Offset       int64    `json:"offset"`
	SeenEventIDs []string `json:"seen_event_ids"`
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := dispatch(ctx, os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "haco-notify:", err)
		os.Exit(1)
	}
}

func dispatch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: haco-notify <web|native> [options]")
	}
	switch args[0] {
	case "web":
		return webCommand(ctx, args[1:])
	case "native":
		return nativeCommand(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func webCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("web", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	listen := fs.String("listen", "127.0.0.1:18081", "loopback HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || !interactionhttp.IsLoopbackAddress(*listen) {
		return errors.New("web notifications require an explicit loopback listen address")
	}

	reader, err := interaction.NewDefaultReader()
	if err != nil {
		return err
	}
	handler, err := interactionhttp.NewHandler(reader)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              *listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Hacocoon browser notifications: http://%s/\n", *listen)
	err = server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func nativeCommand(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("native", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	statePath := fs.String("state", defaultStatePath(), "cursor/dedup state file")
	poll := fs.Duration("poll", 2*time.Second, "poll interval")
	once := fs.Bool("once", false, "process one available batch and exit")
	includeCompleted := fs.Bool("include-completed", false, "show low-priority completed events")
	backend := fs.String("backend", "auto", "notification backend: auto, windows, linux")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *poll < 250*time.Millisecond {
		return interaction.ErrInvalidArgument
	}

	reader, err := interaction.NewDefaultReader()
	if err != nil {
		return err
	}
	presenter, err := chooseNotifier(*backend)
	if err != nil {
		return err
	}
	return runNative(ctx, reader, presenter, *statePath, *poll, *once, *includeCompleted)
}

func runNative(ctx context.Context, reader batchReader, presenter notifier, statePath string, poll time.Duration, once, includeCompleted bool) error {
	if reader == nil || presenter == nil || statePath == "" || poll < 250*time.Millisecond {
		return interaction.ErrInvalidArgument
	}
	state, err := loadState(statePath)
	if err != nil {
		return err
	}

	for {
		batch, batchErr := reader.Batch(ctx, state.Offset, interaction.DefaultBatchSize)
		for _, event := range batch.Events {
			if !state.hasSeen(event.EventID) {
				title, body, show := notificationText(event, includeCompleted)
				if show {
					if err := presenter.Notify(ctx, title, body); err != nil {
						return err
					}
					state.remember(event.EventID)
				}
			}
			state.Offset = event.NextOffset
			if err := saveState(statePath, state); err != nil {
				return err
			}
		}
		if batch.NextOffset > state.Offset {
			state.Offset = batch.NextOffset
			if err := saveState(statePath, state); err != nil {
				return err
			}
		}
		if batchErr != nil {
			return batchErr
		}
		if once {
			return nil
		}
		timer := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func chooseNotifier(backend string) (notifier, error) {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "auto":
		if strings.TrimSpace(os.Getenv("WSL_DISTRO_NAME")) != "" {
			if _, err := exec.LookPath("powershell.exe"); err == nil {
				return windowsNotifier(), nil
			}
		}
		if runtime.GOOS == "linux" {
			if _, err := exec.LookPath("notify-send"); err == nil {
				return linuxNotifier(), nil
			}
		}
		return nil, errors.New("no supported native notification backend found")
	case "windows":
		if _, err := exec.LookPath("powershell.exe"); err != nil {
			return nil, fmt.Errorf("powershell.exe not found: %w", err)
		}
		return windowsNotifier(), nil
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return nil, fmt.Errorf("notify-send not found: %w", err)
		}
		return linuxNotifier(), nil
	default:
		return nil, interaction.ErrInvalidArgument
	}
}

func windowsNotifier() notifier {
	return commandNotifier{command: func(ctx context.Context, title, body string) *exec.Cmd {
		script := windowsToastScript(title, body)
		return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodePowerShell(script))
	}}
}

func linuxNotifier() notifier {
	return commandNotifier{command: func(ctx context.Context, title, body string) *exec.Cmd {
		return exec.CommandContext(ctx, "notify-send", "--app-name=Hacocoon", title, body)
	}}
}

func windowsToastScript(title, body string) string {
	title64 := base64.StdEncoding.EncodeToString([]byte(title))
	body64 := base64.StdEncoding.EncodeToString([]byte(body))
	return "$title=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + title64 + "'));" +
		"$body=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + body64 + "'));" +
		"[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] > $null;" +
		"[Windows.UI.Notifications.ToastNotification,Windows.UI.Notifications,ContentType=WindowsRuntime] > $null;" +
		"$template=[Windows.UI.Notifications.ToastTemplateType]::ToastText02;" +
		"$xml=[Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent($template);" +
		"$nodes=$xml.GetElementsByTagName('text');" +
		"$null=$nodes.Item(0).AppendChild($xml.CreateTextNode($title));" +
		"$null=$nodes.Item(1).AppendChild($xml.CreateTextNode($body));" +
		"$toast=New-Object Windows.UI.Notifications.ToastNotification $xml;" +
		"[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Hacocoon').Show($toast);"
}

func encodePowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(raw[i*2:], unit)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func notificationText(event interaction.Event, includeCompleted bool) (string, string, bool) {
	details := strings.Join(nonEmpty(event.Environment, event.Capability, event.Action), " · ")
	if details == "" {
		details = event.Code
	}
	switch event.Kind {
	case interaction.ApprovalRequired:
		return "Hacocoon approval required", details, true
	case interaction.RecoveryRequired:
		return "Hacocoon needs recovery", details, true
	case interaction.OperationFailed:
		return "Hacocoon operation failed", details, true
	case interaction.PolicyDenied:
		return "Hacocoon policy denied", details, true
	case interaction.ApprovalDenied:
		return "Hacocoon approval denied", details, true
	case interaction.OperationCompleted:
		return "Hacocoon operation completed", details, includeCompleted
	default:
		return "", "", false
	}
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func defaultStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".haco-notify-state.json"
	}
	return filepath.Join(home, ".local", "state", "hacocoon", "native-notify.json")
}

func loadState(path string) (notifyState, error) {
	var state notifyState
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(payload, &state); err != nil || state.Offset < 0 {
		return notifyState{}, fmt.Errorf("invalid notification state: %w", interaction.ErrInvalidArgument)
	}
	if len(state.SeenEventIDs) > maxSeenEventIDs {
		state.SeenEventIDs = append([]string(nil), state.SeenEventIDs[len(state.SeenEventIDs)-maxSeenEventIDs:]...)
	}
	return state, nil
}

func saveState(path string, state notifyState) error {
	if path == "" || state.Offset < 0 {
		return interaction.ErrInvalidArgument
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, payload, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (s notifyState) hasSeen(eventID string) bool {
	for _, candidate := range s.SeenEventIDs {
		if candidate == eventID {
			return true
		}
	}
	return false
}

func (s *notifyState) remember(eventID string) {
	if eventID == "" || s.hasSeen(eventID) {
		return
	}
	s.SeenEventIDs = append(s.SeenEventIDs, eventID)
	if len(s.SeenEventIDs) > maxSeenEventIDs {
		s.SeenEventIDs = append([]string(nil), s.SeenEventIDs[len(s.SeenEventIDs)-maxSeenEventIDs:]...)
	}
}
