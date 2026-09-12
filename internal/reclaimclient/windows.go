// Package reclaimclient invokes the installed Windows helper through the trusted
// Host's existing Windows interop. Native enrollment and disk authority remain
// in wslreclaim; this package has no Incus or filesystem mutation authority.
package reclaimclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

// Only fixed actions and validated identity reach the existing Windows interop.
// The native helper independently requires saved enrollment, pins and exclusion.
func windowsReclaimScript(target reclamation.WSLTarget, mode string) (string, error) {
	if target.Validate() != nil || (mode != "start" && mode != "status") {
		return "", errors.New("invalid reclamation request")
	}
	prefix := windowsReclaimPrefix(target)
	if mode == "status" {
		return prefix + `& $helper _status $reg.ToString('B');exit $LASTEXITCODE`, nil
	}
	return prefix + `$raw=& $helper _prepare $reg.ToString('B');if($LASTEXITCODE -ne 0){exit 1};$prepared=($raw -join "` + "`n" + `")|ConvertFrom-Json;$op=[guid]::Empty;if($prepared.state -cne 'pending' -or ![guid]::TryParse($prepared.operation,[ref]$op) -or $op -eq [guid]::Empty){exit 1};$dispatch=& $helper _launch $reg.ToString('B') $op.ToString('B');if($LASTEXITCODE -ne 0){exit 1};$text=$dispatch -join "` + "`n" + `";if($text -cnotmatch '^Dispatched Windows worker ([1-9][0-9]*); inspect the prepared operation for completion\.\s*$'){exit 1};[ordered]@{operation=$op.ToString('B');worker_pid=[int]$Matches[1]}|ConvertTo-Json -Compress`, nil
}

func windowsReclaimPrefix(target reclamation.WSLTarget) string {
	return `$ErrorActionPreference='Stop';[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$reg=[guid]'` + target.RegistrationID + `';$root=[Environment]::GetFolderPath('LocalApplicationData');if([string]::IsNullOrWhiteSpace($root)){exit 1};$helper=Join-Path $root ('Hacocoon\reclamation\'+$reg.ToString('N')+'\haco-wsl.exe');if(!(Test-Path -LiteralPath $helper -PathType Leaf)){exit 1};`
}

func windowsReviewScript(target reclamation.WSLTarget, operation, state string) (string, error) {
	operation = strings.ToLower(operation)
	if target.Validate() != nil || (reclamation.WSLTarget{RegistrationID: operation, InstallationID: target.InstallationID}).Validate() != nil {
		return "", errors.New("invalid reclamation review identity")
	}
	var action string
	switch state {
	case "failed":
		action = "_review-failed"
	case "pending", "interrupted":
		action = "_review-interrupted"
	default:
		return "", errors.New("invalid reclamation review state")
	}
	return windowsReclaimPrefix(target) + `$null=& $helper ` + action + ` $reg.ToString('B') '` + operation + `';exit $LASTEXITCODE`, nil
}

// ReviewWindows acknowledges only the selected observed operation. It never
// prepares/launches another run; native enrollment, pins and exclusion still apply.
func ReviewWindows(ctx context.Context, target reclamation.WSLTarget, operation, state string) error {
	script, err := windowsReviewScript(target, operation, state)
	if err != nil {
		return err
	}
	_, err = invokeWindowsScript(ctx, script)
	return err
}

type reclaimOutput struct{ bytes.Buffer }

func (b *reclaimOutput) Write(p []byte) (int, error) {
	if len(p) > 16384-b.Len() {
		return 0, errors.New("Windows result too large")
	}
	return b.Buffer.Write(p)
}
func InvokeWindows(ctx context.Context, target reclamation.WSLTarget, mode string) ([]byte, error) {
	script, err := windowsReclaimScript(target, mode)
	if err != nil {
		return nil, err
	}
	return invokeWindowsScript(ctx, script)
}

func invokeWindowsScript(ctx context.Context, script string) ([]byte, error) {
	executable, err := exec.LookPath("powershell.exe")
	if err != nil {
		return nil, errors.New("Windows interop unavailable")
	}
	encoded := utf16.Encode([]rune(script))
	raw := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		binary.LittleEndian.PutUint16(raw[2*i:], v)
	}
	command := exec.CommandContext(ctx, executable, "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", base64.StdEncoding.EncodeToString(raw))
	var output reclaimOutput
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second
	if err := command.Run(); err != nil {
		return nil, errors.New("Windows helper invocation unconfirmed")
	}
	return output.Bytes(), nil
}
