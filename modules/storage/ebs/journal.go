package ebs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

type FileJournal struct{ root string }

func NewFileJournal(root string) *FileJournal { return &FileJournal{root: root} }

var journalID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func (j *FileJournal) Save(_ context.Context, op Operation) error {
	if j == nil || !journalID.MatchString(op.ID) {
		return fmt.Errorf("invalid EBS journal id")
	}
	if err := os.MkdirAll(j.root, 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(op, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(j.root, op.ID+".json")
	tmp, err := os.CreateTemp(j.root, "."+op.ID+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	defer cleanup()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(payload); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}
func (j *FileJournal) Delete(_ context.Context, id string) error {
	if j == nil || !journalID.MatchString(id) {
		return fmt.Errorf("invalid EBS journal id")
	}
	err := os.Remove(filepath.Join(j.root, id+".json"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
