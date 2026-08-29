package core

import "context"

type Runtime interface {
	ID() string
	Probe(context.Context) (RuntimeCapabilities, error)
	Create(context.Context, RuntimeSessionSpec) (RuntimeSession, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Delete(context.Context, string) error
	Exec(context.Context, string, ExecRequest) (ExecResult, error)
	Inspect(context.Context, string) (RuntimeState, error)
}

// RuntimePreparer is an optional host-side initialization capability. Concrete
// runtimes keep provider details behind this seam while Core can make `haco init`
// prepare the selected runtime composition idempotently.
type RuntimePreparer interface {
	Prepare(context.Context, RuntimePrepareSpec) error
}

type Storage interface {
	ID() string
	Probe(context.Context) (StorageCapabilities, error)
	Ensure(context.Context, StorageSpec) (StorageHandle, error)
	Inspect(context.Context, StorageHandle) (StorageState, error)
	Delete(context.Context, StorageHandle) error
}

type ResizableStorage interface {
	Grow(context.Context, StorageHandle, int64) error
	PlanShrink(context.Context, StorageHandle, int64) (ShrinkPlan, error)
	Shrink(context.Context, StorageHandle, ShrinkPlan) error
	Compact(context.Context, StorageHandle) error
}

type SessionStore interface {
	List(context.Context) ([]Session, error)
	Get(context.Context, SessionID) (Session, error)
	Put(context.Context, Session) error
	Delete(context.Context, SessionID) error
}
