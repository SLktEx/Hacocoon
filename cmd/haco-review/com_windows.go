package main

import (
	"errors"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/SLktEx/Hacocoon/internal/desktopreview"
	"golang.org/x/sys/windows"
)

// The ABI follows Microsoft's NotificationActivationCallback.h. COM provides
// native activation and input marshalling; URI/argv entry points cannot submit
// these answers. The presentation layer additionally checks a private page nonce.
type nativeActivation struct {
	argument string
	inputs   map[string]string
	done     chan error
}

// Private read-only acknowledgement: an absent request is different from a
// failed display or unavailable controller. Neither status authorizes an answer.
const eNoLongerPending uintptr = 0x80040201

type notificationInput struct{ key, value *uint16 }
type comFactory struct {
	table  *[5]uintptr
	refs   atomic.Int32
	locks  int
	lockMu sync.Mutex
	server *toastCOM
}
type comActivator struct {
	table  *[4]uintptr
	refs   atomic.Int32
	server *toastCOM
}
type toastCOM struct {
	factory   comFactory
	activator comActivator
	appID     string
	events    chan<- nativeActivation
	stopped   atomic.Bool
}

var (
	ole32                 = windows.NewLazySystemDLL("ole32.dll")
	coInitializeEx        = ole32.NewProc("CoInitializeEx")
	coUninitialize        = ole32.NewProc("CoUninitialize")
	coRegisterClassObject = ole32.NewProc("CoRegisterClassObject")
	coRevokeClassObject   = ole32.NewProc("CoRevokeClassObject")
	coCreateInstance      = ole32.NewProc("CoCreateInstance")
	iidUnknown            = windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidFactory            = windows.GUID{Data1: 1, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidNotification       = windows.GUID{Data1: 0x53e31837, Data2: 0x6600, Data3: 0x4a81, Data4: [8]byte{0x93, 0x95, 0x75, 0xcf, 0xfe, 0x74, 0x6f, 0x94}}
	factoryTable          [5]uintptr
	activatorTable        [4]uintptr
	comTables             sync.Once
	// Native references are not Go GC roots. Retain each object until COM has
	// released it, including callbacks finishing after class revocation.
	comRoots sync.Map
)

const (
	sOK                uintptr = 0
	ePointer           uintptr = 0x80004003
	eNoInterface       uintptr = 0x80004002
	eInvalidArg        uintptr = 0x80070057
	eFail              uintptr = 0x80004005
	classNoAggregation uintptr = 0x80040110
)

func factoryQuery(this *comFactory, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if iid == nil || out == nil {
		return ePointer
	}
	*out = nil
	guid := *iid
	if guid != iidUnknown && guid != iidFactory {
		return eNoInterface
	}
	*out = unsafe.Pointer(this)
	factoryAddRef(this)
	return sOK
}
func factoryAddRef(this *comFactory) uintptr {
	return uintptr(this.refs.Add(1))
}
func factoryRelease(this *comFactory) uintptr {
	n := this.refs.Add(-1)
	if n == 0 {
		comRoots.Delete(unsafe.Pointer(this))
	}
	return uintptr(n)
}
func factoryCreate(this *comFactory, outer unsafe.Pointer, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if out == nil {
		return ePointer
	}
	*out = nil
	if outer != nil {
		return classNoAggregation
	}
	f := this
	if f.server.stopped.Load() {
		return eFail
	}
	return activatorQuery(&f.server.activator, iid, out)
}
func factoryLock(this *comFactory, lock uintptr) uintptr {
	f := this
	f.lockMu.Lock()
	defer f.lockMu.Unlock()
	if (lock != 0 && f.locks >= 256) || (lock == 0 && f.locks <= 0) {
		return eFail
	}
	if lock != 0 {
		f.locks++
		factoryAddRef(this)
	} else {
		f.locks--
		factoryRelease(this)
	}
	return sOK
}
func activatorQuery(this *comActivator, iid *windows.GUID, out *unsafe.Pointer) uintptr {
	if iid == nil || out == nil {
		return ePointer
	}
	*out = nil
	guid := *iid
	if guid != iidUnknown && guid != iidNotification {
		return eNoInterface
	}
	*out = unsafe.Pointer(this)
	activatorAddRef(this)
	return sOK
}
func activatorAddRef(this *comActivator) uintptr {
	return uintptr(this.refs.Add(1))
}
func activatorRelease(this *comActivator) uintptr {
	n := this.refs.Add(-1)
	if n == 0 {
		comRoots.Delete(unsafe.Pointer(this))
	}
	return uintptr(n)
}

func boundedWide(pointer *uint16, limit int) (string, bool) {
	if pointer == nil {
		return "", true
	}
	value := make([]uint16, 0, min(limit, 64))
	for i := 0; i < limit; i++ {
		r := *(*uint16)(unsafe.Add(unsafe.Pointer(pointer), uintptr(i)*2))
		if r == 0 {
			return windows.UTF16ToString(value), true
		}
		value = append(value, r)
	}
	return "", false
}

func activatorActivate(this *comActivator, appID, args *uint16, data *notificationInput, count uint32) uintptr {
	a := this
	server := a.server
	if server.stopped.Load() {
		return eFail
	}
	name, ok := boundedWide(appID, 256)
	if !ok || name != server.appID {
		return eInvalidArg
	}
	argument, ok := boundedWide(args, 128)
	if !ok {
		return eInvalidArg
	}
	// The toast body, display and dismissal never answer, even when controls
	// happened to contain a selected value at the time of the body click.
	if argument == "" {
		return sOK
	}
	if count > 2 || (count > 0 && data == nil) {
		return eInvalidArg
	}
	inputs := map[string]string{}
	for _, entry := range unsafe.Slice(data, int(count)) {
		key, kOK := boundedWide(entry.key, 16)
		value, vOK := boundedWide(entry.value, 64)
		if !kOK || !vOK || (key != "scope" && key != "policy") {
			return eInvalidArg
		}
		if _, duplicate := inputs[key]; duplicate {
			return eInvalidArg
		}
		inputs[key] = value
	}
	event := nativeActivation{argument: argument, inputs: inputs}
	if strings.HasPrefix(argument, "open:") {
		if count != 0 || len(argument) != 37 {
			return eInvalidArg
		}
		for _, r := range argument[5:] {
			if !(r >= 'a' && r <= 'f' || r >= '0' && r <= '9') {
				return eInvalidArg
			}
		}
		event.done = make(chan error, 1)
	}
	select {
	case server.events <- event:
		if event.done == nil {
			return sOK
		}
		// Read-only duplicate launches acknowledge actual Show, not queueing.
		select {
		case err := <-event.done:
			if err == nil {
				return sOK
			}
			if errors.Is(err, desktopreview.ErrNoLongerPending) {
				return eNoLongerPending
			}
			return eFail
		case <-time.After(10 * time.Second):
			return eFail
		}
	default:
		return eFail
	}
}

// startToastCOM must be called and stopped on the same locked OS thread.
func startToastCOM(class windows.GUID, appID string, events chan<- nativeActivation) (func(), error) {
	hr, _, _ := coInitializeEx.Call(0, 0) // COINIT_MULTITHREADED
	if int32(hr) < 0 {
		return nil, errors.New("initialize native notification activation")
	}
	comTables.Do(func() {
		factoryTable = [5]uintptr{syscall.NewCallback(factoryQuery), syscall.NewCallback(factoryAddRef), syscall.NewCallback(factoryRelease), syscall.NewCallback(factoryCreate), syscall.NewCallback(factoryLock)}
		activatorTable = [4]uintptr{syscall.NewCallback(activatorQuery), syscall.NewCallback(activatorAddRef), syscall.NewCallback(activatorRelease), syscall.NewCallback(activatorActivate)}
	})
	s := &toastCOM{appID: appID, events: events}
	s.factory.table, s.factory.server = &factoryTable, s
	s.activator.table, s.activator.server = &activatorTable, s
	s.factory.refs.Store(1)
	s.activator.refs.Store(1)
	factoryPointer := unsafe.Pointer(&s.factory)
	activatorPointer := unsafe.Pointer(&s.activator)
	comRoots.Store(factoryPointer, s)
	comRoots.Store(activatorPointer, s)
	var cookie uint32
	hr, _, _ = coRegisterClassObject.Call(uintptr(unsafe.Pointer(&class)), uintptr(factoryPointer), 4, 1, uintptr(unsafe.Pointer(&cookie))) // LOCAL_SERVER, MULTIPLEUSE
	if int32(hr) < 0 {
		factoryRelease(&s.factory)
		activatorRelease(&s.activator)
		coUninitialize.Call()
		return nil, errors.New("register native notification activation")
	}
	return func() {
		s.stopped.Store(true)
		coRevokeClassObject.Call(uintptr(cookie))
		factoryRelease(&s.factory)
		activatorRelease(&s.activator)
		coUninitialize.Call()
		runtime.KeepAlive(s)
	}, nil
}

// wakeToastCOM only asks the already running native helper to refresh its read
// view. An optional request ID is correlation only; no page token or answer is
// forwarded from argv.
func wakeToastCOM(class windows.GUID, appID string, request ...string) error {
	hr, _, _ := coInitializeEx.Call(0, 0)
	if int32(hr) < 0 {
		return errors.New("initialize notification refresh")
	}
	defer coUninitialize.Call()
	var object unsafe.Pointer
	hr, _, _ = coCreateInstance.Call(uintptr(unsafe.Pointer(&class)), 0, 4, uintptr(unsafe.Pointer(&iidNotification)), uintptr(unsafe.Pointer(&object)))
	if int32(hr) < 0 || object == nil {
		return errors.New("notification review is unavailable")
	}
	table := *(*[4]uintptr)(*(*unsafe.Pointer)(object))
	defer syscall.SyscallN(table[2], uintptr(object))
	name, _ := windows.UTF16PtrFromString(appID)
	argument := "refresh"
	if len(request) == 1 {
		argument = "open:" + request[0]
	}
	args, _ := windows.UTF16PtrFromString(argument)
	hr, _, _ = syscall.SyscallN(table[3], uintptr(object), uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(args)), 0, 0)
	runtime.KeepAlive(name)
	runtime.KeepAlive(args)
	if uint32(hr) == uint32(eNoLongerPending) {
		return desktopreview.ErrNoLongerPending
	}
	if int32(hr) < 0 {
		return errors.New("notification refresh was refused")
	}
	return nil
}
