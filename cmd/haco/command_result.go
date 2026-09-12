package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type commandExitError struct {
	code int
}

func executionResultError(result core.ExecutionResult, err error) error {
	if err != nil {
		return err
	}
	if result.ExitCode > 0 {
		return commandExitError{code: result.ExitCode}
	}
	return nil
}

func fail(err error) {
	code := 1
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
		code = exitCoder.ExitCode()
	}
	message := strings.TrimSpace(err.Error())
	if message != "" {
		fmt.Fprintln(os.Stderr, "haco:", message)
	}
	os.Exit(code)
}

func (e commandExitError) Error() string { return fmt.Sprintf("command exited %d", e.code) }
func (e commandExitError) ExitCode() int { return e.code }
