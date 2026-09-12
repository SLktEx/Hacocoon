# ADR 0035: Publish streamed AWS downloads after verified completion

Status: accepted; implemented repository slice; authenticated AWS acceptance pending
Date: 2026-09-08

## Decision

GetObject uses the existing AWS capability and Policy boundary, with its own
action, exact object ARN/key and IAM action. Listing permission is not download
permission. Approval describes the current object at execution time; the adapter
makes one GetObject request, without pre-approval metadata reads, multipart range
reconstruction, implicit restore or credential vending.

The optional plugin carries a trusted, operation-local output sink in context.
Only the broker supplies this sink; a generic capability request without it
cannot download. Core retains request identity, authorization, audit and execution
outcome, and never stores file contents in CapabilityResult or audit. Result output
contains byte count and SHA-256 only.

The logical Host resolves and pins credentials, rechecks the approved identity,
and streams binary data in bounded frames. ContentLength, complete EOF and a final
identity/hash receipt must agree. Provider transport completion must also succeed:
a receipt followed by an error or extra data is not success. Controller/client
frames are bounded independently of total object size; client backpressure has
a deadline. The Host unit has a ten-minute execution bound even after controller
loss, with memory/task limits. No fixed small object-size limit replaces streaming.

Received bytes are provisional. The Linux client writes them into a private
temporary directory beside the selected destination and independently verifies
size/hash, successful execution and completed audit before publication. Existing
regular files are replaced atomically; symlink and non-regular destinations are
refused. Pinned parent/private directory descriptors prevent renamed directories
from redirecting publication. No remote key or response becomes a local path.

Cancellation, AWS failure, incomplete transport, bad receipt or audit failure
preserves the existing destination. Cleanup never recurses into unexpected
contents; changed private-directory identity is an error requiring inspection.
A post-publication directory-sync/cleanup failure is reported as a local failure,
not silently presented as fully completed cleanup. Filesystems unable to enforce
the private-directory contract are refused, not given a weaker guarantee.

## Rejected alternatives

- Buffering an entire object in JSON creates a hidden small-file limit.
- Writing directly to the destination destroys prior data on partial failure.
- Publishing upon the last data chunk misses transport and final audit failures.
- Reopening staging paths for rename can adopt a substituted directory.
- Range retries without a pinned version can assemble inconsistent object data.

## Evidence and limits

Tests cover real-SDK binary/empty/truncated/partial-content responses with
intercepted HTTP, 20 MiB through the controller stream, ordinary approval and saved
Policy revocation, digest/audit/cancellation refusal, directory replacement and
cleanup identity drift. These do not prove authenticated AWS, guest transport,
native Windows filesystem publication or AWS desktop approval acceptance.
See [AWS operations](../design/aws-operations.md).
