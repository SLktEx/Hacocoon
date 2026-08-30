package ec2

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	createJournalVersion = 1
	createTokenBytes      = 24
	maxCreateJournalBytes = 4096
)

type createJournal struct {
	dir string
}

type createOperation struct {
	Key         string
	Fingerprint string
	ClientToken string
}

type createJournalRecord struct {
	Version     int    `json:"version"`
	Key         string `json:"key"`
	Fingerprint string `json:"fingerprint"`
	ClientToken string `json:"client_token"`
}

type createFingerprintInput struct {
	AccountID        string   `json:"account_id"`
	Region           string   `json:"region"`
	Name             string   `json:"name"`
	WorkspacePath    string   `json:"workspace_path"`
	WorkspaceDigest  string   `json:"workspace_digest"`
	ReadOnly         bool     `json:"read_only"`
	ImageID          string   `json:"image_id"`
	InstanceType     string   `json:"instance_type"`
	SubnetID         string   `json:"subnet_id"`
	SecurityGroupIDs []string `json:"security_group_ids"`
	InstanceProfile  string   `json:"instance_profile"`
	WorkspaceBucket  string   `json:"workspace_bucket"`
	WorkspacePrefix  string   `json:"workspace_prefix"`
}

func newCreateJournal(dir string) (*createJournal, error) {
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." || strings.ContainsRune(dir, '\x00') {
		return nil, core.ErrInvalidArgument
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create EC2 create-journal directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, fmt.Errorf("inspect EC2 create-journal directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return nil, fmt.Errorf("EC2 create-journal directory is not trusted: %w", core.ErrIncompatibleState)
	}
	return &createJournal{dir: dir}, nil
}

func (j *createJournal) prepare(accountID string, cfg Config, spec core.EnvironmentRuntimeSpec) (createOperation, error) {
	if j == nil || strings.TrimSpace(j.dir) == "" {
		return createOperation{}, fmt.Errorf("EC2 create journal is not configured: %w", core.ErrRuntimeUnavailable)
	}
	workspaceDigest, err := digestWorkspace(context.Background(), spec.WorkspacePath)
	if err != nil {
		return createOperation{}, fmt.Errorf("identify EC2 create workspace: %w", err)
	}
	key := createOperationKey(accountID, cfg.Region, spec.Name)
	fingerprint, err := createRequestFingerprint(accountID, cfg, spec, workspaceDigest)
	if err != nil {
		return createOperation{}, err
	}
	path := filepath.Join(j.dir, key+".json")
	if existing, err := j.read(path); err == nil {
		return matchCreateOperation(existing, key, fingerprint)
	} else if !errors.Is(err, os.ErrNotExist) {
		return createOperation{}, err
	}

	tokenBytes := make([]byte, createTokenBytes)
	if _, err := rand.Read(tokenBytes); err != nil {
		return createOperation{}, fmt.Errorf("generate EC2 create client token: %w", err)
	}
	record := createJournalRecord{
		Version:     createJournalVersion,
		Key:         key,
		Fingerprint: fingerprint,
		ClientToken: hex.EncodeToString(tokenBytes),
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return createOperation{}, fmt.Errorf("encode EC2 create journal: %w", err)
	}
	payload = append(payload, '\n')

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		existing, readErr := j.read(path)
		if readErr != nil {
			return createOperation{}, readErr
		}
		return matchCreateOperation(existing, key, fingerprint)
	}
	if err != nil {
		return createOperation{}, fmt.Errorf("create EC2 create journal: %w", err)
	}
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(payload); err != nil {
		return createOperation{}, fmt.Errorf("write EC2 create journal: %w", err)
	}
	if err := file.Sync(); err != nil {
		return createOperation{}, fmt.Errorf("sync EC2 create journal: %w", err)
	}
	if err := file.Close(); err != nil {
		return createOperation{}, fmt.Errorf("close EC2 create journal: %w", err)
	}
	if err := syncCreateJournalDir(j.dir); err != nil {
		return createOperation{}, err
	}
	committed = true
	return createOperation{Key: key, Fingerprint: fingerprint, ClientToken: record.ClientToken}, nil
}

