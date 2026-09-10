//go:build linux

package environment

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"reflect"
	"testing"
)

type recordingExportProvider struct {
	recordingSnapshotProvider
	root  string
	limit int64
}

var exportRouteSentinel = errors.New("native export sentinel")

func (p *recordingExportProvider) ExportSnapshotComponent(_ context.Context, c core.SnapshotComponent, root string, limit int64) (environmenttransfer.Archive, error) {
	p.component = c
	p.root = root
	p.limit = limit
	p.calls = append(p.calls, "export")
	return nil, exportRouteSentinel
}
func TestSnapshotExportRoutesCanonicalRefBeforeNativeCall(t *testing.T) {
	chosen, other := &recordingExportProvider{}, &recordingExportProvider{}
	router, err := NewRouter("other", Register(testProvider, chosen), Register("other", other))
	if err != nil {
		t.Fatal(err)
	}
	local := core.SnapshotComponent{Role: "rootfs", NativeRef: "saved-native", Owner: "exact-owner", Binding: "opaque-binding", State: "verified"}
	routed := local
	routed.NativeRef = encodeRouteRef(testProvider, local.NativeRef)
	archive, err := router.ExportSnapshotComponent(context.Background(), routed, "/private/staging", 123)
	if archive != nil || !errors.Is(err, exportRouteSentinel) || !reflect.DeepEqual(chosen.component, local) || chosen.root != "/private/staging" || chosen.limit != 123 || len(other.calls) != 0 {
		t.Fatal(archive, err, chosen, other)
	}
	for _, ref := range []string{"saved-native", encodeRouteRef("missing", "saved-native"), routed.NativeRef + "="} {
		invalid := routed
		invalid.NativeRef = ref
		if _, err := router.ExportSnapshotComponent(context.Background(), invalid, "/private/staging", 123); err == nil {
			t.Fatal("unvalidated route accepted", ref)
		}
	}
	if len(chosen.calls) != 1 || len(other.calls) != 0 {
		t.Fatal("bad route reached native adapter", chosen.calls, other.calls)
	}
}
