package gitadapter

import (
	"net/url"
	"strings"
)

// CanonicalRemote maps supported GitHub transports to the same credential-free
// Host route. Other hosts are never equated; existing file routes stay exact.
func CanonicalRemote(raw string) (string, error) {
	value := raw
	if strings.HasPrefix(strings.ToLower(value), "git@github.com:") {
		value = "https://github.com/" + value[len("git@github.com:"):]
	}
	u, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	if strings.EqualFold(u.Host, "github.com") && (u.Scheme == "https" || u.Scheme == "ssh") && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" {
		if u.User != nil && (u.Scheme != "ssh" || u.User.String() != "git") {
			return "", ValidateRemote(raw)
		}
		path := strings.ToLower(strings.TrimSuffix(strings.TrimRight(u.Path, "/"), ".git"))
		value = "https://github.com" + path
	}
	if err := ValidateRemote(value); err != nil {
		return "", err
	}
	return value, nil
}
