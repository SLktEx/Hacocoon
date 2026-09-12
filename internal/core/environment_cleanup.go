package core

// EnvironmentDeletionComplete recognizes the provider's complete cleanup
// contract. A joined absence report cannot hide failed network/ownership cleanup.
// Filesystem ENOENT is not provider absence (it may mean a missing executable).
func EnvironmentDeletionComplete(err error) bool {
	if err == nil {
		return true
	}
	return onlyEnvironmentNotFound(err)
}

func onlyEnvironmentNotFound(err error) bool {
	if err == ErrNotFound {
		return true
	}
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		causes := wrapped.Unwrap()
		if len(causes) == 0 {
			return false
		}
		for _, cause := range causes {
			if !onlyEnvironmentNotFound(cause) {
				return false
			}
		}
		return true
	case interface{ Unwrap() error }:
		return onlyEnvironmentNotFound(wrapped.Unwrap())
	default:
		return false
	}
}
