# Blocking validation and external observations

[日本語](validation-and-probes.ja.md) | English

These development and review rules apply when adding or changing a check that
can stop installation, setup or a user operation. Review the check's necessity
and its observation mechanism together: a broken check can reject a working
environment.

## Justify the refusal

Record the concrete prerequisite and the consequence of continuing without it
in the owning design or change description. Tie a version restriction to a
required API, known defect, or deliberate compatibility boundary with a stated
reason. Having tested only one version does not itself establish that other
versions are incompatible. Neither an LTS label nor an existing restriction
replaces this reasoning.

Prefer the normal product operation when it can establish the prerequisite and
report an actionable failure. Add a separate preflight check only when it prevents
a concrete unsafe or costly action, or provides a necessary earlier diagnosis.
Do not retain a redundant check solely because it already exists.

## Establish the observation contract

Use a documented API or machine-readable output for automated decisions. Check
the actual upstream schema, field types, command exit behavior and relevant
versions before implementing the reader. Prefer a shared provider/helper boundary
when installation and CI need the same observation.

Human-facing labels, tables and localized messages are presentation. Fixing the
locale alone does not make their format a stable interface. If no structured
interface exists, document the fallback's assumptions and cover supported
language and formatting variations before relying on it.

Distinguish a confirmed unmet prerequisite from an observation failure, such as
a timeout, permission error or malformed response. Define the stop, bounded retry
or warning behavior for each according to the operation's consequences. Unknown
authorization, ownership or isolation still fails closed under the
[security principles](../DESIGN_PRINCIPLES.md#fail-closed-at-trust-boundaries);
an observation error must not be reported as proven incompatibility or absence.

## Test the observer and the decision

For each changed blocking check, provide all three pieces of review evidence:

- The refusal rationale and a link to the prerequisite or compatibility contract.
- The observation contract and its upstream specification or sanitized actual
  response, including the source/version needed to assess the fixture.
- Regressions showing that supported inputs pass and invalid inputs or failed
  observations receive the intended outcome.

Derive mocks and fixtures from those independent sources, not solely from strings
the implementation expects. Include valid variations relevant to the changed
boundary, such as locale, optional fields, JSON layout or supported patch
versions. Exercise missing or wrongly typed fields and nonzero command exits,
including a failed command that still emits plausible data. Keep the tests at
the lowest faithful layer and retain the ordinary user-path E2E when applicable.
Mock success does not establish real-provider acceptance.

The [Incus installer decision](../adr/0064-shared-incus-lts-installation.md)
illustrates the failure: English `Server version` labels and a mock that always
returned those labels missed Japanese output. Reading the JSON API fixes that
observation dependency; the necessity of the version restriction remains a
separate review question.
