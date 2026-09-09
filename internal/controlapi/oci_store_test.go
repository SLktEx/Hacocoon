package controlapi

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"testing"
)

func TestOCIFromCannotSelectForeignKindsOrOtherOperations(t *testing.T) {
	// Nil dependencies ensure malformed requests are rejected before service use.
	socket := doctorTestSocket(t, func(server *control.Server) {
		if err := RegisterOCIStores(server, &persistentresource.Service{}); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(socket)
	for _, req := range []OCIStoreRequest{
		{Operation: "create", ID: "oci:dev", From: "workspace:source"},
		{Operation: "create", ID: "oci:dev", From: "oci:../source"},
		{Operation: "delete", ID: "oci:dev", From: "oci:source"},
		{Operation: "inspect", ID: "oci:dev", From: "oci:source"},
		{Operation: "list", From: "oci:source"},
		{Operation: "delete", ID: "oci:dev"},
		{Operation: "delete", ID: "oci:dev", Owner: "invalid"},
		{Operation: "list", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	} {
		_, err := client.OCIStore(context.Background(), req)
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatalf("request=%+v err=%v", req, err)
		}
	}
}
