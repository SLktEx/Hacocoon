package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/desktopreview"
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
	plan, err := desktopreview.SessionPlan(c.Distribution, os.Getenv("SystemRoot"))
	if err != nil {
		return err
	}
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	mutexName, err := windows.UTF16PtrFromString(`Local\` + appID + "-" + tokenUser.User.Sid.String())
	if err != nil {
		return err
	}
	mutex, err := windows.CreateMutex(nil, false, mutexName)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return err
	}
	defer windows.CloseHandle(mutex)
	state, err := windows.WaitForSingleObject(mutex, 0)
	if err != nil {
		return err
	}
	if state == uint32(windows.WAIT_TIMEOUT) {
		if id == "" {
			return errors.New("native review is already running")
		}
		if err := wakeToastCOM(class, appID, id); err != nil {
			return err
		}
		_, err = fmt.Fprintln(os.Stdout, "HACO_REVIEW_READY")
		return err
	}
	if state != windows.WAIT_OBJECT_0 && state != windows.WAIT_ABANDONED {
		return errors.New("cannot own native review session")
	}
	defer windows.ReleaseMutex(mutex)
	events := make(chan nativeActivation, 32)
	stop, err := startToastCOM(class, appID, events)
	if err != nil {
		return err
	}
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	surface := &nativeToastSurface{plan: plan, appID: appID}
	// Expired in-memory nonces are never recovered after a crash or restart.
	if err := surface.Clear(ctx); err != nil {
		return err
	}
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 8*time.Second)
		defer done()
		resultErr = errors.Join(resultErr, surface.Clear(cleanup))
	}()
	peer, err := startReviewPeer(ctx, plan, own)
	if err != nil {
		return err
	}
	defer peer.Close()
	language, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	manager := &desktopreview.ToastManager{Exchange: peer, Surface: surface, Japanese: language&0x3ff == 0x11}
	if id != "" {
		opening, done := context.WithTimeout(ctx, 10*time.Second)
		err := manager.Review(opening, id)
		done()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(os.Stdout, "HACO_REVIEW_READY"); err != nil {
			return err
		}
	}
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
				decisionPeer, err := startReviewPeer(decisionCtx, plan, own)
				if err == nil {
					reply, err = job.Run(decisionCtx, decisionPeer)
					decisionPeer.Close()
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
