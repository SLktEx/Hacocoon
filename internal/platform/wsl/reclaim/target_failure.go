package wslreclaim

type targetFailure struct {
	stage string
	err   error
}

func (e *targetFailure) Error() string { return "reclamation " + e.stage + ": " + e.err.Error() }
func (e *targetFailure) Unwrap() error { return e.err }
func targetStage(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &targetFailure{stage, err}
}
