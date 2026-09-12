# ADR 0006: Run installed Host setup through the controller

Status: accepted  
Date: 2026-09-06

## Context

The installer started the Physical Host controller but still called
`hacoq host ensure`. That legacy process built another local composition and
owned the client provisioning sequence. A product command implemented by calling
it would preserve a second orchestration path and a permanent legacy dependency.

## Decision

The installer calls `haco setup`. This thin client invokes the fixed
`system.setup` method of the already running Physical Host controller. The
controller reuses the Incus adapter's owned-host/storage/network reconciliation
and verified client provisioners. Only `haco` and client-only `haco-host` are
required for trusted-host provisioning; legacy guest-client provisioning is removed.
The legacy `hacoq host ensure` entry fails before local composition.

Requests carry no paths, binary names, commands, image, project, network or force
options. Companion paths come from the running controller executable, not the
client, environment or PATH. Both source binaries must pass the existing
executable/ownership/mode validation before provider mutation. Exact owned
resources are retained and revalidated; unknown collisions fail closed.

Authorization is the existing root/privileged-group Unix socket boundary.
Trusted clients can already request a Host shell through that same controller.
No raw Incus socket, guest controller or additional management state is created.

Only one setup RPC executes at a time; overlapping calls receive busy. A lost
client connection does not imply the provider operation stopped: the controller
keeps the operation serialized until completion, controller shutdown or its
15-minute deadline. The client has a 16-minute deadline and does not retry a
mutation automatically. Explicit retries reconcile the retained owned resources
and already verified binaries; partial failure does not trigger deletion,
unregister, reformat or ownership release.

Setup acknowledges resource preparation. The installer separately proves the
trusted controller round trip and bounded DNS/route/HTTPS readiness before its
success message. `haco doctor` remains read-only; it is never a repair alias.
The fixed connectivity probe clears inherited environment and curl user config.
Provider errors are not copied into setup logs or responses. The controller logs
the owning failure once; the client renders its selected error and next action.

## Rejected alternatives

- Product-to-hacoq subprocess delegation or a second installer composition.
- Caller-selected binary paths, root commands or force/delete recovery options.
- Treating a disconnected client as proof that a mutation stopped.
- Rolling back an existing owned host by deleting user data on a failed retry.

## Validation and limits

Component tests cover source validation before provider calls, ownership/client
mode refusal, interrupted client provisioning followed by idempotent retry,
rejection of caller parameters, serialized concurrent/lost-client requests, safe
failure responses and a CLI that works with no legacy executable or local state.
The actual installer control flow is tested for failure before each later stage.
Packaged Windows current-install application, rerun, cold ordinary entry and
trusted-host data retention passed on `b71f88e`. The fresh Windows BAT/restart/rerun
gate passed on `a4c6e2d`, with unchanged product source and corrected test-harness
exit/terminal handling. Ubuntu installer and all four real-Incus jobs passed on
`b71f88e`. These results do not establish Windows reboot, Environment egress or
Workspace work-retention acceptance. See [implementation status](../IMPLEMENTATION_STATUS.md).
