package core

// BaseAsset is independently retained immutable Base material. Provider and Scope
// qualify both revision reuse and native ownership. Binding is an opaque, bounded
// provider plan; it must never contain credentials or controller configuration.
// Assets are not writable Environment attachments and are not snapshot records.
type BaseAsset struct {
	ID        string  `json:"id"`
	Base      BaseRef `json:"base"`
	Provider  string  `json:"provider"`
	Scope     string  `json:"scope"`
	Owner     string  `json:"owner"`
	NativeRef string  `json:"native_ref"`
	Binding   string  `json:"binding"`
	State     string  `json:"state"`
}
