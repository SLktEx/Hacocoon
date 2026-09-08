package aws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/url"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const GetAction = "GetObject"

type GetSpec ListSpec
type HostStream func(context.Context, string, []byte, io.Writer) error
type downloadSinkKey struct{}
type DownloadReceipt struct {
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type downloadFrame struct {
	Data     []byte           `json:"data,omitempty"`
	Identity *identity        `json:"identity,omitempty"`
	Receipt  *DownloadReceipt `json:"receipt,omitempty"`
	Error    string           `json:"error,omitempty"`
}

func getRequest(s ListSpec, bucket, key string, i identity) core.CapabilityRequest {
	r := request(s, bucket, key, i)
	r.Action = GetAction
	r.Resource += "/" + key
	delete(r.Attributes, "prefix")
	r.Attributes["key"] = key
	r.Attributes["iam_action"] = "s3:GetObject"
	r.Attributes["description"] = "Download current object at execution"
	return r
}
func (b *Broker) Download(ctx context.Context, s GetSpec, sink io.Writer) (core.CapabilityResult, error) {
	if sink == nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	r, err := b.prepare(ctx, ListSpec(s), true)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	return b.Capabilities.Request(context.WithValue(ctx, downloadSinkKey{}, sink), r)
}
func (p *Provider) download(ctx context.Context, r core.CapabilityRequest) (core.CapabilityResult, error) {
	sink, ok := ctx.Value(downloadSinkKey{}).(io.Writer)
	if !ok || sink == nil || p.Stream == nil {
		return core.CapabilityResult{}, core.ErrUnsupported
	}
	a := r.Attributes
	i := identity{Account: a["account"], Principal: a["principal"], Region: a["region"], AccountName: rawAccountName(a["account_name"])}
	s := ListSpec{Environment: r.Environment, Profile: a["profile"], Region: i.Region, URL: (&url.URL{Scheme: "s3", Host: a["bucket"], Path: "/" + a["key"]}).String()}
	_, bucket, key, err := parse(s)
	if err != nil || !validObjectKey(key) || !validIdentity(i) || len(r.Parameters) != 0 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	want := getRequest(s, bucket, key, i)
	if r.Capability != Capability || r.Action != GetAction || r.Resource != want.Resource || !maps.Equal(a, want.Attributes) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	input, _ := json.Marshal(agentRequest{Mode: "get", Profile: s.Profile, Region: i.Region, Account: i.Account, AccountName: i.AccountName, Principal: i.Principal, Bucket: bucket, Key: key})
	reader, writer := io.Pipe()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	complete := make(chan error, 1)
	go func() { err := p.Stream(runCtx, HostAgent, input, writer); writer.CloseWithError(err); complete <- err }()
	defer reader.Close()
	hash := sha256.New()
	var count int64
	var receipt *DownloadReceipt
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 128<<10)
	for scanner.Scan() {
		var frame downloadFrame
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&frame) != nil || decoder.Decode(new(any)) != io.EOF || receipt != nil {
			return core.CapabilityResult{}, core.ErrIncompatibleState
		}
		if frame.Error != "" {
			if len(frame.Data) != 0 || frame.Identity != nil || frame.Receipt != nil {
				return core.CapabilityResult{}, core.ErrIncompatibleState
			}
			_, err := call(ctx, func(context.Context, string, []byte) ([]byte, error) { return scanner.Bytes(), nil }, agentRequest{})
			if err == nil {
				err = core.ErrIncompatibleState
			}
			return core.CapabilityResult{}, err
		}
		if frame.Receipt != nil {
			if len(frame.Data) != 0 || frame.Identity == nil || *frame.Identity != i || frame.Receipt.Bytes != count || frame.Receipt.SHA256 != hex.EncodeToString(hash.Sum(nil)) {
				return core.CapabilityResult{}, core.ErrIncompatibleState
			}
			receipt = frame.Receipt
			continue
		}
		if frame.Identity != nil || len(frame.Data) == 0 || len(frame.Data) > 64<<10 {
			return core.CapabilityResult{}, core.ErrIncompatibleState
		}
		n, err := sink.Write(frame.Data)
		if err != nil {
			return core.CapabilityResult{}, err
		}
		if n != len(frame.Data) {
			return core.CapabilityResult{}, io.ErrShortWrite
		}
		hash.Write(frame.Data)
		count += int64(n)
	}
	if err := scanner.Err(); err != nil {
		return core.CapabilityResult{}, fmt.Errorf("AWS download stream incomplete: %w", core.ErrRuntimeUnavailable)
	}
	if err := <-complete; err != nil || receipt == nil {
		return core.CapabilityResult{}, fmt.Errorf("AWS download completion missing: %w", core.ErrRuntimeUnavailable)
	}
	encoded, _ := json.Marshal(receipt)
	return core.CapabilityResult{Provider: Capability, Output: string(encoded)}, nil
}

func validObjectKey(key string) bool {
	if key == "" {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

// VerifyDownload must pass before a client publishes its temporary file.
func VerifyDownload(result core.CapabilityResult, count int64, digest string) error {
	if result.ExecutionState != core.CapabilitySucceeded || !result.AuditComplete || result.Provider != Capability || result.RequestID == "" {
		return core.ErrIncompatibleState
	}
	var receipt DownloadReceipt
	decoder := json.NewDecoder(bytes.NewBufferString(result.Output))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || receipt.Bytes != count || receipt.SHA256 != digest {
		return core.ErrIncompatibleState
	}
	return nil
}
