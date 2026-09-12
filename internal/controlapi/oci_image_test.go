package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"testing"
)

func TestOCIImageRejectsExtraAuthorityBeforeService(t *testing.T) {
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterOCIImages(server, nil); err != nil {
			t.Fatal(err)
		}
	})
	c, _ := NewClient(socket)
	for _, raw := range []string{
		`{"operation":"list","host":true,"environment":"dev","runtime":"docker"}`,
		`{"operation":"delete","host":true,"id":"sha256:bad"}`,
		`{"operation":"delete","id":"sha256:bad"}`,
		`{"operation":"list","environment":"dev","runtime":"docker","executable":"sh"}`,
		`{"operation":"list","environment":"dev","runtime":"arbitrary"}`,
		`{"operation":"list","environment":"dev","runtime":"docker","target":{"environment":"other"}}`,
		`{"operation":"delete","environment":"dev","runtime":"docker"}`,
	} {
		var result any
		err := c.wire.Call(context.Background(), MethodOCIImage, json.RawMessage(raw), &result)
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatalf("request %s: %v", raw, err)
		}
	}
}
