package oci

import "net/http"

// MaintenanceTooling supplies pinned tools for disposable metadata Environments.
// The directory is trusted Host configuration, never an Environment request.
type MaintenanceTooling struct {
	Directory string
	client    *http.Client
}

const maintenanceArchiveURL = "https://github.com/containerd/nerdctl/releases/download/v2.3.5/nerdctl-full-2.3.5-linux-amd64.tar.gz"
const maintenanceArchiveSHA256 = "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa"
const maxMaintenanceArchive = 512 << 20
