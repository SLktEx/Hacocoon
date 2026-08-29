# v0.3 — Client & Interactive Access

Status: **roadmap contract implemented on `main`.** Client/status/connection behavior exists, while Hacocoon remains pre-1.0 and the concrete CLI/client surface may still change.

## Goal

Make a Hacocoon Environment comfortable for a human developer without making any single IDE part of Core.

## In scope

- Environment inspect/status suitable for clients.
- SSH access where appropriate.
- VS Code Remote-SSH workflow as a documented client path where applicable.
- code-server usage pattern where useful.
- Local-development port forwarding/exposure needed to reach services in an Environment.
- Stable conceptual client-facing identifiers and connection information.
- Clear separation between Client adapters and Environment implementation.

## Port boundary

v0.3 provides the **connection mechanism**, but local exposure must default to safely scoped/loopback access. Broad network exposure, sensitive port publication, or exceptions that require an authorization decision belong behind the v0.4 Policy/Capability boundary.

Do not invent a second approval system inside the client/port layer.

## VS Code rule

VS Code is a first-class **client**, not the Hacocoon UI architecture.

No VS Code extension should be created merely to duplicate capabilities already provided by standard terminals, SSH, ports, browser mechanisms, or another existing protocol. Extension work remains optional and demand-driven.

## Other clients

JetBrains or another IDE can be added when it can consume the same Environment/connection boundary. Hacocoon Core must not branch on IDE brand.

## Not in scope

- GitHub security capability.
- Policy decisions for privileged external exposure.
- AI orchestration.
- Proprietary editor/chat UI.

## Compatibility note

Client commands and connection metadata are still pre-1.0 implementation surface. Preserve the client/Environment responsibility boundary, but do not keep an awkward concrete interface solely for backward compatibility.
