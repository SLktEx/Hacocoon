package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type hostControllerClient interface {
	OpenTrustedHostShell(context.Context) (net.Conn, error)
}

type hostControllerFactory func() (hostControllerClient, error)

// `haco host shell` is a normal controller-client operation and therefore must
// be intercepted before main initializes composition.Local(). `haco host
// ensure` deliberately stays on the Physical Host bootstrap/recovery path.
func init() {
	handled, err := handleHostClientArgs(
		context.Background(),
		os.Args[1:],
		newDefaultHostController,
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if !handled {
		return
	}
	if err != nil {
		code := 1
		var exitCoder interface{ ExitCode() int }
		if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
			code = exitCoder.ExitCode()
		}
		if message := strings.TrimSpace(err.Error()); message != "" {
			fmt.Fprintln(os.Stderr, "haco:", message)
		}
		os.Exit(code)
	}
	os.Exit(0)
}

func newDefaultHostController() (hostControllerClient, error) {
	return controlapi.NewDefaultClient()
}

func handleHostClientArgs(
	ctx context.Context,
	args []string,
	factory hostControllerFactory,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	if len(args) < 2 || args[0] != "host" || args[1] != "shell" {
		return false, nil
	}
	if factory == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, core.ErrInvalidArgument
	}
	client, err := factory()
	if err != nil {
		return true, err
	}
	return true, hostClientShell(ctx, client, args[2:], stdin, stdout, stderr)
}

func hostClientShell(
	ctx context.Context,
	client hostControllerClient,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if client == nil || len(args) != 0 {
		return fmt.Errorf("usage: haco host shell: %w", core.ErrInvalidArgument)
	}
	stream, err := client.OpenTrustedHostShell(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(stderr, trustedHostLoginWarning())
	return bridgeControllerStream(ctx, stream, stdin, stdout)
}
