package aws

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"net/http"
	"time"
)

const GuestEndpoint = "http://169.254.254.1:18080" + GuestPath

type GuestClient struct {
	client   *http.Client
	endpoint string
}

func NewGuestClient() *GuestClient {
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, MaxResponseHeaderBytes: 16 << 10, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext}
	return &GuestClient{client: &http.Client{Transport: transport, Timeout: 15 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, endpoint: GuestEndpoint}
}
func (*GuestClient) GuestSource() bool { return true }
func (*GuestClient) ListEnvironments(context.Context) ([]core.Environment, error) {
	return nil, core.ErrUnsupported
}
func (c *GuestClient) ListS3(ctx context.Context, s ListSpec) (core.CapabilityResult, error) {
	return c.perform(ctx, "list", s, nil)
}
func (c *GuestClient) DownloadS3(ctx context.Context, s GetSpec, sink io.Writer) (core.CapabilityResult, error) {
	if sink == nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	return c.perform(ctx, "get", ListSpec(s), sink)
}
func (c *GuestClient) perform(ctx context.Context, operation string, s ListSpec, sink io.Writer) (core.CapabilityResult, error) {
	if s.Environment != "" {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	check := s
	check.Environment = "source-bound"
	if _, _, key, err := parse(check); err != nil || operation == "get" && !validObjectKey(key) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	input, _ := json.Marshal(GuestRequest{Operation: operation, URL: s.URL, Profile: s.Profile, Region: s.Region})
	request, err := http.NewRequestWithContext(ctx, "POST", c.endpoint, bytes.NewReader(input))
	if err != nil {
		return core.CapabilityResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.client.Do(request)
	if err != nil {
		return core.CapabilityResult{}, fmt.Errorf("guest AWS transport unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 || response.Header.Get("Content-Type") != "application/x-ndjson" {
		return core.CapabilityResult{}, fmt.Errorf("guest AWS request refused (HTTP %d)", response.StatusCode)
	}
	scan := bufio.NewScanner(response.Body)
	// Listings are bounded separately; downloads never buffer the whole object.
	limit := 128 << 10
	if operation == "list" {
		limit = 16 << 20
	}
	scan.Buffer(make([]byte, 4096), limit)
	hash := sha256.New()
	var count int64
	var result *core.CapabilityResult
	for scan.Scan() {
		var frame GuestFrame
		decoder := json.NewDecoder(bytes.NewReader(scan.Bytes()))
		decoder.DisallowUnknownFields()
		if result != nil || decoder.Decode(&frame) != nil || decoder.Decode(new(any)) != io.EOF {
			return core.CapabilityResult{}, core.ErrIncompatibleState
		}
		if frame.Result != nil {
			if len(frame.Data) != 0 {
				return core.CapabilityResult{}, core.ErrIncompatibleState
			}
			if frame.Error != "" {
				return *frame.Result, fmt.Errorf("AWS operation did not succeed")
			}
			result = frame.Result
			continue
		}
		if frame.Error != "" || operation != "get" || len(frame.Data) == 0 || len(frame.Data) > 64<<10 {
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
	if scan.Err() != nil || result == nil {
		return core.CapabilityResult{}, fmt.Errorf("guest AWS response incomplete")
	}
	if operation == "get" {
		if err := VerifyDownload(*result, count, hex.EncodeToString(hash.Sum(nil))); err != nil {
			return *result, err
		}
	} else if result.ExecutionState != core.CapabilitySucceeded || !result.AuditComplete || result.Provider != Capability || result.RequestID == "" || !json.Valid([]byte(result.Output)) {
		return *result, core.ErrIncompatibleState
	}
	return *result, nil
}
