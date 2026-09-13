package sshconfig

import (
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"regexp"
)

var distroPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

// StreamCommand accepts only an encoded typed target and a literal registered
// distribution name. OpenSSH expansions and shell metacharacters cannot enter it.
func StreamCommand(target *core.StreamTarget, distro string) (string, error) {
	if target == nil {
		return "", core.ErrIncompatibleState
	}
	token, err := core.EncodeStreamTarget(*target)
	if err != nil {
		return "", err
	}
	if distro == "" {
		return "/usr/local/bin/haco stream " + token, nil
	}
	if !distroPattern.MatchString(distro) {
		return "", core.ErrInvalidArgument
	}
	return fmt.Sprintf("C:/Windows/System32/wsl.exe --distribution %s --exec /usr/local/bin/haco stream %s", distro, token), nil
}