func (j *createJournal) complete(key, token string) error {
	if j == nil || key == "" || token == "" {
		return nil
	}
	if !validHex(key, sha256.Size) || !validHex(token, createTokenBytes) {
		return fmt.Errorf("invalid EC2 create-journal completion identity: %w", core.ErrIncompatibleState)
	}
	path := filepath.Join(j.dir, key+".json")
	record, err := j.read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if record.Key != key || record.ClientToken != token {
		return errors.Join(
			fmt.Errorf("EC2 create-journal identity does not match persisted operation"),
			core.ErrRecoveryRequired,
		)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove EC2 create journal: %w", err)
	}
	return syncCreateJournalDir(j.dir)
}

func (j *createJournal) read(path string) (createJournalRecord, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return createJournalRecord{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return createJournalRecord{}, fmt.Errorf("EC2 create journal is not a protected regular file: %w", core.ErrIncompatibleState)
	}
	if info.Size() <= 0 || info.Size() > maxCreateJournalBytes {
		return createJournalRecord{}, fmt.Errorf("EC2 create journal size %d: %w", info.Size(), core.ErrIncompatibleState)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return createJournalRecord{}, fmt.Errorf("read EC2 create journal: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var record createJournalRecord
	if err := decoder.Decode(&record); err != nil {
		return createJournalRecord{}, fmt.Errorf("decode EC2 create journal: %w", core.ErrIncompatibleState)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return createJournalRecord{}, fmt.Errorf("EC2 create journal has trailing data: %w", core.ErrIncompatibleState)
	}
	if record.Version != createJournalVersion || !validHex(record.Key, sha256.Size) || !validHex(record.Fingerprint, sha256.Size) || !validHex(record.ClientToken, createTokenBytes) {
		return createJournalRecord{}, fmt.Errorf("invalid EC2 create journal record: %w", core.ErrIncompatibleState)
	}
	return record, nil
}

func matchCreateOperation(record createJournalRecord, key, fingerprint string) (createOperation, error) {
	if record.Key != key || record.Fingerprint != fingerprint {
		return createOperation{}, errors.Join(
			fmt.Errorf("EC2 create operation already exists with different authority, workspace identity, or parameters"),
			core.ErrRecoveryRequired,
		)
	}
	return createOperation{Key: record.Key, Fingerprint: record.Fingerprint, ClientToken: record.ClientToken}, nil
}

func createOperationKey(accountID, region, name string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(accountID) + "\x00" + strings.TrimSpace(region) + "\x00" + strings.TrimSpace(name)))
	return hex.EncodeToString(sum[:])
}

func createRequestFingerprint(accountID string, cfg Config, spec core.EnvironmentRuntimeSpec, workspaceDigest string) (string, error) {
	if !validWorkspaceDigest(workspaceDigest) {
		return "", fmt.Errorf("invalid EC2 create workspace identity: %w", core.ErrIncompatibleState)
	}
	payload, err := json.Marshal(createFingerprintInput{
		AccountID:        accountID,
		Region:           cfg.Region,
		Name:             spec.Name,
		WorkspacePath:    spec.WorkspacePath,
		WorkspaceDigest:  workspaceDigest,
		ReadOnly:         spec.ReadOnly,
		ImageID:          cfg.ImageID,
		InstanceType:     cfg.InstanceType,
		SubnetID:         cfg.SubnetID,
		SecurityGroupIDs: append([]string(nil), cfg.SecurityGroupIDs...),
		InstanceProfile:  cfg.InstanceProfile,
		WorkspaceBucket:  cfg.WorkspaceBucket,
		WorkspacePrefix:  cfg.WorkspacePrefix,
	})
	if err != nil {
		return "", fmt.Errorf("fingerprint EC2 create request: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

func validHex(value string, bytes int) bool {
	if len(value) != bytes*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == bytes
}

func syncCreateJournalDir(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open EC2 create-journal directory for sync: %w", err)
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync EC2 create-journal directory: %w", err)
	}
	return nil
}
