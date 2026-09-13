// Package wsllaunch owns fixed local Windows-to-WSL child invocations.
package wsllaunch

import (
	"errors"
	"regexp"
	"strings"
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
	if !distributionPattern.MatchString(distribution) || !rootPattern.MatchString(systemRoot) || (operation != Review && operation != ControlStdio) {
		return Invocation{}, errors.New("invalid fixed WSL invocation")
	}
	root := strings.TrimRight(systemRoot, "\\")
	for _, part := range strings.Split(root[3:], "\\") {
		if part == "" || part == "." || part == ".." {
			return Invocation{}, errors.New("invalid Windows system root")
		}
	}
	return Invocation{File: root + "\\System32\\wsl.exe", Args: []string{"--distribution", distribution, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", string(operation)}, Env: []string{"SystemRoot=" + root, "WINDIR=" + root}}, nil
}
