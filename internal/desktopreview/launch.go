// Package desktopreview defines the optional Windows notification launch boundary.
// Its public launch accepts no answer, credentials, commands or endpoints.
package desktopreview

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrInvalid = errors.New("invalid local approval launch")
var distroPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
var requestPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var windowsRootPattern = regexp.MustCompile(`^[A-Za-z]:\\[^\r\n"<>|?*]+$`)

func Scheme(distribution string) (string, error) {
	if !distroPattern.MatchString(distribution) {
		return "", ErrInvalid
	}
	sum := sha256.Sum256([]byte(strings.ToLower(distribution)))
	return "hacocoon-review-" + hex.EncodeToString(sum[:8]), nil
}
func URI(distribution, request string) (string, error) {
	scheme, err := Scheme(distribution)
	if err != nil || !requestPattern.MatchString(request) {
		return "", ErrInvalid
	}
	return scheme + "://request/" + request, nil
}

// ClassID gives each installed distribution its own native notification server.
// The installer uses the same domain-separated digest and UUID version 8 bits.
func ClassID(distribution string) (string, error) {
	if _, err := Scheme(distribution); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte("Hacocoon.ToastCOM\x00" + strings.ToLower(distribution)))
	sum[6] = (sum[6] & 15) | 0x80
	sum[8] = (sum[8] & 63) | 0x80
	s := hex.EncodeToString(sum[:16])
	return fmt.Sprintf("{%s-%s-%s-%s-%s}", s[:8], s[8:12], s[12:16], s[16:20], s[20:]), nil
}

func RequestFromURI(distribution, uri string) (string, error) {
	scheme, err := Scheme(distribution)
	prefix := scheme + "://request/"
	if err != nil || !strings.HasPrefix(uri, prefix) || !requestPattern.MatchString(strings.TrimPrefix(uri, prefix)) {
		return "", ErrInvalid
	}
	return strings.TrimPrefix(uri, prefix), nil
}

func SessionPlan(distribution, systemRoot string) (Invocation, error) {
	if _, err := Scheme(distribution); err != nil || !windowsRootPattern.MatchString(systemRoot) {
		return Invocation{}, ErrInvalid
	}
	root := strings.TrimRight(systemRoot, "\\")
	return Invocation{File: root + "\\System32\\wsl.exe", Args: []string{"--distribution", distribution, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", "_desktop-review"}, Env: []string{"SystemRoot=" + root, "WINDIR=" + root}}, nil
}

type Invocation struct {
	File string
	Args []string
	Env  []string
}

func Plan(distribution, uri, systemRoot string) (Invocation, error) {
	if _, err := RequestFromURI(distribution, uri); err != nil {
		return Invocation{}, ErrInvalid
	}
	// No URL decoding or normalization: escaped separators, query strings and
	// alternate authorities cannot change the one request selected for review.
	return SessionPlan(distribution, systemRoot)
}
