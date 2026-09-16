package reclamation

// InvocationFailure contains only stable diagnostic categories. It is never a
// receipt of absence, authorization to retry, or provider/subprocess output.
type InvocationFailure struct {
	Phase       string `json:"phase"`
	Stage       string `json:"stage"`
	NativeError uint32 `json:"native_error,omitempty"`
}

func (f InvocationFailure) Valid() bool {
	if f.Phase == "prepare" {
		switch f.Stage {
		case "registration", "exclusion", "disk_path", "disk_access", "record_access", "windows_owner", "enrollment", "installation", "binding", "intent", "other":
			return true
		}
	}
	if f.Phase == "launch" {
		switch f.Stage {
		case "pin_executable", "process_start", "readiness", "other":
			return true
		}
	}
	return false
}

type InvocationFailureReceipt struct {
	Failure InvocationFailure `json:"failure"`
}
