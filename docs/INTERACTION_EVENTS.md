# Client-neutral interaction events

Hacocoon exposes a small, read-only interaction-event contract for client adapters through `github.com/SLktEx/Hacocoon/pkg/interaction`.

This is a **presentation and resume boundary**, not an authorization boundary. Reading an event never approves, executes, retries, or mutates a capability. Approval and execution stay inside the existing Policy/Capability path.

## Event kinds

| Kind | Meaning | Attention |
| --- | --- | --- |
| `approval-required` | Policy requires operator approval before execution | yes |
| `approval-approved` | The trusted approval provider approved the request | no |
| `approval-denied` | Approval was denied or unavailable | yes |
| `policy-denied` | Policy denied the request or policy evaluation failed | yes |
| `operation-completed` | Provider execution completed successfully | no |
| `operation-failed` | Provider execution failed without a proven recovery-required state | yes |
| `recovery-required` | Provider state is incompatible or manual recovery is required | yes |

Every event includes a stable `event_id`, the capability `request_id`, UTC time, kind, optional Environment/capability/action labels, attention/recovery flags, and `next_offset`.

`event_id` is deterministic for one capability lifecycle (`<request_id>:<kind>`), so multiple clients can deduplicate the same observation independently.

## Security minimization

The client event schema deliberately does **not** expose the raw capability resource, request attributes, opaque parameters, provider output, approval tokens, credentials, or free-form audit reason text.

Failure `code` values come from a closed allowlist. Unknown or historical free-form reasons collapse to a generic code such as `operation-failed` or `policy-error`; they are never copied into the client payload.

This means a UI can render notifications without accidentally surfacing Git credentials, command stderr, signed URLs, local sensitive paths, or policy detail strings that were never intended as presentation data.

## Resume and dedup

Use `Batch` with the last committed byte cursor:

```go
reader, err := interaction.NewDefaultReader()
if err != nil {
    return err
}

batch, err := reader.Batch(ctx, cursor, interaction.DefaultBatchSize)
for _, event := range batch.Events {
    // Persist event.EventID if the client needs cross-session deduplication.
    present(event)
}
// Commit this cursor after the batch has been accepted by the client.
cursor = batch.NextOffset
if err != nil {
    return err
}
```

`next_offset` advances across audit records that intentionally produce no client event, so reconnecting clients do not need to replay hidden/internal lifecycle records.

Batches are bounded (`MaxBatchSize`) and the underlying audit reader remains record-bounded. If the audit stream is malformed, `Batch` returns the trustworthy prefix plus a public `*interaction.CorruptionError`; records at and after the first corrupt boundary are not exposed.

## Multiple clients

Any number of clients may read the same interaction stream. Observation is side-effect free, so two clients seeing one `approval-required` event cannot execute the operation twice merely by reading it.

A future browser, VS Code, code-server, JetBrains, or other client that wants to submit an approval must still call a trusted Hacocoon approval/action boundary. An interaction event itself is never an approval token.

## Browser Notifications mapping

Browser-capable clients can map only the minimized fields, for example:

- `approval-required` -> title `Hacocoon approval required`
- `recovery-required` -> title `Hacocoon needs recovery`
- `operation-failed` -> title `Hacocoon operation failed`
- `operation-completed` -> optional low-priority completion notification

A browser client should build body text from `capability`, `action`, and `environment` only. It should not attempt to recover hidden raw audit fields.

Browser Notification permission, service-worker behavior, and UI presentation stay client-owned and outside Hacocoon Core.

## Root selection

`interaction.NewDefaultReader()` follows the same root convention as the local Hacocoon composition: `HACO_ROOT` when set, otherwise `/var/lib/hacocoon`. `NewReader(root)` is available for explicitly scoped adapters and tests.
