// Package desktopreview defines the optional Windows notification launch boundary.
// It never accepts an answer, credentials, commands or controller endpoints.
package desktopreview

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

type Invocation struct {
	File string
	Args []string
	Env  []string
}

func Plan(distribution, uri, systemRoot string) (Invocation, error) {
	scheme, err := Scheme(distribution)
	prefix := scheme + "://request/"
	if err != nil || !strings.HasPrefix(uri, prefix) || !requestPattern.MatchString(strings.TrimPrefix(uri, prefix)) || !windowsRootPattern.MatchString(systemRoot) {
		return Invocation{}, ErrInvalid
	}
	// No URL decoding or normalization: escaped separators, query strings and
	// alternate authorities cannot change the one request selected for review.
	id := strings.TrimPrefix(uri, prefix)
	root := strings.TrimRight(systemRoot, "\\")
	return Invocation{
		File: root + "\\System32\\wsl.exe",
		Args: []string{"--distribution", distribution, "--exec", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "/usr/local/bin/haco", "approve", id},
		Env:  []string{"SystemRoot=" + root, "WINDIR=" + root},
	}, nil
}
