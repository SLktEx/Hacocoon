# Client-neutral interaction events

## Ordinary trusted Host subscription

Implemented: normal setup provisions the same-release `haco-notify` companion in
`haco-host`. Run `haco-notify native` there, or `haco-notify web` for the optional
browser view. The existing `HACO_CLIENT_MODE=controller` selects the existing
management socket automatically; no endpoint or audit-path argument is needed.
A controller failure never falls back to local audit files. The explicit
`NewReader(root)` API remains available for file-based/offline consumers.

The trusted client receives the existing private `events.stream` records and
projects them through the same minimized public schema before notification or
HTTP delivery. Neither raw audit files nor a new control endpoint are mounted
into Environments. Consumer errors and batch limits preserve resume semantics;
controller failures return a generic public error and the received prefix.
Detailed typed corruption diagnostics remain specific to the direct file reader.

Windows installation records its validated distribution identity alongside the
Windows PATH record and setup projects it into the owned Host. Conflicting
identities fail closed. Upgrading an older installation requires rerunning the
Windows installer so the identity is captured before controller setup.


The trusted Host can review current pending requests with haco approve. This is a separate private management path; the optional Windows native adapter opens the same CLI. See [pending approval review](design/pending-approval-review.md).

## Approval correlation

Trusted approval prompts and pending Git proposals now carry the same controller-assigned
`request_id` as interaction events, audit records and the final capability result.
This is implemented groundwork for notification review; Windows native activation opens the local CLI. The ID grants no authority. Git decisions still use the existing
trusted management endpoint and proposal ID. No action endpoint or sensitive detail
is added to the read-only event bridge.

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

## Reference notification adapters

`haco-notify` is a presentation-only helper. It never submits approvals or executes capabilities.

### Browser

Run the loopback-only reference client on the trusted Host:

```bash
haco-notify web --listen 127.0.0.1:18081
```

Then open `http://127.0.0.1:18081/` and grant Browser Notification permission. The page uses a same-origin, read-only `/api/v1/events` endpoint backed by `pkg/interaction`, stores the committed cursor and recent stable event IDs in browser local storage, and uses a service worker for notification presentation. The HTTP bridge rejects non-loopback listen addresses and does not enable CORS.

### Native OS notifications

Run:

```bash
haco-notify native
```

On WSL the first maintained path uses `powershell.exe` to surface Windows toast notifications. On a Linux desktop, `notify-send` is used when available. The adapter stores its cursor and recent stable event IDs in a mode-0600 state file. `operation-completed` remains opt-in through `--include-completed`.

Native notification text is constructed only from the minimized public interaction fields. Windows notification strings are transferred to PowerShell as encoded data rather than interpolated into executable script text.

### VS Code

The optional VS Code presentation client lives at [`../clients/vscode-notify/README.md`](../clients/vscode-notify/README.md). It reads the same loopback `/api/v1/events` bridge, persists cursor/dedup state through VS Code `globalState`, and shows normal VS Code notifications.

The extension is not required by `haco-vscode` and does not replace standard Remote-SSH. Presentation remains read-only; Review opens a separate local CLI. Displaying or clicking a notification is not an approval.

## Root selection

`interaction.NewDefaultReader()` follows the same root convention as the local Hacocoon composition: `HACO_ROOT` when set, otherwise `/var/lib/hacocoon`. `NewReader(root)` is available for explicitly scoped adapters and tests.

The optional desktop VS Code Review action now opens the trusted local CLI without answering. Native OS activation remains planned. [Contract](design/pending-approval-review.md).

### Native notification state

The Linux/WSL client holds a process lock for its state file until it exits. A
second client using the same state stops before reading events or delivering
notifications; no additional user option is required. The lock file remains in
place after exit so parallel processes cannot acquire locks on different inodes.

State files must be regular, private, owned by the current user and have one hard
link. The parent must be an owned directory without group/other write access.
The client pins this directory for its lifetime, rejects linked/special state
files, limits input to 128 KiB, and saves through an exclusive random temporary
file, file sync, atomic rename and directory sync. It never writes through the
old predictable `.tmp` path. Unsupported platforms fail closed rather than
silently omitting ownership or process locking. These guarantees prepare for
background use; automatic notification startup remains unimplemented.

Notification delivery and cursor persistence are separate operations. A crash between them can repeat a presentation after restart; it cannot approve or replay a capability.
