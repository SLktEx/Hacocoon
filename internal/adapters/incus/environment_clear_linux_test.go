package incus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestEmptyNativeDataEnumeratesThenDeletesWithoutFollowingLinks(t *testing.T) {
	var deleted []string
	do := func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/volume/files" || req.URL.Query().Get("project") != "default" {
			t.Fatal(req.URL)
		}
		target := req.URL.Query().Get("path")
		kind := "file"
		body := ""
		status := 200
		switch req.Method {
		case "GET":
			kind = "directory"
			names := []string{}
			switch target {
			case "/":
				if len(deleted) == 0 {
					names = []string{"nested", "outside-link"}
				}
			case "/nested":
				names = []string{"data"}
			default:
				t.Fatal("followed link", target)
			}
			data, err := json.Marshal(map[string]any{"type": "sync", "status_code": 200, "metadata": names})
			if err != nil {
				t.Fatal(err)
			}
			body = string(data)
		case "HEAD":
			if len(deleted) != 0 {
				t.Fatal("deleted before full enumeration")
			}
			if target == "/nested" {
				kind = "directory"
			}
			if target == "/outside-link" {
				kind = "symlink"
			}
		case "DELETE":
			if target == "/" {
				t.Fatal("deleted volume root")
			}
			deleted = append(deleted, target)
		default:
			t.Fatal(req.Method)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"X-Incus-Type": {kind}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	}
	if err := emptyNativeDataVolume(context.Background(), "http://unix.socket", "/volume/files", "default", do); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(deleted, []string{"/nested/data", "/nested", "/outside-link"}) {
		t.Fatal(deleted)
	}
}

func TestEmptyNativeDataRejectsUntrustedListingBeforeAnyDelete(t *testing.T) {
	for _, names := range [][]string{{".."}, {"/outside"}, {"a/b"}, {"a\\b"}, {"same", "same"}, {"bad\nname"}} {
		t.Run(strings.Join(names, "-"), func(t *testing.T) {
			do := func(req *http.Request) (*http.Response, error) {
				if req.Method != "GET" {
					t.Fatal("acted on untrusted listing", req.Method)
				}
				data, err := json.Marshal(map[string]any{"type": "sync", "status_code": 200, "metadata": names})
				if err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"X-Incus-Type": {"directory"}}, Body: io.NopCloser(strings.NewReader(string(data)))}, nil
			}
			if err := emptyNativeDataVolume(context.Background(), "http://unix.socket", "/volume/files", "default", do); !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal(err)
			}
		})
	}
}

func TestEmptyNativeDataRequiresPositiveEmptyResult(t *testing.T) {
	for _, mode := range []string{"delete-fails", "not-empty"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			do := func(req *http.Request) (*http.Response, error) {
				status := 200
				kind := "file"
				body := ""
				if req.Method == "GET" {
					kind = "directory"
					body = `{"type":"sync","status_code":200,"metadata":["file"]}`
				}
				if req.Method == "DELETE" {
					calls++
					if mode == "delete-fails" {
						status = 500
					}
				}
				return &http.Response{StatusCode: status, Header: http.Header{"X-Incus-Type": {kind}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			}
			if err := emptyNativeDataVolume(context.Background(), "http://unix.socket", "/volume/files", "default", do); err == nil || calls != 1 {
				t.Fatal("unverified success", err, calls)
			}
		})
	}
}
