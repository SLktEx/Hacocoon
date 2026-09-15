//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"errors"
	"syscall"
	"testing"
)

func TestPreparationFailureRetainsCauseAndOnlyExportsKnownStage(t *testing.T) {
	for _, stage := range []string{"disk_access", "enrollment", "binding", "intent", "private-content"} {
		err := targetStage(stage, errors.Join(syscall.Errno(5), errors.New("private-content")))
		result := PreparationFailure(err)
		if !result.Valid() || result.NativeError != 5 || !errors.Is(err, syscall.Errno(5)) {
			t.Fatal(result, err)
		}
		if stage == "private-content" && result.Stage != "other" {
			t.Fatal(result)
		}
	}
}
