package incus

// Incus fills omitted copy configuration from the source. Clear inherited
// authority explicitly, but use a valid false value for its non-optional ready
// boolean (Incus 6.0.5 internal/instance/config.go). An empty string is rejected
// when capturing a just-stopped instance. Callers retain only required idmaps
// and then apply their current ownership/security configuration.
func clearedSnapshotInstanceConfig(sources ...map[string]string) map[string]string {
	config := map[string]string{}
	for _, source := range sources {
		for key := range source {
			config[key] = ""
		}
	}
	if _, present := config["volatile.last_state.ready"]; present {
		config["volatile.last_state.ready"] = "false"
	}
	return config
}
