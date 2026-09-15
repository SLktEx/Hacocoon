package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

type SettingsSnapshot struct {
	Revision      string        `json:"revision"`
	Configuration Configuration `json:"configuration"`
}

type Settings struct{ Path string }

func settingsRevision(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s Settings) Select(ctx context.Context, generations Generations, repositories Repositories) (*Selector, error) {
	snapshot, err := s.Read(ctx)
	if err != nil {
		return nil, err
	}
	return NewSelector(snapshot.Configuration, generations, repositories)
}
