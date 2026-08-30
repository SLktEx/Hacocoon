# SSM execution recovery contract

`runtime.ec2` treats an AWS Systems Manager `send-command` response containing a valid `CommandId` as proof that AWS accepted the remote operation.

After that point Hacocoon **must not automatically submit the command again** merely because a waiter, network read, or `get-command-invocation` observation fails. A side-effecting command may already be running or may already have completed.

The runtime therefore reconciles only the accepted `CommandId` with bounded `get-command-invocation` observations:

- `Pending`, `InProgress`, `Delayed`, and `Cancelling` remain non-terminal observations and are retried only as reads of the same `CommandId`.
- `Success` or `Failed` with a trustworthy non-negative `ResponseCode` becomes the normal provider-neutral `ExecutionResult`.
- `Cancelled`, `TimedOut`, an unknown status, a terminal result without a trustworthy response code, context cancellation after acceptance, or exhaustion of bounded observations returns `core.ErrExecutionOutcomeUnknown`.
- the returned ambiguity error includes the accepted `CommandId` so an operator/recovery tool can reconcile the original operation instead of blindly re-executing it.
- failure of `send-command` itself is not classified as `ErrExecutionOutcomeUnknown`, because no accepted `CommandId` was obtained.

Callers and orchestrators must treat `ErrExecutionOutcomeUnknown` as **not automatically retryable**. Recovery must inspect the referenced SSM command or require an explicit human/application decision before issuing another side-effecting command.

This contract is intentionally stricter than ordinary `ErrRuntimeUnavailable`: availability loss after remote acceptance is an observation problem, not proof that execution did not happen.
