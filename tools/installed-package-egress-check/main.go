// installed-package-egress-check is a CI acceptance client for the product-owned
// package repository egress baseline. It uses the installed controller as the
// ordinary Physical Host user and never edits administrator Policy.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

var acceptanceName = regexp.MustCompile(`^m1-egress-[a-f0-9]{16}$`)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: installed-package-egress-check <environment>")
		os.Exit(2)
	}
	if err := check(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("package_update=allowed package_install=allowed third_party_source=denied")
}

func check(name string) (result error) {
	if os.Geteuid() == 0 || !acceptanceName.MatchString(name) {
		return errors.New("check requires the ordinary WSL user and a unique acceptance name")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	client, err := controlapi.NewClient(control.DefaultSocketPath)
	if err != nil {
		return err
	}
	workspace, err := os.MkdirTemp("", name+"-package-")
	if err != nil {
		return err
	}
	if err := os.Chmod(workspace, 0o755); err != nil {
		return err
	}
	created, err := client.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{
		Name: name, WorkspacePath: workspace, AccessMode: core.WorkspaceReadOnly, SkipDefaultResource: true,
	})
	if err != nil {
		return fmt.Errorf("controller create failed; retain workspace %s: %w", workspace, err)
	}
	if created.Name != name || created.RuntimeRef == "" || created.Workspace.Path != workspace {
		return errors.New("unexpected created identity; retaining resources for inspection")
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Minute)
		defer stop()
		if err := client.DeleteEnvironment(cleanup, name); err != nil {
			result = errors.Join(result, fmt.Errorf("controller cleanup failed; retain workspace %s: %w", workspace, err))
			return
		}
		remaining, err := client.ListEnvironments(cleanup)
		if err != nil {
			result = errors.Join(result, errors.New("cleanup inventory unavailable; retaining workspace"))
			return
		}
		for _, environment := range remaining {
			if environment.Name == name {
				result = errors.Join(result, errors.New("deleted Environment still present; retaining workspace"))
				return
			}
		}
		result = errors.Join(result, os.Remove(workspace))
	}()

	// The first two commands exercise only the fixed Ubuntu package endpoints.
	// The temporary third-party source must not expand egress authority; force
	// APT to report any repository acquisition failure as a non-zero result, then
	// independently prove the same hostname is still refused by the Standard
	// proxy after guest-controlled APT configuration referenced it.
	script := `set -eu
export DEBIAN_FRONTEND=noninteractive
apt-get -qq update
apt-get -qq install -y --reinstall --no-install-recommends ca-certificates curl
third_party=/tmp/haco-third-party.list
printf 'deb [trusted=yes] http://example.com/ubuntu stable main\n' > "$third_party"
if apt-get -qq -o APT::Update::Error-Mode=any -o Dir::Etc::sourcelist="$third_party" -o Dir::Etc::sourceparts='-' -o APT::Get::List-Cleanup=0 update >/tmp/haco-third-party.out 2>&1; then
  rm -f "$third_party" /tmp/haco-third-party.out
  exit 91
fi
status="$(curl --silent --output /dev/null --write-out '%{http_code}' --proxy "$HTTP_PROXY" http://example.com/ || true)"
rm -f "$third_party" /tmp/haco-third-party.out
[ "$status" = 403 ]
`
	executed, err := client.ExecEnvironment(ctx, name, []string{"/bin/sh", "-lc", script})
	if err != nil {
		return fmt.Errorf("controller package exec failed: %w", err)
	}
	if executed.ExitCode != 0 || executed.StdoutTruncated || executed.StderrTruncated {
		return fmt.Errorf("package egress baseline acceptance failed (exit %d)", executed.ExitCode)
	}
	return nil
}
