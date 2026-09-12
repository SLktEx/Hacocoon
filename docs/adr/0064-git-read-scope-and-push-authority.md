# ADR 0064: Separate all-branch Git reads from push authority

Status: accepted for implementation  
Date: 2026-09-13

## Context

The Standard remote helper currently binds both reads and writes to the branch
chosen when registering a repository. Developers need other branches without
granting external write authority. Clone provenance is not a push permission.

## Decision

The trusted agent discovers all heads from the registered upstream without
fetching their objects. Discovery requests are authorized as `git.repository`
/ `fetch`, with target `refs/heads/*`.
This intentionally does not reuse a saved read rule limited to one exact ref.
The agent validates a bounded list of unique full head refs and SHA-1 commits,
and checks each exact ref through the same capability service before returning
the list. A narrower ref denial fails the listing even if broad discovery is
allowed. Each requested head then receives a fresh exact-ref authorization before
its objects are fetched. The fetched OID must match the advertised OID and name a
commit. Missing, moved, malformed or excessive refs fail closed. One helper batch
contains at most 1024 heads, with a 32 MiB aggregate pack limit.

An independent Host cache namespace avoids treating the symbolic origin HEAD
as an upstream branch. Ordinary Workspace Git uses a wildcard origin fetch
refspec. Registration still selects the initial checkout and the only currently
supported existing-branch push target. Push preparation reads that exact branch;
external mutation then requires its own Policy/approval with fixed old/new OIDs,
Environment generation, repository and remote. Fetch never authorizes push.

No guest-supplied remote, filesystem path, Git configuration or credential enters
the trusted agent. These changes remain in the Standard integration. Core
capability and lifecycle contracts do not acquire Git-specific behavior.

Separate per-ref execution avoids treating an earlier grant as a batch token
while another approval is pending. It can transfer shared history more than once;
efficient large-pack batching is an M4 measurement/optimization item and must
preserve exact-ref deny, revocation and cancellation semantics.

## Rejected alternatives and limits

Silently expanding a one-ref read approval, deriving an allow-push rule from
clone, accepting arbitrary requested OIDs, exposing a credentialed remote or
trusted `.git`, and retrying an ambiguous push are not acceptable shortcuts.
New-branch pushes are a separate change. Force, deletion, multi-ref push, LFS,
submodules and large-pack transport remain outside this slice. Real Git component
tests do not establish native Environment or giant-repository acceptance.
