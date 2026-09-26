//go:build linux

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitadapter "github.com/SLktEx/Hacocoon/internal/adapters/git"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

type defaultOpenClient interface {
	workflowClient
	RepositoryManage(context.Context, controlapi.RepositoryManageRequest) (controlapi.RepositoryManageResponse, error)
}

// This is a client navigation reference, using the same pinned, locked and
// durable file format as explicit directory opens. It is never guest authority.
func defaultOpenDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) {
		return "", core.ErrInvalidArgument
	}
	path := filepath.Join(home, ".haco-default")
	if err := os.Mkdir(path, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", err
	}
	// LockReference refuses symlinks, foreign owners and writable directories.
	return path, nil
}

func openDefaultWorkspace(ctx context.Context, c defaultOpenClient, path, base, oci string, diagnostic io.Writer) (workflow.OpenResult, error) {
	var result workflow.OpenResult
	h, err := workflow.LockReference(ctx, path)
	if err != nil {
		return result, err
	}
	defer h.Close()
	var repositories []string
	err = openStage(ctx, diagnostic, "repositories", func() error {
		response, e := c.RepositoryManage(ctx, controlapi.RepositoryManageRequest{Operation: "list"})
		if e != nil {
			return e
		}
		for _, use := range response.Sources {
			if use.Source.Kind != "repo" || !gitadapter.ValidID(use.Source.ID) || use.Source.State != "ready" {
				return core.ErrRecoveryRequired
			}
			repositories = append(repositories, use.Source.ID)
		}
		sort.Strings(repositories)
		if len(repositories) == 0 {
			return errors.New(cliMessage("open.no_repositories"))
		}
		if len(repositories) > 8 {
			return errors.New(cliMessage("open.too_many_repositories"))
		}
		for i := 1; i < len(repositories); i++ {
			if repositories[i] == repositories[i-1] {
				return core.ErrIncompatibleState
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	var ref workflow.PathReference
	err = openStage(ctx, diagnostic, "project", func() error {
		var e error
		ref, e = preparePath(ctx, c, h, pathOpenOptions{Repositories: strings.Join(repositories, ","), Base: base, OCI: oci})
		return e
	})
	if err != nil {
		return result, err
	}
	err = openStage(ctx, diagnostic, "environment", func() error {
		var e error
		result, e = openPreparedWorkspace(ctx, c, h, ref, pathOpenOptions{Base: base, OCI: oci})
		return e
	})
	return result, err
}

// Progress is user output on stderr, not raw backend output or an application
// log. Join the ticker before returning so callers can safely reuse the writer.
func openStage(ctx context.Context, out io.Writer, stage string, run func() error) error {
	label := cliMessage("open.stage." + stage)
	if _, err := fmt.Fprintln(out, cliMessage("open.progress", label)); err != nil {
		return err
	}
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = fmt.Fprintln(out, cliMessage("open.waiting", label))
			}
		}
	}()
	err := run()
	close(done)
	<-joined
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	_, err = fmt.Fprintln(out, cliMessage("open.finished", label))
	return err
}
