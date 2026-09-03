package block

import "context"

type Capabilities struct {
	Available bool
	Shrink    bool
	Compact   bool
	Details   []string
}

type Spec struct {
	ID        string
	Path      string
	SizeBytes int64
}

type Handle struct {
	ID   string
	Path string
	// Device is the current runtime attachment, not durable identity. Callers
	// must tolerate it becoming stale after detach, reattach, or reboot.
	Device string
	Bytes  int64
}

type State struct {
	Healthy bool
	Bytes   int64
	Device  string
	Details []string
}

type Store interface {
	ID() string
	Probe(context.Context) (Capabilities, error)
	Ensure(context.Context, Spec) (Handle, error)
	Inspect(context.Context, Handle) (State, error)
	Attach(context.Context, Handle) (Handle, error)
	Detach(context.Context, Handle) error
	Grow(context.Context, Handle, int64) (Handle, error)
	Shrink(context.Context, Handle, int64) (Handle, error)
	Compact(context.Context, Handle) error
	Delete(context.Context, Handle) error
}
