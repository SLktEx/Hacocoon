package environment

// DecodePersistedRuntimeRef resolves the provider-routing envelope stored in an
// Environment RuntimeRef back to the provider identity and provider-local ref.
//
// Pre-v0.7 unwrapped refs retain the router's existing compatibility behavior
// and are interpreted as Incus-backed refs. Callers must still verify that the
// returned provider is the authority they expect before trusting the local ref.
func DecodePersistedRuntimeRef(raw string) (provider, ref string, err error) {
	return decodeRouteRef(raw)
}
