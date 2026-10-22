# Approval configuration

For optional editor settings and extensions, see
[Experimental VS Code configuration](experimental-vscode.md).

Status: **implemented; configuration round-trip passed installed GHA at 2584ec6**.

Run these commands in the trusted Linux/WSL Host:

```bash
haco config
haco config --edit
```

The first command prints the current revision and Policy in readable text; use
`haco config --json` for a machine-readable snapshot. The second
edits that same document using `VISUAL`, then `EDITOR`, or `vi` when neither
is set. Edit `policy`; keep `revision` unchanged. This requires no new argument
for ordinary approval or Git operations.

For file-based editing:

```bash
haco config --json > configuration.json
# Edit policy inside configuration.json, preserving revision.
haco config --file configuration.json
```

Add `--json` to the apply command when a script also parses the saved receipt.

`policy.rules` contains administrator rules and `policy.saved_decisions`
contains choices saved by ordinary approval. Both participate in the same
matching rules: deny, then require-approval, then allow. Removing a saved choice
does not remove an administrator restriction. A saved Environment choice binds
to its creation identity; global choices explicitly use `environment: "*"`.

Inspection, successful save and recovery guidance use the shared English/Japanese
presentation. `--json`, Policy values and revision checks are language-independent.
Unconfirmed saves direct you to inspect current settings before retrying; an
edited file remains available on failure. A successful save does not revoke
existing connections. Display failure never retries an edit or grants permission.

Saved Git approval scopes keep repository, remote, target ref and update kind fixed.
Administrator rules may deliberately use explicit wildcards; that is broader authority,
not the default saved scope.
See [Policy semantics](../design/policy-and-capability-foundation.md) and
[Git workflow](../guides/git-workflow.md).

Edits apply to subsequent requests without restarting the controller. Existing
connections are not revoked. In-flight operations still have their normal
pre-execution Policy check.

A concurrent saved approval or configuration edit invalidates an older snapshot.
Read the latest configuration, review the differences and reapply your intended
edit; there is no automatic merge or retry. Failed editor work is retained and
its path is reported. Successful output means persistence and completion audit
both succeeded. After any ambiguous error, inspect current configuration before
retrying; a write may already have completed.

The protected Physical Host `/var/lib/hacocoon/policy.json` remains the single
source. Raw administrator edits must coordinate with its `.policy-save.lock`
or be made while the controller is stopped. The ordinary command does this
coordination automatically. An already malformed or unsafe on-disk Policy is
refused and requires administrator repair; the command does not discard or
replace unreadable rules with defaults. Policy details are not notification
payloads. See [ADR 0027](../adr/0027-revision-bound-policy-editing.md).

Repository tests cover stale edits, simultaneous approval saving, unsafe files,
malformed input, pre/post-write audit failure and next-request behavior. Shipped
CLI/controller E2E covers inspect, editor, file apply and conflict refusal.
These tests do not establish installed Windows/WSL acceptance.


## Discover capability schemas

Before editing rules, ask the running trusted Host controller for its registered
production schemas:

```bash
haco config capabilities
haco config capabilities --json
```

This is read-only metadata, not current Policy, credentials, pending operations,
or a probe of provider availability. It does not prompt, execute an operation,
change rules or save decisions. Missing provider metadata fails discovery rather
than silently producing an incomplete list. The demonstration `local.echo` is
excluded. Descriptions and JSON are language-independent; command guidance is
available in English and Japanese.

JSON version `1` contains `matching` and `capabilities`. Capabilities, actions and
required attribute names are sorted. Each action supplies a description,
`resource` meaning, required `attributes`, optional enum `values`, and relevant
`notes`. This comes from the registered providers, not a separate CLI inventory.
Schemas describe supported authority, not a way to bypass normal preparation or
provider availability checks.

Capability and action names match exactly. Resource, Environment and each
attribute value may be exact or the explicit `"*"` wildcard, never a partial glob.
Every request attribute must appear in the rule and no extra attribute is allowed.
An empty value is not an omitted key; omitted Environment matches only an empty
Environment. Enum values describe canonical requests, not a ban on deliberate
Policy wildcards. Prefer exact values and review broader authority explicitly.
Saved Environment scopes additionally bind to a trusted creation identity.

For example, this administrator rule asks before HTTPS access to one hostname
from `dev`. Insert it into `policy.rules` in the revision-bound snapshot; it is
not a complete replacement configuration:

```json
{
  "capability": "network.egress",
  "action": "connect",
  "resource": "example.com",
  "environment": "dev",
  "attributes": {"protocol": "https", "port": "443"},
  "decision": "require-approval"
}
```

`network.resolve/lookup` is a separate explicit DNS operation. HTTP/HTTPS proxy
egress performs trusted Host DNS after `network.egress/connect` authorization;
that same proxied request does not need a second DNS approval. Network connection
schemas separately describe the prepared destination identity, addresses and
bounded duration. Git push includes `target_ref` and `update_kind`, whose request
values are `fast-forward` or `create`; saved Git scopes widen only `operation_id`,
`old_oid` and `new_oid`. AWS schemas describe account/profile/region and S3
bucket/prefix/key authority without disclosing actual credentials or account data.

## Acceptance

Installed round trips passed at `2584ec6` and `71dbb4f`. The original empty-array display failure and unresolved earlier preview/doctor failure are retained in [acceptance evidence](../status/acceptance-evidence.md#development). Repository tests and installed results remain separate.

Rules may specify an optional RFC 3339 `expires_at`. At that deadline the rule
stops matching; remaining rules and default policy still apply. Malformed values
fail loading. See [rule lifetime](../design/policy-and-capability-foundation.md#rule-lifetime)
and [active connection cancellation](../design/network-connections.md).
