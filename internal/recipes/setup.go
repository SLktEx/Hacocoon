package recipes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrExecutionFailed = errors.New("setup recipe failed")

const MaxScriptBytes = 1 << 20

// Update is explicit caller intent. Nil Script replays the stored recipe.
type Update struct {
	Script *string `json:"script,omitempty"`
	Clear  bool    `json:"clear_script,omitempty"`
}

func (u Update) Validate() error {
	if u.Clear && u.Script != nil {
		return fmt.Errorf("script and clear_script are mutually exclusive")
	}
	if u.Script != nil && (len(*u.Script) > MaxScriptBytes || !utf8.ValidString(*u.Script) || strings.ContainsRune(*u.Script, 0)) {
		return fmt.Errorf("setup script must be UTF-8 without NUL and at most 1 MiB")
	}
	return nil
}

type Service struct {
	Root    string
	Execute func(context.Context, []byte) error
}

func (s *Service) Apply(ctx context.Context, update Update) (resultErr error) {
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(ErrExecutionFailed, resultErr)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := update.Validate(); err != nil {
		return err
	}
	if s == nil || s.Execute == nil {
		return fmt.Errorf("setup recipe executor is unavailable")
	}
	files, err := openStore(s.Root)
	if err != nil {
		return err
	}
	defer files.close()
	if update.Clear {
		return files.remove()
	}
	if update.Script != nil {
		script := strings.ReplaceAll(strings.TrimPrefix(*update.Script, "\ufeff"), "\r\n", "\n")
		if err := files.save([]byte(script)); err != nil {
			return err
		}
	}
	script, err := files.read()
	if err != nil {
		return err
	}
	if script == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Execute(ctx, script)
}

func ReadScript(path string) ([]byte, error) {
	f, err := openInput(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("setup script must be a regular file")
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxScriptBytes+1))
	if err != nil {
		return nil, err
	}
	text := string(data)
	if err := (Update{Script: &text}).Validate(); err != nil {
		return nil, err
	}
	return data, nil
}
