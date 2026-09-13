package main

import (
	"crypto/rand"
	"encoding/hex"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestNativeCOMActivationRoundTripWithoutRegistrationWrites(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		t.Fatal(err)
	}
	id := hex.EncodeToString(random[:])
	class, err := windows.GUIDFromString("{" + id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:] + "}")
	if err != nil {
		t.Fatal(err)
	}
	events := make(chan nativeActivation, 4)
	stop, err := startToastCOM(class, "Hacocoon.NativeTest", events)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err := wakeToastCOM(class, "Hacocoon.NativeTest"); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-events:
		if event.argument != "refresh" || len(event.inputs) != 0 {
			t.Fatalf("%+v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native COM activation was not delivered")
	}
	if err := wakeToastCOM(class, "wrong.application"); err == nil {
		t.Fatal("cross-application activation accepted")
	}
	opened := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		opened <- wakeToastCOM(class, "Hacocoon.NativeTest", strings.Repeat("a", 32))
	}()
	select {
	case event := <-events:
		if event.done == nil || event.argument != "open:"+strings.Repeat("a", 32) {
			t.Fatal("missing read-only display acknowledgement")
		}
		select {
		case <-opened:
			t.Fatal("launch acknowledged queueing before display")
		default:
		}
		event.done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("read-only launch was not delivered")
	}
	if err := <-opened; err != nil {
		t.Fatal(err)
	}
}

func TestNativeCallbackBodyInvalidInputsAndStoppedServer(t *testing.T) {
	comTables.Do(func() {
		factoryTable = [5]uintptr{syscall.NewCallback(factoryQuery), syscall.NewCallback(factoryAddRef), syscall.NewCallback(factoryRelease), syscall.NewCallback(factoryCreate), syscall.NewCallback(factoryLock)}
		activatorTable = [4]uintptr{syscall.NewCallback(activatorQuery), syscall.NewCallback(activatorAddRef), syscall.NewCallback(activatorRelease), syscall.NewCallback(activatorActivate)}
	})
	events := make(chan nativeActivation, 4)
	server := &toastCOM{appID: "Hacocoon.NativeTest", events: events}
	server.activator.table, server.activator.server = &activatorTable, server
	server.activator.refs.Store(1)
	this := uintptr(unsafe.Pointer(&server.activator))
	name, _ := windows.UTF16PtrFromString(server.appID)
	argument, _ := windows.UTF16PtrFromString(strings.Repeat("a", 64) + ":choose")
	key, _ := windows.UTF16PtrFromString("scope")
	value, _ := windows.UTF16PtrFromString("global")
	input := []notificationInput{{key, value}}
	invoke := func(args *uint16, data []notificationInput, count uintptr) uintptr {
		var pointer uintptr
		if len(data) > 0 {
			pointer = uintptr(unsafe.Pointer(&data[0]))
		}
		hr, _, _ := syscall.SyscallN(activatorTable[3], this, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(args)), pointer, count)
		runtime.KeepAlive(data)
		runtime.KeepAlive(args)
		return hr
	}
	if hr := invoke(nil, input, 1); hr != sOK || len(events) != 0 {
		t.Fatalf("body answered: %#x", hr)
	}
	if hr := invoke(argument, input, 1); hr != sOK {
		t.Fatalf("native input rejected: %#x", hr)
	}
	event := <-events
	if event.inputs["scope"] != "global" {
		t.Fatal(event.inputs)
	}
	if hr := invoke(argument, []notificationInput{input[0], input[0]}, 2); hr != eInvalidArg {
		t.Fatalf("duplicate accepted: %#x", hr)
	}
	if hr := invoke(argument, nil, 3); hr != eInvalidArg {
		t.Fatalf("oversized input accepted: %#x", hr)
	}
	server.stopped.Store(true)
	if hr := invoke(argument, input, 1); hr != eFail || len(events) != 0 {
		t.Fatalf("stopped callback: %#x", hr)
	}
	runtime.KeepAlive(name)
	runtime.KeepAlive(server)
}
