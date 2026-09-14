package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/SLktEx/Hacocoon/internal/desktopreview"
)

// Constant script only on argv. Literal JSON/XML travels over stdin; raw native
// output never becomes a product diagnostic or an authorization receipt.
const nativeToastScript = `$ErrorActionPreference='Stop';$stage='runtime';try{
function Write-NativeStage([string]$value){[Console]::Error.WriteLine('HACO_TOAST_STAGE:'+$value)}
Write-NativeStage 'runtime'
[Console]::InputEncoding=[Text.UTF8Encoding]::new($false)
Write-NativeStage 'input'
$raw=[Console]::In.ReadToEnd()
Write-NativeStage 'decode'
$data=$raw|ConvertFrom-Json
Write-NativeStage 'winrt'
[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]>$null
[Windows.UI.Notifications.ToastNotification,Windows.UI.Notifications,ContentType=WindowsRuntime]>$null
[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom,ContentType=WindowsRuntime]>$null
if($data.operation -eq 'show'){
 $stage='xml'
 Write-NativeStage 'xml'
 $xml=New-Object Windows.Data.Xml.Dom.XmlDocument
 $xml.LoadXml($data.xml)
 $stage='create';Write-NativeStage 'create';$toast=New-Object Windows.UI.Notifications.ToastNotification $xml
 $stage='identity'
 Write-NativeStage 'identity'
 $toast.Tag=$data.tag;$toast.Group='HacocoonReview'
 $toast.ExpirationTime=[DateTimeOffset]::Now.AddMinutes(2)
 $notifier=[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($data.appID)
 $stage='show'
 Write-NativeStage 'show'
 if([int]$notifier.Setting -ne 0){[Console]::Out.Write('HACO_TOAST_DISABLED:'+[int]$notifier.Setting);exit 1}
 $notifier.Show($toast)
}elseif($data.operation -eq 'remove'){
 $stage='history'
 Write-NativeStage 'history'
 [Windows.UI.Notifications.ToastNotificationManager]::History.Remove($data.tag,'HacocoonReview',$data.appID)
}elseif($data.operation -eq 'clear'){
 $stage='history'
 Write-NativeStage 'history'
 [Windows.UI.Notifications.ToastNotificationManager]::History.RemoveGroup('HacocoonReview',$data.appID)
}else{exit 1}
Write-NativeStage 'complete'
[Console]::Out.Write('HACO_TOAST_OK')
}catch{[Console]::Out.Write('HACO_TOAST_FAILURE:'+$stage+':'+$_.Exception.HResult);exit 1}`

var nativeToastFailure = regexp.MustCompile(`^HACO_TOAST_FAILURE:(runtime|xml|create|identity|show|history):(-?[0-9]{1,11})$`)

type nativeToastSurface struct {
	plan  desktopreview.Invocation
	appID string
}
type limitedNativeReply struct{ data []byte }

// Drain all diagnostics, retaining only a bounded candidate for fixed stage
// parsing. Overflow or unrelated output never becomes a product log field.
type nativeProgress struct {
	data []byte
	invalid bool
}

func (p *nativeProgress) Write(data []byte) (int, error) {
	if len(data) > 512-len(p.data) {
		p.invalid = true
	} else if !p.invalid {
		p.data = append(p.data, data...)
	}
	return len(data), nil
}

func (p *nativeProgress) stage() string {
	if p.invalid || len(p.data) == 0 || p.data[len(p.data)-1] != '\n' {
		return "unobserved"
	}
	last := "unobserved"
	for _, line := range strings.Split(strings.TrimSuffix(string(p.data), "\n"), "\n") {
		value, ok := strings.CutPrefix(strings.TrimSuffix(line, "\r"), "HACO_TOAST_STAGE:")
		if !ok || nativeProgressStage(value) == "unobserved" {
			return "unobserved"
		}
		last = value
	}
	return last
}

func (b *limitedNativeReply) Write(p []byte) (int, error) {
	if len(p) > 64-len(b.data) {
		return 0, errors.New("oversized native reply")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func encodeNativeScript(s string) string {
	units := utf16.Encode([]rune(s))
	data := make([]byte, len(units)*2)
	for i, v := range units {
		binary.LittleEndian.PutUint16(data[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(data)
}
func (s *nativeToastSurface) invoke(ctx context.Context, operation, id, xml string) error {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	data, err := json.Marshal(struct {
		Operation string `json:"operation"`
		AppID     string `json:"appID"`
		Tag       string `json:"tag"`
		XML       string `json:"xml"`
	}{operation, s.appID, id, xml})
	if err != nil {
		return err
	}
	root := s.plan.Env[0][len("SystemRoot="):]
	cmd := exec.CommandContext(ctx, root+`\System32\WindowsPowerShell\v1.0\powershell.exe`, "-NoProfile", "-NonInteractive", "-EncodedCommand", encodeNativeScript(nativeToastScript))
	cmd.Env = s.plan.Env
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = time.Second
	return runNativeToastCommand(ctx, cmd)
}

// Context cancellation owns the outcome even if the child wrote a success
// marker before being killed. Only fixed result classes and numeric process
// observations reach the reporting boundary; raw exec errors/output do not.
func runNativeToastCommand(ctx context.Context, cmd *exec.Cmd) error {
	started := time.Now()
	var response limitedNativeReply
	var progress nativeProgress
	cmd.Stdout, cmd.Stderr = &response, &progress
	err := cmd.Run()
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	result := nativeToastResult(response.data, err)
	if result == nil {
		return nil
	}
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return &nativeToastProcessFailure{cause: result, exitCode: exitCode, durationMS: time.Since(started).Milliseconds(), progress: progress.stage()}
}

func nativeToastResult(output []byte, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if err != nil || string(output) != "HACO_TOAST_OK" {
		if value := string(output); value == "HACO_TOAST_DISABLED:1" || value == "HACO_TOAST_DISABLED:2" || value == "HACO_TOAST_DISABLED:3" || value == "HACO_TOAST_DISABLED:4" {
			return fmt.Errorf("native notifications are disabled (setting %c)", value[len(value)-1])
		}
		if match := nativeToastFailure.FindSubmatch(output); match != nil {
			if status, parseErr := strconv.ParseInt(string(match[2]), 10, 64); parseErr == nil {
				return &nativeDisplayFailure{stage: string(match[1]), status: status}
			}
		}
		return errors.New("native notification display unavailable")
	}
	return nil
}
func toastTag(id string) string { sum := sha256.Sum256([]byte(id)); return fmt.Sprintf("%x", sum[:8]) }
func (s *nativeToastSurface) Show(ctx context.Context, id string, page desktopreview.ToastPage) error {
	if len(id) != 32 {
		return desktopreview.ErrInvalid
	}
	xml, err := desktopreview.ToastXML(page)
	if err != nil {
		return err
	}
	return s.invoke(ctx, "show", toastTag(id), xml)
}
func (s *nativeToastSurface) Remove(ctx context.Context, id string) error {
	if len(id) != 32 {
		return desktopreview.ErrInvalid
	}
	return s.invoke(ctx, "remove", toastTag(id), "")
}
func (s *nativeToastSurface) Clear(ctx context.Context) error { return s.invoke(ctx, "clear", "", "") }
