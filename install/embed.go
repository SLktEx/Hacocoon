// Package install supplies the same signed Incus package policy to installation
// and trusted-host tooling. It does not execute installers on the controller.
package install

import _ "embed"

//go:embed incus-lts.sh
var IncusLTS string
