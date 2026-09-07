# ADR 0026: Separate reusable Git approval from fixed execution

Status: accepted. Installed saved-ask GitHub acceptance passed at eb16300; see [the exact scope](../reference/managed-repository-workflow.md#installed-saved-approval-acceptance). Other saved choices have repository integration coverage.

A trusted provider declares reusable scope for a prepared request. The controller
preserves capability, action, resource, Environment identity and every attribute
name. Only declared changing values may become wildcards. It rejects added or
removed keys, replacement values, opaque parameters and caller-observed literal
wildcards. General clients cannot supply saved scope.

Git checks the context-bound prepared operation before declaring scope. Repository,
registered remote, ref and `update_kind=fast-forward` stay fixed. Only old/new OIDs
and operation ID become wildcards. Fetch is separate. Exact attribute-name matching
remains: future authority fields make old rules stop matching until reviewed.
Existing hand-written push rules need the new update_kind field.

Execution still checks the exact prepared request, fixed OIDs, current Environment
identity and latest Policy. Git checks ancestry and atomically compares remote old
OID. A branch allow never grants history rewriting, branch deletion or creation.

Pending output separates current commits/summary from saved_scope. Existing
approve/deny optionally save env/global allow, deny or ask; ask retains a separate
current answer. Display snapshots are copies; malformed/replayed responses cannot
rewrite scope or consume another request.

Clients check pending saved-scope support before sending a saved choice, preventing
older servers from ignoring it and performing one-shot approval. A saved_choice
receipt requires durable persistence and successful audit. Later Git/transport
failure does not roll back Policy: inspect Policy and remote before retrying.
Administrator deny/ask remains stronger than saved allow.

Audit keeps exact request attributes and separate saved_scope on policy-saved.
Both contain only Policy-visible authority, never packs, credentials or opaque
parameters.

Rejected: deleting attributes to loosen matching, pinning reusable rules to
commits, caller-defined wildcard scopes, queue success as persistence proof, and
automatic retry after ambiguous push.

Tests use actual local Git, bare remotes and the ordinary helper. They do not prove
GitHub credential or installed-provider acceptance. Feature push acceptance must
use Hacocoon-test through ordinary approval.
