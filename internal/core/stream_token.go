package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"regexp"
)

var streamTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func EncodeStreamTarget(t StreamTarget) (string, error) {
	if !t.Valid() {
		return "", ErrInvalidArgument
	}
	b, err := json.Marshal(t)
	if err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	if len(token) > 4096 {
		return "", ErrInvalidArgument
	}
	return token, nil
}

func DecodeStreamTarget(token string) (StreamTarget, error) {
	var t StreamTarget
	if len(token) > 4096 || !streamTokenPattern.MatchString(token) {
		return t, ErrInvalidArgument
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return t, ErrInvalidArgument
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&t) != nil || !t.Valid() {
		return StreamTarget{}, ErrInvalidArgument
	}
	canonical, err := EncodeStreamTarget(t)
	if err != nil || canonical != token {
		return StreamTarget{}, ErrInvalidArgument
	}
	return t, nil
}
