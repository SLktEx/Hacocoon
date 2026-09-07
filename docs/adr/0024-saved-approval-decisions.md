# ADR 0024: Saved approval decisions preserve administrator Policy

Status: accepted; controller transport and user interface integration pending.

Persistent choices are explicit allow/deny for the selected Environment or for
all Environments. They are stored as saved_decisions in the same human-editable
Policy JSON. They never replace administrator rules. Both lists participate in
deny > require-approval > allow evaluation; default applies only without matches.
Consequently an explicit administrator ask can still prompt after a saved allow.

The rule copies the observed capability, action, resource and every attribute.
Global means environment:"*" only. No resource or attribute wildcard is inferred;
an observed literal "*" cannot be persisted through this interface. Opaque
provider parameters never become policy or approval-display data.

The Linux writer requires a safe owner-controlled directory and a private,
regular, singly linked Policy file. It uses a no-follow advisory lock, bounded
strict decoding, an exclusive temporary file, fsync and atomic replacement.
It detects intervening content changes before replacement. Manual editors should
coordinate with the lock or edit when no approvals are active; arbitrary editors
do not participate in advisory locking. It never overwrites malformed input.

The capability service audits the selected decision before saving. A failed
save or post-save audit prevents execution; a post-save audit failure reports
audit-incomplete because the durable choice may already exist. It reevaluates
Policy after saving before provider execution. Retries replace only a saved
rule with the same complete match scope. A saved allow never removes deny/ask.

Rejected: mixing generated choices into administrator rules, silently widening
commit/attribute scopes, granting persistence after a failed approval audit,
and pretending a failed persistence operation was a successful one-shot choice.
