package reclamation

// WindowsResumeFailure contains only a fixed category and a numeric process or
// native error code. It never contains subprocess output or changes retry authority.
type WindowsResumeFailure struct {
	Kind string `json:",omitempty"`
	Code uint32 `json:",omitempty"`
}

func (f WindowsResumeFailure) Valid() bool {
	switch f.Kind {
	case "", "timeout", "canceled", "other":
		return f.Code == 0
	case "exit", "native":
		return f.Code != 0
	default:
		return false
	}
}
