package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"testing"
)

func TestSourceManageRejectsMissingOwnerAndExtraAuthority(t *testing.T) {
	handler := repositoryManageHandler(nil)
	for _, payload := range []string{`{"operation":"delete","id":"source"}`, `{"operation":"delete","id":"../source","owner":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, `{"operation":"list","owner":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, `{"operation":"delete","id":"source","owner":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","path":"/"}`, `{"operation":"list"} {}`, `{"operation":"create"}`} {
		_, err := handler(context.Background(), json.RawMessage(payload))
		if !errors.Is(err, control.ErrInvalidArgument) {
			t.Fatalf("%s: %v", payload, err)
		}
	}
}
