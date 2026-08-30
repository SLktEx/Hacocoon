package ec2

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxRuntimeRefLength = 4096

var (
	ec2InstanceIDPattern = regexp.MustCompile(`^i-[0-9a-f]{8,17}$`)
	awsAccountIDPattern  = regexp.MustCompile(`^[0-9]{12}$`)
)

type runtimeRef struct {
	Version           int    `json:"version"`
	AccountID         string `json:"account_id"`
	Region            string `json:"region"`
	InstanceID        string `json:"instance_id"`
	WorkspacePath     string `json:"workspace_path"`
	Bucket            string `json:"bucket"`
	Prefix            string `json:"prefix"`
	ReadOnly          bool   `json:"read_only"`
	BaseDigest        string `json:"base_digest,omitempty"`
	CreateOperationID string `json:"create_operation_id,omitempty"`
}

func encodeRef(ref runtimeRef) (string, error) {
	ref.Version = 2
	payload, err := json.Marshal(ref)
	if err != nil {
		return "", err
	}
	return "ec2v2." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeRef(raw string) (runtimeRef, error) {
	if len(raw) == 0 || len(raw) > maxRuntimeRefLength {
		return runtimeRef{}, fmt.Errorf("EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	if strings.HasPrefix(raw, "ec2v1.") {
		return runtimeRef{}, errors.Join(
			fmt.Errorf("legacy EC2 runtime ref lacks pinned AWS account/region authority: %w", core.ErrIncompatibleState),
			core.ErrRecoveryRequired,
		)
	}
	if !strings.HasPrefix(raw, "ec2v2.") {
		return runtimeRef{}, fmt.Errorf("EC2 runtime ref: %w", core.ErrIncompatibleState)
	}
	encoded := strings.TrimPrefix(raw, "ec2v2.")
	if encoded == "" {
		return runtimeRef{}, fmt.Errorf("EC2 runtime ref payload: %w", core.ErrIncompatibleState)
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return runtimeRef{}, fmt.Errorf("decode EC2 runtime ref: %w", core.ErrIncompatibleState)
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var ref runtimeRef
	if err := decoder.Decode(&ref); err != nil {
		return runtimeRef{}, fmt.Errorf("decode EC2 runtime ref JSON: %w", core.ErrIncompatibleState)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return runtimeRef{}, fmt.Errorf("EC2 runtime ref has trailing JSON: %w", core.ErrIncompatibleState)
	}
	if err := validateRuntimeRef(ref); err != nil {
		return runtimeRef{}, err
	}
	return ref, nil
}

func validateRuntimeRef(ref runtimeRef) error {
	switch {
	case ref.Version != 2:
		return fmt.Errorf("invalid EC2 runtime ref version: %w", core.ErrIncompatibleState)
	case !awsAccountIDPattern.MatchString(ref.AccountID):
		return fmt.Errorf("invalid EC2 runtime AWS account id: %w", core.ErrIncompatibleState)
	case !regionPattern.MatchString(ref.Region):
		return fmt.Errorf("invalid EC2 runtime AWS region: %w", core.ErrIncompatibleState)
	case !ec2InstanceIDPattern.MatchString(ref.InstanceID):
		return fmt.Errorf("invalid EC2 runtime instance id: %w", core.ErrIncompatibleState)
	case !validRuntimeWorkspacePath(ref.WorkspacePath):
		return fmt.Errorf("invalid EC2 runtime workspace path: %w", core.ErrIncompatibleState)
	case !bucketPattern.MatchString(ref.Bucket):
		return fmt.Errorf("invalid EC2 runtime bucket: %w", core.ErrIncompatibleState)
	case !validRuntimePrefix(ref.Prefix):
		return fmt.Errorf("invalid EC2 runtime prefix: %w", core.ErrIncompatibleState)
	case ref.BaseDigest != "" && !validWorkspaceDigest(ref.BaseDigest):
		return fmt.Errorf("invalid EC2 runtime workspace digest: %w", core.ErrIncompatibleState)
	case ref.CreateOperationID != "" && !createOperationIDPattern.MatchString(ref.CreateOperationID):
		return fmt.Errorf("invalid EC2 runtime create operation identity: %w", core.ErrIncompatibleState)
	default:
		return nil
	}
}

func validRuntimeWorkspacePath(value string) bool {
	if value == "" || hasControl(value) || !filepath.IsAbs(value) {
		return false
	}
	clean := filepath.Clean(value)
	return clean == value && clean != string(filepath.Separator)
}

func validRuntimePrefix(value string) bool {
	if value == "" || hasControl(value) || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
