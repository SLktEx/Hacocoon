package egressproxy

import (
	"net"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

func parseAuthority(authority string, defaultPort int) (string, int, error) {
	if authority == "" || strings.ContainsAny(authority, "\r\n\x00/@") {
		return "", 0, core.ErrInvalidArgument
	}
	host := authority
	port := defaultPort
	if parsedHost, parsedPort, err := net.SplitHostPort(authority); err == nil {
		host = parsedHost
		value, convErr := strconv.Atoi(parsedPort)
		if convErr != nil || value < 1 || value > 65535 {
			return "", 0, core.ErrInvalidArgument
		}
		port = value
	} else if strings.Contains(authority, ":") {
		return "", 0, core.ErrInvalidArgument
	}
	canonical, err := egressapp.CanonicalHost(host)
	if err != nil {
		return "", 0, err
	}
	return canonical, port, nil
}
