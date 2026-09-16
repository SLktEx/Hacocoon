package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/network"
	"github.com/SLktEx/Hacocoon/internal/policy"
)

type networkClient interface {
	ReadConfiguration(context.Context) (capability.PolicySnapshot, error)
	ReplaceConfiguration(context.Context, capability.PolicySnapshot) (capability.PolicySnapshot, error)
	ListNetworkConnections(context.Context) ([]networkrelay.Session, error)
	RevokeNetworkConnection(context.Context, string) error
	AddNetworkRule(context.Context, networkrelay.RuleSpec) (capability.PolicyRule, error)
}

func runNetwork(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if len(args) > 0 && (args[0] == "tcp" || args[0] == "udp") {
		return networkListenCommand(ctx, args, os.Stdout, os.Stderr)
	}
	client := controlapi.NewDefaultClient()
	return networkCommand(ctx, client, args, os.Stdout, os.Stderr)
}
func networkUsage(out io.Writer) {
	commandHelp(out, "network", cliLanguage())
}
func networkCommand(ctx context.Context, client networkClient, args []string, out, diagnostic io.Writer) int {
	usage := func() int { networkUsage(diagnostic); return 2 }
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(diagnostic, "haco:", flagErr)
		return 2
	}
	args = clean
	result := func(value any, err error, notice ...string) int {
		if err != nil {
			_, _ = fmt.Fprintln(diagnostic, cliMessage("network.failed"), err)
			return 1
		}
		if value != nil {
			if writeCLIResult(out, value, jsonOutput) != nil {
				return 1
			}
		}
		if !jsonOutput && len(notice) != 0 {
			if _, err := fmt.Fprintln(out, cliMessage(notice[0])); err != nil {
				return 1
			}
		}
		return 0
	}
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			networkUsage(out)
			return 0
		case "list":
			v, e := client.ListNetworkConnections(ctx)
			if e == nil && len(v) == 0 && !jsonOutput {
				return result(nil, nil, "network.connections_empty")
			}
			return result(v, e)
		}
	}
	if len(args) == 2 && args[0] == "revoke" {
		return result(nil, client.RevokeNetworkConnection(ctx, args[1]), "network.revoked")
	}
	if len(args) > 1 && args[0] == "host" {
		if args[1] == "add" {
			f := flag.NewFlagSet("network host add", flag.ContinueOnError)
			configureCLIFlags(f, diagnostic)
			var service capability.NetworkService
			f.StringVar(&service.Address, "address", "", cliMessage("detail.host_address"))
			f.IntVar(&service.Port, "port", 0, cliMessage("detail.target_port"))
			f.StringVar(&service.Protocol, "protocol", "tcp", cliMessage("detail.protocol"))
			if f.Parse(args[2:]) != nil || len(f.Args()) != 1 {
				return usage()
			}
			service.Name = f.Args()[0]
			var token [16]byte
			_, _ = rand.Read(token[:])
			service.Instance = "svc-" + hex.EncodeToString(token[:])
			if capability.ValidateNetworkService(service) != nil {
				return usage()
			}
			snapshot, err := client.ReadConfiguration(ctx)
			if err != nil {
				return result(nil, err)
			}
			var policy capability.PolicyFile
			if err = json.Unmarshal(snapshot.Policy, &policy); err != nil {
				return result(nil, err)
			}
			for _, old := range policy.NetworkServices {
				if old.Name == service.Name {
					_, _ = fmt.Fprintln(diagnostic, cliMessage("network.service_exists"))
					return 1
				}
			}
			policy.NetworkServices = append(policy.NetworkServices, service)
			snapshot.Policy, err = json.Marshal(policy)
			if err != nil {
				return result(nil, err)
			}
			_, err = client.ReplaceConfiguration(ctx, snapshot)
			return result(service, err, "network.host_added")
		}
		if (args[1] == "list" && len(args) == 2) || (args[1] == "remove" && len(args) == 3) {
			snapshot, err := client.ReadConfiguration(ctx)
			if err != nil {
				return result(nil, err)
			}
			var policy capability.PolicyFile
			if err = json.Unmarshal(snapshot.Policy, &policy); err != nil {
				return result(nil, err)
			}
			if args[1] == "list" {
				if len(policy.NetworkServices) == 0 && !jsonOutput {
					return result(nil, nil, "network.hosts_empty")
				}
				return result(policy.NetworkServices, nil)
			}
			index := -1
			for i, s := range policy.NetworkServices {
				if s.Name == args[2] {
					index = i
				}
			}
			if index < 0 {
				return result(nil, core.ErrNotFound)
			}
			policy.NetworkServices = append(policy.NetworkServices[:index], policy.NetworkServices[index+1:]...)
			snapshot.Policy, err = json.Marshal(policy)
			if err != nil {
				return result(nil, err)
			}
			_, err = client.ReplaceConfiguration(ctx, snapshot)
			return result(nil, err, "network.host_removed")
		}
	}
	if len(args) > 0 && args[0] == "rule" {
		f := flag.NewFlagSet("network rule", flag.ContinueOnError)
		configureCLIFlags(f, diagnostic)
		var spec networkrelay.RuleSpec
		var duration, ttl time.Duration
		var decision string
		networkSpecFlags(f, &spec.Connection, &duration)
		f.StringVar(&spec.Connection.Protocol, "protocol", "tcp", cliMessage("detail.protocol"))
		f.StringVar(&spec.Environment, "env", "", cliMessage("detail.rule_env"))
		f.StringVar(&spec.Scope, "scope", "instance", cliMessage("detail.scope"))
		f.StringVar(&decision, "decision", "", cliMessage("detail.decision"))
		f.DurationVar(&ttl, "ttl", time.Hour, cliMessage("detail.ttl"))
		if f.Parse(args[1:]) != nil || len(f.Args()) != 0 || duration < time.Second || duration%time.Second != 0 || ttl <= 0 {
			return usage()
		}
		spec.Connection.DurationSeconds = int(duration / time.Second)
		spec.Decision = core.PolicyDecision(decision)
		if decision == "ask" {
			spec.Decision = core.PolicyRequireApproval
		}
		spec.ExpiresAt = time.Now().UTC().Add(ttl)
		v, e := client.AddNetworkRule(ctx, spec)
		return result(v, e, "network.rule_added")
	}
	return usage()
}
func networkSpecFlags(f *flag.FlagSet, spec *networkrelay.Spec, duration *time.Duration) {
	f.StringVar(&spec.Kind, "kind", "external", cliMessage("detail.kind"))
	f.StringVar(&spec.Target, "target", "", cliMessage("detail.target"))
	f.IntVar(&spec.Port, "port", 0, cliMessage("detail.network_port"))
	f.DurationVar(duration, "duration", 5*time.Minute, cliMessage("detail.duration"))
}
func networkListenCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(diagnostic, "haco:", flagErr)
		return 2
	}
	args = clean
	f := flag.NewFlagSet("network "+args[0], flag.ContinueOnError)
	configureCLIFlags(f, diagnostic)
	spec := networkrelay.Spec{Protocol: args[0]}
	var duration time.Duration
	var listen string
	networkSpecFlags(f, &spec, &duration)
	f.StringVar(&listen, "listen", "127.0.0.1:0", cliMessage("detail.listen"))
	if f.Parse(args[1:]) != nil || len(f.Args()) != 0 || duration < time.Second || duration%time.Second != 0 {
		networkUsage(diagnostic)
		return 2
	}
	spec.DurationSeconds = int(duration / time.Second)
	if networkrelay.ValidateSpec(spec) != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("network.invalid_destination"))
		return 2
	}
	host, _, err := net.SplitHostPort(listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("network.loopback"))
		return 2
	}
	logger, err := logging.NewFromEnv(diagnostic)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("error.logging"))
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	observe := func(session networkrelay.Session, err error) {
		fields := []any{"component", "network", "operation", "connect", "request_id", session.RequestID, "target_host", spec.Target, "target_port", spec.Port}
		if err != nil {
			logger.ErrorContext(ctx, "network connection failed", append(fields, "error", err)...)
		} else {
			logger.InfoContext(ctx, "network connection active", append(fields, "connection_id", session.ID, "expires_at", session.ExpiresAt)...)
		}
	}
	emit := func(address string) error {
		if err := writeCLIResult(out, map[string]any{"listen": address, "protocol": spec.Protocol, "target": spec.Target, "kind": spec.Kind, "duration_seconds": spec.DurationSeconds}, jsonOutput); err != nil {
			return err
		}
		if !jsonOutput {
			_, err := fmt.Fprintln(out, cliMessage("network.listener_ready"))
			return err
		}
		return nil
	}
	if spec.Protocol == "udp" {
		address, e := net.ResolveUDPAddr("udp", listen)
		if e != nil {
			err = e
		} else {
			listener, e := net.ListenUDP("udp", address)
			if e != nil {
				err = e
			} else {
				defer listener.Close()
				if err = emit(listener.LocalAddr().String()); err == nil {
					err = networkrelay.ServeUDP(ctx, listener, spec, networkrelay.OpenGuest, observe)
				}
			}
		}
	} else {
		listener, e := net.Listen("tcp", listen)
		if e != nil {
			err = e
		} else {
			defer listener.Close()
			if err = emit(listener.Addr().String()); err == nil {
				err = networkrelay.ServeTCP(ctx, listener, spec, networkrelay.OpenGuest, observe)
			}
		}
	}
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("network.listener_failed"), err)
		return 1
	}
	return 0
}
