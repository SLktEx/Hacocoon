# Approval configuration

Status: **implemented repository slice; installed acceptance pending**.

Run these commands in the trusted Linux/WSL Host:

```bash
haco config
haco config --edit
```

The first command prints the current revision and Policy as JSON. The second
edits that same document using `VISUAL`, then `EDITOR`, or `vi` when neither
is set. Edit `policy`; keep `revision` unchanged. This requires no new argument
for ordinary approval or Git operations.

For file-based editing:

```bash
haco config > configuration.json
# Edit policy inside configuration.json, preserving revision.
haco config --file configuration.json
```

`policy.rules` contains administrator rules and `policy.saved_decisions`
contains choices saved by ordinary approval. Both participate in the same
matching rules: deny, then require-approval, then allow. Removing a saved choice
does not remove an administrator restriction. A saved Environment choice binds
to its creation identity; global choices explicitly use `environment: "*"`.
Git rules keep repository, remote, ref and fast-forward update kind fixed.
See [Policy semantics](../design/policy-and-capability-foundation.md) and
[Git workflow](managed-repository-workflow.md).

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
