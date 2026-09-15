// Package wsllaunch owns fixed local Windows-to-WSL child invocations.
package wsllaunch

import (
	"errors"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

type Operation string

const Review Operation = "_desktop-review"
const ControlStdio Operation = "_control-stdio"

var distributionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var rootPattern = regexp.MustCompile(`^[A-Za-z]:\\[^\r\n"<>|?*:]+$`)

type Invocation struct {
	File      string
	Args, Env []string
}

func Plan(distribution, systemRoot string, operation Operation) (Invocation, error) {
	if !distributionPattern.MatchString(distribution) {
		return Invocation{}, errors.New("invalid fixed WSL invocation")
	}
	return fixedPlan("--distribution", distribution, systemRoot, operation)
}

// RegisteredPlan retains the discovered registration instead of reselecting a
// possibly replaced distribution by its display name. Installation identity is
// checked through the controller before a delegated listener is opened.
func RegisteredPlan(target reclamation.WSLTarget, systemRoot string) (Invocation, error) {
	if target.Validate() != nil {
		return Invocation{}, errors.New("invalid WSL installation")
	}
	return fixedPlan("--distribution-id", target.RegistrationID, systemRoot, ControlStdio)
}

func fixedPlan(selector, distribution, systemRoot string, operation Operation) (Invocation, error) {
	if !rootPattern.MatchString(systemRoot) || (operation != Review && operation != ControlStdio) {
		return Invocation{}, errors.New("invalid fixed WSL invocation")
	}
	root := strings.TrimRight(systemRoot, "\\")
	for _, part := range strings.Split(root[3:], "\\") {
		if part == "" || part == "." || part == ".." {
			return Invocation{}, errors.New("invalid Windows system root")
		}
	}
	return Invocation{File: root + "\\System32\\wsl.exe", Args: []string{selector, distribution, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", string(operation)}, Env: []string{"SystemRoot=" + root, "WINDIR=" + root}}, nil
}
