//go:build linux

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var errEnvironmentChoiceCanceled = errors.New("Environment selection canceled")

// Selection is presentation only; SetupSelected rechecks the selected identity.
func chooseDesktopEnvironment(environments []core.Environment, interactive bool, in io.Reader, out io.Writer) (core.Environment, error) {
	if len(environments) == 0 {
		return core.Environment{}, fmt.Errorf("no Environments; create one with haco create")
	}
	if len(environments) == 1 {
		return environments[0], nil
	}
	if len(environments) > 1000 {
		return core.Environment{}, fmt.Errorf("too many Environments to display; specify a name")
	}
	choices := append([]core.Environment(nil), environments...)
	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })
	for i, env := range choices {
		if env.Name == "" || i > 0 && env.Name == choices[i-1].Name {
			return core.Environment{}, core.ErrIncompatibleState
		}
		if _, err := fmt.Fprintf(out, "%d. %q  Workspace: %q (%s)\n", i+1, env.Name, env.Workspace.ID, strconv.Quote(string(env.AccessMode))); err != nil {
			return core.Environment{}, err
		}
	}
	if !interactive {
		return core.Environment{}, fmt.Errorf("specify an Environment name when input is not interactive")
	}
	if _, err := fmt.Fprint(out, "Choose an Environment (blank cancels): "); err != nil {
		return core.Environment{}, err
	}
	reader := bufio.NewReader(io.LimitReader(in, 129))
	line, err := reader.ReadString('\n')
	if len(line) > 128 || err != nil && err != io.EOF {
		return core.Environment{}, core.ErrInvalidArgument
	}
	answer := strings.TrimSpace(line)
	if answer == "" {
		return core.Environment{}, errEnvironmentChoiceCanceled
	}
	if err != nil {
		return core.Environment{}, core.ErrInvalidArgument
	}
	selected, err := strconv.Atoi(answer)
	if err != nil || selected < 1 || selected > len(choices) {
		return core.Environment{}, core.ErrInvalidArgument
	}
	return choices[selected-1], nil
}
