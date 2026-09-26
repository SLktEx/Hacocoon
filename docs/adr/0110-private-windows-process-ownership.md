# ADR 0110: Own private Windows launch descendants

[日本語](0110-private-windows-process-ownership.ja.md) | English

Status: accepted for the implementation candidate.

## Evidence and decision

On main `63bc41d1`, a native regression reproduced a delayed descendant surviving
private review cancellation and `Close`. `exec.CommandContext` killed the immediate
wrapper, while its child could still run after the launch reservation was released.
This violates [ADR 0108](0108-background-wsl-start-coordination.md)'s cleanup
contract. It does not establish the cause of every earlier attached-disk failure.

The Windows platform implementation creates a private, unnamed job with
`JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` and attaches the review process using
`PROC_THREAD_ATTRIBUTE_JOB_LIST` during `CreateProcessW`. Ownership is established
before execution, so descendants cannot escape by starting before assignment.
No breakaway option is enabled. The job handle is noninheritable. A separate
handle list inherits only the anonymous stdin/stdout and discarded stderr handles;
the validated executable, arguments, directory and minimal environment remain explicit.

Cancellation terminates the exact job and observes zero active processes before
closing its handle. Native accounting, rather than PID/name enumeration, establishes
absence even after the immediate wrapper exits. Observation has a two-second bound.
If termination or observation fails, cleanup reports failure and retains the job
handle; failed readiness also retains the launch reservation until owner exit.
Closing the last job handle on owner exit requests termination of remaining members.
No process-name cleanup, WSL shutdown or additional permission is introduced.

The primitive lives outside Core and currently owns only private notification
peers. The deliberately detached reclaim continuation is not placed in this job.
Human answers, saved Policy and all disk identity/detachment checks are unchanged.

## Native contract and refusal

[UpdateProcThreadAttribute](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute)
documents the creation-time job list on Windows 10+ and explicit inherited handles.
[Job objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
define descendant membership and kill-on-close behavior.
[TerminateJobObject](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-terminatejobobject)
and [basic accounting](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_accounting_information)
define termination and active-member observation. Failure to establish ownership
refuses launch: starting without containment would recreate the demonstrated race.
This is an operation-level API result, not a speculative Windows version probe.

## Rejected alternatives and checks

Assigning after `Start` leaves a spawn race. Killing only the root leaves its
descendants alive. Enumerating by name or reopening PIDs risks unrelated processes
and PID reuse. Retrying until green, increasing the compaction deadline or stopping
unrelated distributions would conceal rather than repair the ownership defect.

Native regressions cover failed readiness with delayed descendants, an already
exited wrapper, unaffected unrelated peers, ordinary exchanges, cancellation and
reservation release. A local installed Hacocoon read-only review handshake also
passed. [Acceptance evidence](../status/acceptance-evidence.md#private-windows-launch-descendants)
separates those checks from human notification actions, packaged acceptance and
the unresolved intermittent tunnel/disk failures.
