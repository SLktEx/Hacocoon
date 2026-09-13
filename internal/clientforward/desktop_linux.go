package clientforward

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

// DesktopCommand keeps native Linux local and delegates an ordinary WSL/trusted
// Host entry to its installed Windows companion. WSL hints select UX only;
// discovery and the native controller independently validate installation identity.
func DesktopCommand(ctx context.Context, args []string, out, diagnostic io.Writer, language cliui.Language, connect func() (*controlapi.Client, error), usage func()) int {
	run := runPrepared
	if os.Getenv("WSL_INTEROP") != "" || os.Getenv("WSL_DISTRO_NAME") != "" {
		run = runWindows
	}
	return command(ctx, args, out, diagnostic, language, connect, usage, run)
}

func runWindows(ctx context.Context, client *controlapi.Client, request prepared, out, diagnostic io.Writer) int {
	code, err := delegateWindows(ctx, client, request, out, diagnostic)
	if err != nil {
		fmt.Fprintln(diagnostic, request.Language.Format("forward.windows_unavailable"))
		return 1
	}
	return code
}

func delegateWindows(ctx context.Context, client *controlapi.Client, request prepared, out, diagnostic io.Writer) (int, error) {
	discovery, stop := context.WithTimeout(ctx, 20*time.Second)
	defer stop()
	target, err := client.ReclamationTarget(discovery)
	if err != nil {
		return 1, err
	}
	helper, err := resolveWindowsHelper(discovery, target)
	if err != nil {
		return 1, err
	}
	stop()
	deadline, ok := ctx.Deadline()
	if !ok {
		return 1, errors.New("tunnel deadline required")
	}
	cmd := exec.CommandContext(ctx, helper, "_delegate")
	cmd.Env = interopEnvironment()
	cmd.Dir = filepath.Dir(helper)
	return runCompanion(ctx, cmd, delegation{Version: 1, Installation: target, Request: request, Expires: deadline.UnixMilli()}, out, diagnostic)
}

func interopEnvironment() []string {
	return []string{"WSL_INTEROP=" + os.Getenv("WSL_INTEROP"), "WSLENV="}
}

// Discovery executes fixed read-only PowerShell, with no caller command text.
// The installer owns this same per-user/per-registration path. No disk worker,
// registry mutation, management permission or reusable credential is involved.
func resolveWindowsHelper(ctx context.Context, target reclamation.WSLTarget) (string, error) {
	if target.Validate() != nil {
		return "", errors.New("invalid installation")
	}
	executable, err := exec.LookPath("powershell.exe")
	if err != nil {
		return "", err
	}
	script := `[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);[Environment]::GetFolderPath('LocalApplicationData')|ConvertTo-Json -Compress`
	cmd := exec.CommandContext(ctx, executable, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = interopEnvironment()
	cmd.Dir = filepath.Dir(executable)
	cmd.WaitDelay = 5 * time.Second
	var output boundedDiscovery
	cmd.Stdout, cmd.Stderr = &output, io.Discard
	if err := cmd.Run(); err != nil {
		return "", err
	}
	var native string
	if json.Unmarshal(output.Bytes(), &native) != nil {
		return "", errors.New("invalid Windows data directory")
	}
	root, err := projectedWindowsPath(native)
	if err != nil {
		return "", err
	}
	return installedWindowsHelper(root, target)
}

func installedWindowsHelper(root string, target reclamation.WSLTarget) (string, error) {
	if target.Validate() != nil {
		return "", errors.New("invalid Windows installation")
	}
	directory := filepath.Join(root, "Hacocoon", "client", strings.ReplaceAll(target.RegistrationID[1:37], "-", ""))
	// Reject static redirections and foreign installation markers. Windows user
	// ownership remains the execution trust boundary, as for the installed client.
	for cursor := directory; ; cursor = filepath.Dir(cursor) {
		info, err := os.Lstat(cursor)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("Windows client directory unavailable")
		}
		if cursor == root {
			break
		}
	}
	record, err := os.OpenFile(filepath.Join(directory, "installation.json"), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", err
	}
	defer record.Close()
	info, err := record.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return "", errors.New("invalid Windows client ownership")
	}
	var owner struct {
		RegistrationID string `json:"registration_id"`
		Version        int    `json:"schema_version"`
	}
	decoder := json.NewDecoder(io.LimitReader(record, 4097))
	decoder.DisallowUnknownFields()
	var extra any
	if decoder.Decode(&owner) != nil || decoder.Decode(&extra) != io.EOF || owner.Version != 1 || owner.RegistrationID != target.RegistrationID {
		return "", errors.New("Windows client ownership differs")
	}
	helper := filepath.Join(directory, "haco-tunnel.exe")
	info, err = os.Lstat(helper)
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("Windows tunnel client unavailable")
	}
	return helper, nil
}

type boundedDiscovery struct{ bytes.Buffer }

func (b *boundedDiscovery) Write(p []byte) (int, error) {
	if len(p) > 8192-b.Len() {
		return 0, errors.New("Windows discovery result too large")
	}
	return b.Buffer.Write(p)
}

func projectedWindowsPath(native string) (string, error) {
	if len(native) < 4 || native[1:3] != ":\\" || !((native[0] >= 'A' && native[0] <= 'Z') || (native[0] >= 'a' && native[0] <= 'z')) || strings.ContainsAny(native[3:], ":/\r\n\x00\"<>|?*") {
		return "", errors.New("invalid Windows data directory")
	}
	for _, part := range strings.Split(native[3:], "\\") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return "", errors.New("invalid Windows directory component")
		}
	}
	return "/mnt/" + strings.ToLower(native[:1]) + "/" + strings.ReplaceAll(native[3:], "\\", "/"), nil
}
