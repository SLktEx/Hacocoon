# ADR 0115: Start fresh reclamation from a Windows-native client

[日本語](0115-native-windows-reclamation-entry.ja.md) | English

Status: accepted for the source candidate; native and installed acceptance pending.

## Evidence and decision

The ordinary Host route crosses Linux-to-Windows interop. Current Windows runs
report process-creation error 5 while the observed immediate Job forbids breakaway.
This identifies an incompatible launch context, not every ancestor Job or the
exact failing kernel operation. Do not remove the worker's breakaway requirement
or change WSL/runner Job settings.

The acceptance terminal has a separate ownership defect: suppressing explicit
termination does not make pywinpty/ConPTY destruction non-signalling. Pipes change
ordinary Host entry to Physical Host Bash under [ADR 0066](0066-wsl-login-bootstrap-routing.md).
Neither is a repair or permission to retry an uncertain operation.

Add the explicitly invoked Windows public companion `haco-reclaim.exe`, following
the installed-client separation of `haco-tunnel.exe`. The fixed internal
`haco-wsl.exe` remains preparation/worker host. This extends
[ADR 0048](0048-storage-reclamation-identity.md)'s public entry, not canonical
record, enrollment, mutation or review authority. The Linux command never falls
back to the new client after a failed dispatch.

## Authority and fresh preparation

An explicit distribution name selects exactly one registration with matching
saved enrollment and Windows owner. Selection starts no WSL, creates no key,
enrolls no target and does not pin the active disk. Names are selectors, not
authority. Native status reads saved results without WSL or disk pins.

Preparation holds the existing continuation/launch reservations, disk/ancestor
pins and enrollment checks. Pending, malformed or unreadable current evidence
refuses before any WSL process. Final failed/interrupted review eligibility stays
in the unchanged canonical `beginVersion` operation.

Fixed GUID-bound read-only discovery has creation-time private Windows descendant
ownership, extending [ADR 0110](0110-private-windows-process-ownership.md), not
placing the mutation worker in that Job. Successful output alone is insufficient:
observe root success where required, match both identities to enrollment, and
confirm owned Windows Job members absent before intent or reservation release.
Explicit owner cleanup must preserve errors discarded by RPC connection Close.
Unconfirmed cleanup refuses intent and retains reservations until helper exit.

Windows Job accounting does not prove Linux-process absence. The fixed root
identity reader completes before success; controller discovery returns after its
fixed identity reader exits. A residual Linux stdio bridge has no retry, WSL-launch
or mutation action in this operation. Its teardown and WSL Service behavior remain
installed-verification gaps. Exact-distro stop and detached-VHDX checks still gate
compaction.

Only then generate, flush and reread a fresh version-2 intent. Release preparation
guards before the unchanged worker reacquires its own guards and pins. Launch only
the fresh ID returned by this preparation, once, never an ID from status or a prior
call. Do not review, replay, clear records or choose a fallback after uncertainty.

## Nonterminating client supervision

Pin the installed helper's fixed executable, ownership record and ancestor paths.
Use explicit native standard handles, minimal environment and bounded receipts.
Establish nonblocking pipe reads before process creation or refuse without a
weaker fallback. The public client owns no PTY and adds no kill-on-close worker Job.

Cancellation, timeout, pipe loss, malformed output and nonzero exit after possible
preparation/launch remain failures with potentially retained evidence. Closing
observation/process handles does not request termination or prove no worker exists.
There is no worker kill, retry, WSL reentry, automatic review or alternate launcher.
The read-only discovery Jobs are deliberately cancellable and reaped; the detached
worker is excluded. External Jobs, runner/console shutdown and OS exit may still
end processes. This is not an arbitrary process-survival guarantee.

## Packaging and rejected alternatives

Install the separately checksummed amd64/arm64 public client in its own per-user,
per-registration component area, separate from helper enrollment. Add no PATH
entry, elevation, service, scheduled task, credential or persistent endpoint.
Keep the helper's self-executable `_continue` coupling instead of importing its
launcher into a binary without worker mode.

Reject internal helper commands as a public workaround, a generic recovery broker,
WMI/COM escape, relaxed Job flags, WSL downgrade, library monkeypatches, leaked
references, destructor suppression, and treating EOF/PID/dispatch as completion.
A piped route is not interchangeable with ordinary Host acceptance.

## Verification boundary

Local mocks, packaging tests and cross-compilation are source evidence only.
Frontend synthetic native fixtures require explicit opt-in and fail-fast. The
dedicated command runs both frontend and private-process fixture groups fail-fast
with required PASS checks, containing only fixed self-test processes in
creation-time owned test Jobs, private console events and temporary ordinary files. Their test-only cancellation/cleanup model must never
be applied to a real mutation worker. Required named tests must actually pass;
skipped or missing evidence is not success.

Native pipe/console/handle behavior, caller breakaway, Windows descendant cleanup,
Linux teardown and installed fresh-operation results require separate acceptance.
Reparse fixtures, injected pipe-setup failure, independent pre-canceled zero-start
proof and native arm64 remain gaps. Installed success requires complete persisted
results, exact worker absence, preserved data and resumed Host.

Existing PR workflows still include real reclamation. Do not open a PR or run that
journey merely because source checks pass. A separately reviewed and authorized
synthetic-only branch/workflow can validate components without changing original
gates/history; it does not establish a safe real reclaim CI path.
