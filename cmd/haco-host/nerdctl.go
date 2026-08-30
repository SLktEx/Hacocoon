package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func nerdctlCommand(ctx context.Context, client controllerClient, args []string) error {
	namespace, args, err := parseNerdctlNamespace(args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return nerdctlUsageError()
	}

	switch args[0] {
	case "run":
		if namespace == "" {
			return fmt.Errorf("nerdctl shim requires --namespace <haco-environment> for run: %w", core.ErrInvalidArgument)
		}
		return nerdctlRunCommand(ctx, client, namespace, args[1:])
	case "ps", "container":
		if args[0] == "container" {
			if len(args) < 2 || args[1] != "ls" {
				return nerdctlUnsupported(args)
			}
			args = append([]string{"ps"}, args[2:]...)
		}
		if namespace == "" {
			return fmt.Errorf("nerdctl shim requires --namespace <haco-environment> for ps: %w", core.ErrInvalidArgument)
		}
		return nerdctlPSCommand(ctx, client, namespace, args[1:])
	case "exec":
		if namespace == "" {
			return fmt.Errorf("nerdctl shim requires --namespace <haco-environment> for exec: %w", core.ErrInvalidArgument)
		}
		return nerdctlExecCommand(ctx, client, namespace, args[1:])
	case "stop":
		if namespace == "" {
			return fmt.Errorf("nerdctl shim requires --namespace <haco-environment> for stop: %w", core.ErrInvalidArgument)
		}
		return nerdctlStopCommand(ctx, client, namespace, args[1:])
	case "rm", "remove":
		if namespace == "" {
			return fmt.Errorf("nerdctl shim requires --namespace <haco-environment> for rm: %w", core.ErrInvalidArgument)
		}
		return nerdctlRMCommand(ctx, client, namespace, args[1:])
	case "pull":
		return nerdctlPullCommand(ctx, client, args[1:])
	case "build", "compose", "save", "load", "push", "namespace":
		return nerdctlUnsupported(args)
	case "--version", "version":
		fmt.Println("nerdctl (Hacocoon Incus OCI compatibility shim)")
		return nil
	default:
		return nerdctlUnsupported(args)
	}
}

func parseNerdctlNamespace(args []string) (string, []string, error) {
	namespace := strings.TrimSpace(os.Getenv("HACO_NERDCTL_NAMESPACE"))
	for len(args) > 0 {
		switch {
		case args[0] == "--namespace" || args[0] == "-n":
			if len(args) < 2 || namespace != "" {
				return "", nil, nerdctlUsageError()
			}
			namespace = args[1]
			args = args[2:]
		case strings.HasPrefix(args[0], "--namespace="):
			if namespace != "" {
				return "", nil, nerdctlUsageError()
			}
			namespace = strings.TrimPrefix(args[0], "--namespace=")
			args = args[1:]
		case args[0] == "--debug" || args[0] == "--address" || args[0] == "-a":
			if args[0] == "--debug" {
				args = args[1:]
				continue
			}
			return "", nil, fmt.Errorf("nerdctl containerd address flags are not supported by the Incus shim: %w", core.ErrUnsupported)
		default:
			if namespace != "" && strings.TrimSpace(namespace) != namespace {
				return "", nil, core.ErrInvalidArgument
			}
			return namespace, args, nil
		}
	}
	return namespace, args, nil
}

func nerdctlRunCommand(ctx context.Context, client controllerClient, environment string, args []string) error {
	var (
		name      string
		ephemeral bool
		env       = map[string]string{}
	)
	for len(args) > 0 {
		switch args[0] {
		case "-d", "--detach":
			args = args[1:]
		case "--rm":
			ephemeral = true
			args = args[1:]
		case "--name":
			if len(args) < 2 || name != "" {
				return nerdctlUsageError()
			}
			name = args[1]
			args = args[2:]
		case "-e", "--env":
			if len(args) < 2 {
				return nerdctlUsageError()
			}
			key, value, ok := strings.Cut(args[1], "=")
			if !ok || key == "" {
				return fmt.Errorf("nerdctl shim requires KEY=VALUE for --env: %w", core.ErrInvalidArgument)
			}
			env[key] = value
			args = args[2:]
		default:
			if strings.HasPrefix(args[0], "-") {
				return nerdctlUnsupported(args)
			}
			goto parsed
		}
	}

parsed:
	if len(args) == 0 {
		return nerdctlUsageError()
	}
	image, err := nerdctlImageToIncus(args[0])
	if err != nil {
		return err
	}
	if name == "" {
		name, err = randomWorkloadName()
		if err != nil {
			return err
		}
	}
	created, err := client.CreateWorkload(ctx, core.WorkloadSpec{
		Environment:          environment,
		Name:                 name,
		Image:                image,
		Command:              append([]string(nil), args[1:]...),
		EnvironmentVariables: env,
		Ephemeral:            ephemeral,
	})
	if err != nil {
		return err
	}
	fmt.Println(created.RuntimeRef)
	return nil
}

