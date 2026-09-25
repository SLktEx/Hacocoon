package reviewcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/client/review"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func registeredToastServer(c configuration, own, appID, class string) error {
	for _, entry := range []struct{ path, name, want string }{
		{`Software\Classes\AppUserModelId\` + appID, "HacocoonDistribution", c.Distribution},
		{`Software\Classes\AppUserModelId\` + appID, "CustomActivator", class},
		{`Software\Classes\CLSID\` + class, "HacocoonDistribution", c.Distribution},
		{`Software\Classes\CLSID\` + class + `\LocalServer32`, "", `"` + own + `" --toast-server`},
	} {
		key, err := registry.OpenKey(registry.CURRENT_USER, entry.path, registry.QUERY_VALUE)
		if err != nil {
			return errors.New("missing native review registration")
		}
		value, _, err := key.GetStringValue(entry.name)
		key.Close()
		if err != nil || !strings.EqualFold(value, entry.want) {
			return errors.New("native review registration differs")
		}
	}
	return nil
}
func nativeReview(c configuration, own, id string) (resultErr error) {
	stage := "registration"
	defer func() {
		if resultErr != nil {
			resultErr = &nativeReviewFailure{stage: stage, cause: resultErr}
		}
	}()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	appID, err := desktopreview.Scheme(c.Distribution)
	if err != nil {
		return err
	}
	classText, err := desktopreview.ClassID(c.Distribution)
	if err != nil {
		return err
	}
	class, err := windows.GUIDFromString(classText)
	if err != nil {
		return err
	}
	if err = registeredToastServer(c, own, appID, classText); err != nil {
		return err
	}
	stage = "session_plan"
	plan, err := desktopreview.SessionPlan(c.Distribution, os.Getenv("SystemRoot"))
	if err != nil {
		return err
	}
	stage = "ownership"
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	session, err := openReviewSession(`Local\`+appID+"-"+tokenUser.User.Sid.String(), desktopreview.SessionWaitTimeout)
	if err != nil {
		return err
	}
	defer session.close()
	if !session.owned {
		stage = "activation"
		if id == "" {
			return errors.New("native review is already running")
		}
		if err := wakeToastCOM(class, appID, id); err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, "HACO_REVIEW_READY")
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	startup, finishStartup := context.WithTimeout(ctx, desktopreview.StartupTimeout)
	defer finishStartup()
	surface := &nativeToastSurface{plan: plan, appID: appID}
	// Expired in-memory nonces are never recovered after a crash or restart.
	stage = "clear"
	if err := surface.Clear(startup); err != nil {
		return err
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 8*time.Second)
		defer done()
		resultErr = errors.Join(resultErr, surface.Clear(cleanup))
	}()
	stage = "peer_start"
	peer, err := startReadyReviewPeer(ctx, startup, plan, own, c.Distribution)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, peer.Close()) }()
	stage = "activation"
	events := make(chan nativeActivation, 32)
	stop, err := startToastCOM(class, appID, events)
	if err != nil {
		return err
	}
	defer func() {
		// Withdraw before revoking COM; keep ownership through peer/history cleanup.
		// A duplicate then waits for that cleanup instead of launching another server.
		resultErr = errors.Join(resultErr, session.withdraw())
		stop()
	}()
	language, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	manager := &desktopreview.ToastManager{Exchange: peer, Surface: surface, Japanese: language&0x3ff == 0x11}
	if id != "" {
		stage = "review"
		opening, done := context.WithTimeout(startup, 10*time.Second)
		err := manager.Review(opening, id)
		done()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(os.Stdout, "HACO_REVIEW_READY"); err != nil {
			return err
		}
	}
	finishStartup()
	if err := session.publish(); err != nil {
		return err
	}
	stage = "events"
	type completion struct {
		job   *desktopreview.ToastSubmission
		reply desktopreview.Reply
		err   error
	}
	completed := make(chan completion, 16)
	var workers sync.WaitGroup
	defer func() { cancel(); workers.Wait() }()
	active := 0
	refresh := time.NewTicker(3 * time.Second)
	defer refresh.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-events:
			if event.done != nil {
				opening, done := context.WithTimeout(ctx, 8*time.Second)
				err := manager.Review(opening, strings.TrimPrefix(event.argument, "open:"))
				done()
				event.done <- err
				// A missing/expired request is a normal refusal, not a broken session.
				continue
			}
			if event.argument == "refresh" {
				continue
			}
			job, err := manager.Activate(ctx, event.argument, event.inputs)
			if errors.Is(err, desktopreview.ErrInvalid) {
				continue
			}
			if err != nil {
				return err
			}
			if job == nil {
				continue
			}
			if active >= 16 {
				return errors.New("native answer capacity reached")
			}
			active++
			workers.Add(1)
			go func(job *desktopreview.ToastSubmission) {
				defer workers.Done()
				decisionCtx, done := context.WithTimeout(ctx, 5*time.Minute)
				defer done()
				var reply desktopreview.Reply
				decisionPeer, err := startReadyReviewPeer(decisionCtx, decisionCtx, plan, own, c.Distribution)
				if err == nil {
					reply, err = job.Run(decisionCtx, decisionPeer)
					err = errors.Join(err, decisionPeer.Close())
				}
				completed <- completion{job, reply, err}
			}(job)
		case result := <-completed:
			active--
			if err := manager.Complete(ctx, result.job, result.reply, result.err); err != nil {
				return err
			}
		case <-refresh.C:
			reading, done := context.WithTimeout(ctx, 8*time.Second)
			err := manager.Refresh(reading)
			done()
			if err != nil {
				return err
			}
		}
	}
}
