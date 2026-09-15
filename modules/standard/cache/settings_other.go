//go:build !linux

package cache

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (Settings) Read(context.Context) (SettingsSnapshot, error) {
	return SettingsSnapshot{}, core.ErrUnsupported
}
func (Settings) Replace(context.Context, SettingsSnapshot) (SettingsSnapshot, error) {
	return SettingsSnapshot{}, core.ErrUnsupported
}