func nerdctlPSCommand(ctx context.Context, client controllerClient, environment string, args []string) error {
	all := false
	quiet := false
	for _, arg := range args {
		switch arg {
		case "-a", "--all":
			all = true
		case "-q", "--quiet":
			quiet = true
		default:
			return nerdctlUnsupported(append([]string{"ps"}, args...))
		}
	}
	items, err := client.ListWorkloads(ctx, environment)
	if err != nil {
		return err
	}
	if !quiet {
		fmt.Println("CONTAINER ID\tIMAGE\tSTATUS\tNAMES")
	}
	for _, item := range items {
		if !all && !strings.EqualFold(item.State, "RUNNING") {
			continue
		}
		if quiet {
			fmt.Println(item.RuntimeRef)
			continue
		}
		fmt.Printf("%s\t%s\t%s\t%s\n", item.RuntimeRef, item.Image, item.State, item.Name)
	}
	return nil
}

func nerdctlExecCommand(ctx context.Context, client controllerClient, environment string, args []string) error {
	for len(args) > 0 {
		switch args[0] {
		case "-i", "--interactive", "-t", "--tty", "-it", "-ti":
			args = args[1:]
		default:
			goto parsed
		}
	}
parsed:
	if len(args) < 2 {
		return nerdctlUsageError()
	}
	result, err := client.ExecWorkload(ctx, environment, args[0], args[1:])
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	if err != nil {
		return err
	}
	if result.ExitCode > 0 {
		return commandExitError{code: result.ExitCode}
	}
	return nil
}

func nerdctlStopCommand(ctx context.Context, client controllerClient, environment string, args []string) error {
	if len(args) == 0 {
		return nerdctlUsageError()
	}
	for _, name := range args {
		if strings.HasPrefix(name, "-") {
			return nerdctlUnsupported(append([]string{"stop"}, args...))
		}
		if err := client.StopWorkload(ctx, environment, name); err != nil {
			return err
		}
		fmt.Println(name)
	}
	return nil
}

func nerdctlRMCommand(ctx context.Context, client controllerClient, environment string, args []string) error {
	for len(args) > 0 && (args[0] == "-f" || args[0] == "--force") {
		args = args[1:]
	}
	if len(args) == 0 {
		return nerdctlUsageError()
	}
	for _, name := range args {
		if strings.HasPrefix(name, "-") {
			return nerdctlUnsupported(append([]string{"rm"}, args...))
		}
		if err := client.DeleteWorkload(ctx, environment, name); err != nil {
			return err
		}
		fmt.Println(name)
	}
	return nil
}

func nerdctlPullCommand(ctx context.Context, client controllerClient, args []string) error {
	if len(args) != 1 || strings.HasPrefix(args[0], "-") {
		return nerdctlUsageError()
	}
	image, err := nerdctlImageToIncus(args[0])
	if err != nil {
		return err
	}
	if err := client.PullWorkloadImage(ctx, image); err != nil {
		return err
	}
	fmt.Println(args[0])
	return nil
}

// nerdctlImageToIncus keeps Docker Hub ergonomic while still allowing an
// explicitly configured Incus OCI remote to be addressed as "remote::image".
// Private-registry credential handoff is intentionally a controller/Broker
// concern and is not persisted by this shim.
func nerdctlImageToIncus(image string) (string, error) {
	image = strings.TrimSpace(image)
	if image == "" || strings.HasPrefix(image, "-") || strings.ContainsAny(image, "\x00\r\n") {
		return "", core.ErrInvalidArgument
	}
	if remote, rest, ok := strings.Cut(image, "::"); ok {
		if remote == "" || rest == "" || strings.ContainsAny(remote, "/ \t") {
			return "", core.ErrInvalidArgument
		}
		return remote + ":" + rest, nil
	}

	if strings.HasPrefix(image, "docker.io/") {
		image = strings.TrimPrefix(image, "docker.io/")
	}
	if slash := strings.IndexByte(image, '/'); slash >= 0 {
		first := image[:slash]
		if first == "localhost" || strings.Contains(first, ".") || strings.Contains(first, ":") {
			return "", fmt.Errorf("registry %q needs an Incus OCI remote; use <remote>::<image> after haco registry login/configuration: %w", first, core.ErrUnsupported)
		}
	}
	if !strings.Contains(image, "/") {
		image = "library/" + image
	}
	return "oci-docker:" + image, nil
}

func randomWorkloadName() (string, error) {
	var value [5]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate workload name: %w", err)
	}
	return "wl-" + hex.EncodeToString(value[:]), nil
}

func nerdctlUnsupported(args []string) error {
	command := ""
	if len(args) > 0 {
		command = args[0]
	}
	return fmt.Errorf("nerdctl %s is not implemented by the Incus OCI shim; build/compose and other containerd-specific operations remain separate tooling: %w", command, core.ErrUnsupported)
}

func nerdctlUsageError() error {
	return fmt.Errorf("usage: nerdctl [--namespace <haco-environment>] <run|ps|exec|stop|rm|pull> ...: %w", core.ErrInvalidArgument)
}
