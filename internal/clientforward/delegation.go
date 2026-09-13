package clientforward

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

const delegationLimit = 8192

// A prepared selection crosses the client boundary, never a name to resolve
// again. The private controller still authorizes each exact Env incarnation.
type delegation struct {
	Version      int                   `json:"version"`
	Installation reclamation.WSLTarget `json:"installation"`
	Request      prepared              `json:"request"`
	Expires      int64                 `json:"expires_unix_milli"`
}

var forwardName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func (d delegation) validate(now time.Time) error {
	p := d.Request
	remaining := time.UnixMilli(d.Expires).Sub(now)
	if d.Version != 1 || d.Installation.Validate() != nil || !forwardName.MatchString(p.Target.Environment) ||
		!core.ValidEnvironmentInstanceID(p.Target.Instance) || !core.ValidForwardAddress(p.Target.Address, p.Target.Port) ||
		!validListener(p.Listen) || p.Duration < time.Second || p.Duration > time.Hour || remaining <= 0 || remaining > p.Duration ||
		(p.Language != cliui.English && p.Language != cliui.Japanese) {
		return errors.New("invalid or expired Windows tunnel request")
	}
	return nil
}

func writeDelegation(out io.Writer, request delegation) error {
	if err := request.validate(time.Now()); err != nil {
		return err
	}
	b, err := json.Marshal(request)
	if err != nil || len(b) > delegationLimit {
		return errors.New("Windows tunnel request too large")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(b)))
	_, err = io.Copy(out, io.MultiReader(bytes.NewReader(header[:]), bytes.NewReader(b)))
	return err
}

func readDelegation(input io.Reader) (delegation, error) {
	var request delegation
	var header [4]byte
	if _, err := io.ReadFull(input, header[:]); err != nil {
		return request, err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n == 0 || n > delegationLimit {
		return request, errors.New("invalid Windows tunnel request length")
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(input, b); err != nil {
		return request, err
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return request, errors.New("extra Windows tunnel request")
	}
	return request, request.validate(time.Now())
}

// RunDelegated is the companion's private entry. Its input pipe is also the
// parent's lifetime lease: closure cancels the listener and every upstream.
// There are no subsequent commands on this pipe. Additional bytes fail closed.
func RunDelegated(ctx context.Context, input io.ReadCloser, out, diagnostic io.Writer, connect func(reclamation.WSLTarget) (*controlapi.Client, error)) int {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer input.Close()
	stopClose := context.AfterFunc(ctx, func() { input.Close() })
	defer stopClose()
	timer := time.AfterFunc(10*time.Second, func() { input.Close() })
	request, err := readDelegation(input)
	if !timer.Stop() || err != nil || ctx.Err() != nil {
		fmt.Fprintln(diagnostic, cliui.English.Format("forward.delegate_invalid"))
		return 2
	}
	ctx, deadlineCancel := context.WithDeadline(ctx, time.UnixMilli(request.Expires))
	defer deadlineCancel()
	watch := make(chan bool, 1)
	go func() {
		var one [1]byte
		n, err := input.Read(one[:])
		invalid := n != 0 || (err != io.EOF && ctx.Err() == nil)
		cancel()
		watch <- invalid
	}()
	code := runInstalled(ctx, request, out, diagnostic, connect)
	cancel()
	input.Close()
	if <-watch {
		fmt.Fprintln(diagnostic, request.Request.Language.Format("forward.delegate_invalid"))
		return 1
	}
	return code
}

func runInstalled(ctx context.Context, request delegation, out, diagnostic io.Writer, connect func(reclamation.WSLTarget) (*controlapi.Client, error)) int {
	message := request.Request.Language.Format
	if ctx.Err() != nil {
		return 0
	}
	client, err := connect(request.Installation)
	if err == nil {
		var observed reclamation.WSLTarget
		observed, err = client.ReclamationTarget(ctx)
		if err == nil && observed != request.Installation {
			err = errors.New("installation changed")
		}
		if err == nil {
			selected := request.Request.Target
			current, checkErr := client.PrepareEnvironmentForward(ctx, selected.Environment, selected.Address, selected.Port)
			err = checkErr
			if err == nil && current != selected {
				err = errors.New("Environment changed")
			}
		}
	}
	if err != nil {
		if ctx.Err() != nil {
			return 0
		}
		fmt.Fprintln(diagnostic, message("forward.windows_unavailable"))
		return 1
	}
	return runPrepared(ctx, client, request.Request, out, diagnostic)
}
