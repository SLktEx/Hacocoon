package egress

// DefaultProxyPort is the repository contract between the Standard egress
// proxy and runtime-specific transport adapters. It is not a policy grant;
// adapters must still make this listener the only ordinary outbound path.
const DefaultProxyPort = 18080
