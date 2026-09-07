# ADR 0018: Bind ephemeral execution to its client connection

Status: accepted  
Date: 2026-09-07

## Context

The general `run.execute` RPC inherited the controller lifetime. Closing or
cancelling a client stopped the caller, but left execution running and postponed
cleanup indefinitely for an unbounded command. A controller integration regression
reproduced this before exposing temporary execution through the product CLI.

## Decision

Use the existing versioned stream handshake for `run.execute`. The connection has
no input frames: disconnect, read failure or unsolicited input cancels the execution
context. The existing run service still owns creation, execution, durable recovery
and canonical deletion. Its cleanup ignores execution cancellation and retains its
own deadline. Stream shutdown never releases a lease or claims successful deletion.

The server closes the connection and joins its disconnect watcher on completion.
It sends one size-bounded JSON result, preserving execution status and cleanup
errors. Result delivery has a 30-second write deadline. A cancelled caller cannot
claim cleanup succeeded; controller recovery remains authoritative if it cannot
receive the result. The pre-1.0 call form is replaced, not silently retried: retrying
an ambiguously completed run could execute a workload twice.

## Alternatives rejected

- Cancelling every lifecycle RPC on disconnect would change retained creation and
  setup semantics, which intentionally may finish a bounded operation.
- Making the CLI independently delete an Environment after disconnect would race
  the controller and split canonical lifecycle ownership.
- Cancelling cleanup with execution would strand ownership and leases.

## Validation

A real Unix controller/client test invokes the run service with a blocking execution,
cancels the client, and verifies deletion receives an uncancelled, bounded context.
Existing wire tests preserve nonzero exit codes and partial cleanup failures.
This validates controller behavior, not real Incus execution or product CLI UX.
