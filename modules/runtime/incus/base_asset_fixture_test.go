package incus

import (
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
)

func readyBaseReceipt(base core.BaseRef, scope, source string) core.BaseAsset {
	project, pool, _ := strings.Cut(scope, "/")
	owner := strings.Repeat("c", 32)
	binding, _ := json.Marshal(baseAssetBinding{Version: 1, Project: project, Pool: pool, Source: source})
	return core.BaseAsset{ID: "base-" + owner, Owner: owner, Base: base, Provider: environmentapp.ProviderIncus, Scope: scope, NativeRef: "instance/haco-base-" + owner, Binding: string(binding), State: "ready"}
}
