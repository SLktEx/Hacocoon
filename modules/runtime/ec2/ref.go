package ec2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type runtimeRef struct {
	Version       int    `json:"version"`
	InstanceID    string `json:"instance_id"`
	WorkspacePath string `json:"workspace_path"`
	Bucket        string `json:"bucket"`
	Prefix        string `json:"prefix"`
	ReadOnly      bool   `json:"read_only"`
}

func encodeRef(ref runtimeRef) (string, error) {
	ref.Version = 1
	payload, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	return "ec2v1." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeRef(raw string) (runtimeRef, error) {
	if !strings.HasPrefix(raw, "ec2v1.") {
		return runtimeRef{}, fmt.Errorf("EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(raw, "ec2v1."))
	if err != nil {
		return runtimeRef{}, fmt.Errorf("decode EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	var ref runtimeRef
	if err := json.Unmarshal(payload, &ref); err != nil || ref.Version != 1 || ref.InstanceID == "" || ref.WorkspacePath == "" || ref.Bucket == "" || ref.Prefix == "" {
		return runtimeRef{}, fmt.Errorf("invalid EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	return ref, nil
}
