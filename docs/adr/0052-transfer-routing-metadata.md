# ADR 0052: Transfer Workspace routing as data, not authority

Status: accepted for the version-2 Environment export envelope.

## Context

The first envelope preserves native archives but omits the repository mapping
needed by managed Workspace registration. Reading guest `.git/config` as trusted
broker routing would cross the existing Host/Env boundary. Requiring users to
re-enter every repository at export would make ordinary migration harder.

## Decision

New public exports obtain ordered Workspace name/remote/branch metadata from the
same protected snapshot bindings used for native archive export. The Incus adapter
understands those bindings; the transfer package validates and serializes only
portable routing fields. Existing lifecycle read reservations protect both steps.
No guest command, Host file read, network request or credential lookup is needed.

Envelope version 2 requires one unique named mapping per Workspace archive.
Credential-bearing or unsupported remote strings, invalid names/branches and
ambiguous mappings are rejected before native export. Bounds and full envelope
verification still apply. Missing routing in legacy saved bindings stays explicitly
empty; a mutable guest configuration never fills the gap.

Routing is untrusted input at the destination, not approval or authority. A future
importer must create fresh managed identities and continue using ordinary Git
approval/credential mediation. In particular, a source `file:` remote must never
be reinterpreted as permission to access a destination Host path. Explicit local
rebinding or an offline Workspace is required before such routing can be used.

Version-1 bundle inspection and component reads remain supported without inventing
missing metadata. Version-2 exports require a reader that supports version 2; older
readers reject them explicitly. No existing bundle or catalog is rewritten. Public
import and its offline/rebinding behavior are still to be implemented.

## Rejected alternatives

Do not export old grants, owners, management IDs, device paths or credentials. Do
not silently drop metadata to satisfy an older reader or auto-register source
local paths. Do not add an extra CLI argument per repository or a separate catalog.
