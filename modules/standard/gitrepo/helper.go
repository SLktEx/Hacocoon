package gitrepo

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Exchange func(context.Context, Request) (Response, error)

func UnixExchange(socket string) Exchange {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}, DisableKeepAlives: true}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Minute}
	return func(ctx context.Context, request Request) (Response, error) {
		payload, err := json.Marshal(request)
		if err != nil || len(payload) > MaxMessage {
			return Response{}, fmt.Errorf("Git request exceeds PoC limit")
		}
		req, err := http.NewRequestWithContext(ctx, "POST", "http://haco/git", bytes.NewReader(payload))
		if err != nil {
			return Response{}, err
		}
		response, err := client.Do(req)
		if err != nil {
			return Response{}, fmt.Errorf("Environment Git broker is unavailable")
		}
		defer response.Body.Close()
		var result Response
		decoder := json.NewDecoder(io.LimitReader(response.Body, MaxMessage+1))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result); err != nil || len(result.Pack) > MaxPack {
			return Response{}, fmt.Errorf("invalid Git broker response")
		}
		if response.StatusCode != 200 || result.Error != "" {
			return Response{}, fmt.Errorf("Git broker refused: %s", result.Error)
		}
		return result, nil
	}
}

func Helper(ctx context.Context, args []string, input io.Reader, output, diagnostic io.Writer, exchange Exchange) error {
	if len(args) != 2 || !strings.HasPrefix(args[1], "haco://") {
		return fmt.Errorf("Git remote must use haco://<registered-repository>")
	}
	repo := strings.TrimPrefix(args[1], "haco://")
	if !ValidID(repo) {
		return fmt.Errorf("invalid repository identity")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 4096), 65536)
	var listed Response
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			return nil
		case line == "capabilities":
			fmt.Fprint(output, "fetch\npush\noption\n\n")
		case strings.HasPrefix(line, "option "):
			fields := strings.Fields(line)
			if len(fields) == 3 && (fields[1] == "verbosity" || fields[1] == "progress") {
				fmt.Fprintln(output, "ok")
			} else {
				fmt.Fprintln(output, "unsupported")
			}
		case line == "list" || line == "list for-push":
			var err error
			listed, err = exchange(ctx, Request{Operation: "list", Repository: repo})
			if err != nil {
				return err
			}
			if !ValidOID(listed.OID) || !strings.HasPrefix(listed.Ref, "refs/heads/") || !ValidBranch(strings.TrimPrefix(listed.Ref, "refs/heads/")) {
				return fmt.Errorf("invalid remote ref listing")
			}
			fmt.Fprintf(output, "%s %s\n@%s HEAD\n\n", listed.OID, listed.Ref, listed.Ref)
		case strings.HasPrefix(line, "fetch "):
			batch, err := helperBatch(scanner, line)
			if err != nil {
				return err
			}
			if len(batch) != 1 || batch[0] != "fetch "+listed.OID+" "+listed.Ref {
				return fmt.Errorf("only the listed branch may be fetched")
			}
			response, err := exchange(ctx, Request{Operation: "fetch", Repository: repo, Ref: listed.Ref, NewOID: listed.OID})
			if err != nil {
				return err
			}
			if _, err := helperGit(ctx, response.Pack, "index-pack", "--stdin", "--strict"); err != nil {
				return err
			}
			fmt.Fprintln(output)
		case strings.HasPrefix(line, "push "):
			batch, err := helperBatch(scanner, line)
			if err != nil {
				return err
			}
			if len(batch) != 1 {
				return fmt.Errorf("multiple-ref pushes are unsupported")
			}
			refspec := strings.TrimPrefix(line, "push ")
			parts := strings.Split(refspec, ":")
			if len(parts) != 2 || parts[0] == "" || strings.HasPrefix(parts[0], "+") || strings.HasPrefix(parts[0], "-") || parts[1] != listed.Ref || !ValidOID(listed.OID) {
				return fmt.Errorf("only a normal push to the registered branch is supported")
			}
			value, err := helperGit(ctx, nil, "rev-parse", "--verify", "--end-of-options", parts[0]+"^{commit}")
			if err != nil {
				return err
			}
			oid := strings.TrimSpace(string(value))
			if !ValidOID(oid) {
				return fmt.Errorf("invalid local commit")
			}
			pack, err := helperGit(ctx, []byte(oid+"\n"), "pack-objects", "--stdout", "--revs")
			if err != nil {
				return err
			}
			fmt.Fprintln(diagnostic, "Push awaits trusted Host Policy/approval. In another Host terminal, run: haco git pending")
			_, err = exchange(ctx, Request{Operation: "push", Repository: repo, Ref: listed.Ref, OldOID: listed.OID, NewOID: oid, Pack: pack})
			if err != nil {
				fmt.Fprintf(diagnostic, "%s\n", err)
				fmt.Fprintf(output, "error %s broker-failed\n\n", listed.Ref)
			} else {
				fmt.Fprintf(output, "ok %s\n\n", listed.Ref)
			}
		default:
			return fmt.Errorf("unsupported Git helper command")
		}
	}
	return scanner.Err()
}

func helperBatch(scanner *bufio.Scanner, first string) ([]string, error) {
	batch := []string{first}
	for scanner.Scan() {
		if scanner.Text() == "" {
			return batch, nil
		}
		batch = append(batch, scanner.Text())
		if len(batch) > 16 {
			return nil, fmt.Errorf("Git batch exceeds PoC limit")
		}
	}
	return nil, fmt.Errorf("incomplete Git helper batch")
}

func helperGit(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)...)
	cmd.Env = os.Environ() // Entirely inside the untrusted Environment.
	cmd.Stdin = bytes.NewReader(input)
	var out cappedBuffer
	out.limit = MaxPack
	cmd.Stdout = &out
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("local Git %s failed", args[0])
	}
	return out.Bytes(), nil
}
