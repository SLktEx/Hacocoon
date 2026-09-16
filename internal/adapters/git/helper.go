package gitadapter

import (
	"bufio"
	"bytes"
	"context"
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
		body, err := RequestBody(request)
		if err != nil {
			return Response{}, err
		}
		req, err := http.NewRequestWithContext(ctx, "POST", "http://haco/git", body)
		if err != nil {
			return Response{}, err
		}
		response, err := client.Do(req)
		if err != nil {
			return Response{}, fmt.Errorf("Environment Git broker is unavailable")
		}
		defer response.Body.Close()
		result, err := ReadResponse(response.Body, request.PackOutput)
		if err != nil {
			return Response{}, err
		}
		if response.StatusCode != 200 {
			return Response{}, fmt.Errorf("git broker refused operation")
		}
		return result, nil
	}
}

func Helper(ctx context.Context, args []string, input io.Reader, output, diagnostic io.Writer, exchange Exchange) error {
	if len(args) != 2 || !strings.HasPrefix(args[1], "haco://") {
		return fmt.Errorf("git remote must use haco://<registered-repository>")
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
			heads, err := ValidateHeads(listed.Heads)
			if err != nil || (listed.Ref == "" && listed.OID != "") || (listed.Ref != "" && (!ValidOID(listed.OID) || heads[listed.Ref] != listed.OID)) {
				return fmt.Errorf("invalid remote ref listing")
			}
			for _, head := range listed.Heads {
				_, _ = fmt.Fprintf(output, "%s %s\n", head.OID, head.Ref)
			}
			if listed.Ref != "" {
				_, _ = fmt.Fprintf(output, "@%s HEAD\n", listed.Ref)
			}
			_, _ = fmt.Fprintln(output)
		case strings.HasPrefix(line, "fetch "):
			batch, err := helperBatch(scanner, line)
			if err != nil {
				return err
			}
			listedHeads, err := ValidateHeads(listed.Heads)
			if err != nil {
				return err
			}
			var requested []Head
			for _, line := range batch {
				fields := strings.SplitN(line, " ", 3)
				if len(fields) != 3 || fields[0] != "fetch" || !ValidOID(fields[1]) || listedHeads[fields[2]] != fields[1] {
					return fmt.Errorf("only listed branch commits may be fetched")
				}
				requested = append(requested, Head{Ref: fields[2], OID: fields[1]})
			}
			if _, err := ValidateHeads(requested); err != nil {
				return err
			}
			for _, head := range requested {
				haves, err := helperHaves(ctx)
				if err != nil {
					return err
				}
				if err := helperFetch(ctx, exchange, Request{Operation: "fetch", Repository: repo, Heads: []Head{head}, Haves: haves}); err != nil {
					return err
				}
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
			listedHeads, err := ValidateHeads(listed.Heads)
			if err != nil || len(parts) != 2 || parts[0] == "" || strings.HasPrefix(parts[0], "+") || strings.HasPrefix(parts[0], "-") || !ValidHeadRef(parts[1]) {
				return fmt.Errorf("only a normal single-head creation or fast-forward push is supported")
			}
			oldOID := listedHeads[parts[1]]
			if oldOID == "" {
				oldOID = ZeroOID
			}
			value, err := helperGit(ctx, nil, "rev-parse", "--verify", "--end-of-options", parts[0]+"^{commit}")
			if err != nil {
				return err
			}
			oid := strings.TrimSpace(string(value))
			if !ValidOID(oid) || oid == ZeroOID {
				return fmt.Errorf("invalid local commit")
			}
			basis := oldOID
			if oldOID == ZeroOID {
				basis, err = helperNewBranchBasis(ctx, repo, oid, listed, exchange)
				if err != nil {
					return err
				}
			}
			fmt.Fprintln(diagnostic, "Push awaits trusted Host Policy/approval. In another Host terminal, run: haco git pending")
			err = helperPush(ctx, exchange, Request{Operation: "push", Repository: repo, Ref: parts[1], OldOID: oldOID, NewOID: oid}, basis)
			if err != nil {
				fmt.Fprintf(diagnostic, "%s\n", err)
				_, _ = fmt.Fprintf(output, "error %s broker-failed\n\n", parts[1])
			} else {
				_, _ = fmt.Fprintf(output, "ok %s\n\n", parts[1])
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
		if len(batch) > MaxHeads {
			return nil, fmt.Errorf("git batch exceeds PoC limit")
		}
	}
	return nil, fmt.Errorf("incomplete Git helper batch")
}

func helperCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-c", "core.hooksPath=/dev/null", "-c", "gc.auto=0", "-c", "maintenance.auto=false"}, args...)...)
	cmd.Env = os.Environ() // Entirely inside the untrusted Environment.
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	return cmd
}
func helperGit(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	var out cappedBuffer
	out.limit = maxCommandOutput
	_, err := runPack(helperCommand(ctx, args...), bytes.NewReader(input), &out)
	if err != nil {
		return nil, fmt.Errorf("local Git %s failed", args[0])
	}
	return out.Bytes(), nil
}

func helperFetch(ctx context.Context, exchange Exchange, req Request) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	completed := make(chan error, 1)
	go func() {
		_, err := runPack(helperCommand(ctx, "index-pack", "--stdin", "--strict", fmt.Sprintf("--max-input-size=%d", maxTransferBytes)), reader, io.Discard)
		_ = reader.CloseWithError(err)
		completed <- err
	}()
	req.PackOutput = writer
	response, err := exchange(ctx, req)
	_ = writer.CloseWithError(err)
	if err != nil {
		cancel()
	}
	indexed := <-completed
	if err != nil {
		return err
	}
	if indexed != nil {
		return indexed
	}
	if response.PackBytes <= 0 || response.PackBytes > maxTransferBytes || response.Ref != req.Heads[0].Ref || response.OID != req.Heads[0].OID {
		return fmt.Errorf("invalid Git pack receipt")
	}
	return nil
}

func helperPush(ctx context.Context, exchange Exchange, req Request, basis string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	completed := make(chan error, 1)
	go func() {
		err := helperPushPack(ctx, req.NewOID, basis, writer)
		_ = writer.CloseWithError(err)
		completed <- err
	}()
	req.Pack = reader
	_, err := exchange(ctx, req)
	_ = reader.CloseWithError(err)
	cancel()
	produced := <-completed
	if err != nil {
		return err
	}
	return produced
}
