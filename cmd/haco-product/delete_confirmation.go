package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// confirmDataDeletion only owns the common client warning and confirmation.
// Callers must first review the exact target; canonical controller operations
// still own reference checks, deletion and recovery. A failed display cannot
// authorize deletion, including when --yes skips interactive input.
func confirmDataDeletion(in io.Reader, diagnostic io.Writer, yes bool, warning, prompt, retained string) int {
	if _, err := fmt.Fprintln(diagnostic, cliMessage(warning)); err != nil {
		return 1
	}
	if yes {
		return 0
	}
	if !requireInteractiveConfirmation(in, diagnostic) {
		return 2
	}
	if _, err := fmt.Fprint(diagnostic, cliMessage(prompt)); err != nil {
		return 1
	}
	answer, err := bufio.NewReader(io.LimitReader(in, 128)).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if err != nil || (answer != "y" && answer != "yes") {
		fmt.Fprintln(diagnostic, cliMessage(retained))
		return 1
	}
	return 0
}
