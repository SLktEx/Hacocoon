package main

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

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/modules/standard/networkrelay"
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
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco: cannot open controller client")
		return 1
	}
	return networkCommand(ctx, client, args, os.Stdout, os.Stderr)
}
func networkUsage(out io.Writer) {
	fmt.Fprintln(out, "Usage: haco network tcp|udp --target name --port port [--kind external|host|environment] [--listen 127.0.0.1:0] [--duration 5m]")
	fmt.Fprintln(out, "       haco network list | revoke <connection-id>")
	fmt.Fprintln(out, "       haco network host add --address IP --port port [--protocol tcp|udp] <name>")
	fmt.Fprintln(out, "       haco network host list | remove <name>")
	fmt.Fprintln(out, "       haco network rule --env name --target name --port port --decision allow|ask|deny [--kind external|host|environment] [--protocol tcp|udp] [--duration 5m] [--ttl 1h] [--scope instance|environment|global]")
}
func networkCommand(ctx context.Context, client networkClient, args []string, out, diagnostic io.Writer) int {
	usage := func() int { networkUsage(diagnostic); return 2 }
	result := func(value any, err error) int {
		if err != nil {
			fmt.Fprintln(diagnostic, "haco:", err)
			return 1
		}
		if value != nil {
			if json.NewEncoder(out).Encode(value) != nil {
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
			return result(v, e)
		}
	}
	if len(args) == 2 && args[0] == "revoke" {
		return result(nil, client.RevokeNetworkConnection(ctx, args[1]))
	}
	if len(args) > 1 && args[0] == "host" {
		if args[1] == "add" {
			f := flag.NewFlagSet("network host add", flag.ContinueOnError)
			f.SetOutput(diagnostic)
			var service capability.NetworkService
			f.StringVar(&service.Address, "address", "", "explicit numeric Host/Windows service address")
			f.IntVar(&service.Port, "port", 0, "destination port")
			f.StringVar(&service.Protocol, "protocol", "tcp", "tcp or udp")
			if f.Parse(args[2:]) != nil || len(f.Args()) != 1 {
				return usage()
			}
			service.Name = f.Args()[0]
			var token [16]byte
			if _, err := rand.Read(token[:]); err != nil {
				return result(nil, err)
			}
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
					return result(nil, fmt.Errorf("service already exists; remove it before registering a replacement"))
				}
			}
			policy.NetworkServices = append(policy.NetworkServices, service)
			snapshot.Policy, err = json.Marshal(policy)
			if err != nil {
				return result(nil, err)
			}
			_, err = client.ReplaceConfiguration(ctx, snapshot)
			return result(service, err)
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
			return result(nil, err)
		}
	}
	if len(args) > 0 && args[0] == "rule" {
		f := flag.NewFlagSet("network rule", flag.ContinueOnError)
		f.SetOutput(diagnostic)
		var spec networkrelay.RuleSpec
		var duration, ttl time.Duration
		var decision string
		networkSpecFlags(f, &spec.Connection, &duration)
		f.StringVar(&spec.Connection.Protocol, "protocol", "tcp", "tcp or udp")
		f.StringVar(&spec.Environment, "env", "", "source Environment")
		f.StringVar(&spec.Scope, "scope", "instance", "instance, environment, or global source scope")
		f.StringVar(&decision, "decision", "", "allow, ask, or deny")
		f.DurationVar(&ttl, "ttl", time.Hour, "rule validity, at most 31 days")
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
		return result(v, e)
	}
	return usage()
}
func networkSpecFlags(f *flag.FlagSet, spec *networkrelay.Spec, duration *time.Duration) {
	f.StringVar(&spec.Kind, "kind", "external", "external, host, or environment")
	f.StringVar(&spec.Target, "target", "", "destination name or external IP")
	f.IntVar(&spec.Port, "port", 0, "destination port (optional for registered Host services)")
	f.DurationVar(duration, "duration", 5*time.Minute, "connection/listener maximum lifetime (UDP at most 5m)")
}
func networkListenCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("network "+args[0], flag.ContinueOnError)
	f.SetOutput(diagnostic)
	spec := networkrelay.Spec{Protocol: args[0]}
	var duration time.Duration
	var listen string
	networkSpecFlags(f, &spec, &duration)
	f.StringVar(&listen, "listen", "127.0.0.1:0", "local loopback endpoint")
	if f.Parse(args[1:]) != nil || len(f.Args()) != 0 || duration < time.Second || duration%time.Second != 0 {
		networkUsage(diagnostic)
		return 2
	}
	spec.DurationSeconds = int(duration / time.Second)
	if networkrelay.ValidateSpec(spec) != nil {
		fmt.Fprintln(diagnostic, "haco: invalid destination or lifetime")
		return 2
	}
	host, _, err := net.SplitHostPort(listen)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() {
		fmt.Fprintln(diagnostic, "haco: listener must use a numeric loopback address")
		return 2
	}
	logger, err := logging.NewFromEnv(diagnostic)
	if err != nil {
		fmt.Fprintln(diagnostic, "haco: invalid logging configuration")
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
		return json.NewEncoder(out).Encode(map[string]any{"listen": address, "protocol": spec.Protocol, "target": spec.Target, "kind": spec.Kind, "duration_seconds": spec.DurationSeconds})
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
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	return 0
}
