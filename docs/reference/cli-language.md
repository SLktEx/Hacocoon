# CLI language

[日本語](cli-language.ja.md) | English

Status: **partial**. The shared selector and listed surfaces are implemented, including automatic presentation selection at normal Windows/WSL entry. Full CLI localization and packaged language acceptance remain incomplete. Issue #577 remains open.

## Select a language

`HACO_UI_LANGUAGE=en` or `HACO_UI_LANGUAGE=ja` selects Hacocoon presentation
without changing OS locale. A nonempty unsupported value selects English. When
this override and explicit `LC_ALL` / `LC_MESSAGES` are empty, normal interactive
Windows/WSL login reads the Windows
display language (Japanese selects `ja`, other languages `en`). This bounded
read uses the system PowerShell at `/mnt/c/Windows`; missing interop, other system
paths or failure fall back to the POSIX selection below. Other CLI invocations
use POSIX selection directly. Host entry forwards
only the resolved `en`/`ja` for that shell session; ordinary Environment shells
receive no such forwarding. See [ADR 0079](../adr/0079-host-presentation-language.md).

The product CLI selects the first nonempty `LC_ALL`, `LC_MESSAGES`, or `LANG`, in that order. Japanese locales such as `ja`, `ja_JP.UTF-8`, and `ja-JP` select Japanese. English, `C`, `C.UTF-8`, `POSIX`, unsupported or malformed values, and an entirely unset locale select English. An unsupported higher-priority value does not fall through to a lower-priority Japanese value. Empty strings are skipped; whitespace is not an empty string.

No installed OS locale is required for message selection. To select Japanese for one Linux/WSL invocation, clearing higher-priority overrides:

```bash
HACO_UI_LANGUAGE=ja haco help
```

To force English for one invocation:

```bash
HACO_UI_LANGUAGE=en haco help
```

On Windows, setting `$env:LANG` alone does **not** establish automatic forwarding in this slice. An explicit invocation can set the locale for the Linux CLI instead:

```powershell
wsl -d Hacocoon --exec env HACO_UI_LANGUAGE=ja haco host shell
```

Use the actual distribution name in place of `Hacocoon` when different. This command is a configuration example, not evidence of native Windows acceptance. The selector does not set environment variables or persist OS, WSL, Environment, or child-process locale changes. The separate installer behavior described in [trusted Host entry](../design/trusted-host.md#host-entry-language) is unchanged.

## Implemented surfaces

- Top-level help, unknown-command and top-level usage errors, and human-readable `haco version` output.
- The trusted Host entry notice, preserving terminal coloring and plain redirected output.
- `haco approve` help, request selection, terminal approval options, outcomes, and saved Policy explanations.
- Root `haco env` usage, create/SSH flag descriptions, and its directly emitted controller/output diagnostics.
- Human-readable `haco env list` and `haco env status`: headings, empty-list guidance, connection commands, and retained-Workspace notice for a stopped Environment.
- Human-readable `haco doctor`: headings, next actions for known local checks, connection labels, and the explicit limit of what the diagnostic tests. It does not run additional probes or repair anything when rendering another language.
- `haco env copy`, `haco env import`, and `haco env export`: usage, JSON-option descriptions, direct controller/output diagnostics, completion and retained-data notices, and localized failure explanations around the original error.
- `haco doctor` usage, human report heading, and next-action label. Its controller-provided detail remains unchanged.

## Compatibility and boundaries

Command and flag names, accepted input, resource identifiers, configuration keys, JSON output and values, operation decisions, and stdout/stderr routing do not change with the selected language. This includes Environment operation messages currently encoded as JSON strings, approval listings/receipts, and `version --json`. The compact `--version` output and generated SSH configuration stay unchanged.

Environment state/access values and diagnostic check/status tokens are not translated. The Environment diagnostic's exported `action` and `scope` text remains in its existing English form for JSON. Private rendering metadata selects Japanese explanations without changing those report fields. Unknown report details retain their original text instead of being guessed from string contents.

Human-readable Environment/transfer output applies terminal escaping to nonprintable characters in names, paths and displayed result fields; it does not rename a resource or rewrite its JSON representation. List rendering sorts a copy and does not reorder caller-owned data. Original Git/SSH/OS/backend errors remain intact as error details; they are not passed through a translation filter.

Locale selection does not alter command success/failure or approval exit codes. One separate output-error correction accompanies Environment diagnostic rendering: a failed write after the first heading now returns the existing failure code `1` in both languages, instead of being ignored.

Approval decisions still use the same `y`/`yes`, `N`, and numbered choices. Empty or unknown input does not authorize an operation. Saving an ask-every-time Policy still requires a separate one-shot answer. Rendering preserves existing terminal escaping; display failure cannot grant authority. The stdio approval adapter receives a language value explicitly and retains it for nested prompts. Its existing constructor remains English for other callers.

The shared catalogs translate trusted message IDs before substituting values.
They are not output-filtering writers. Language is not mutable controller state.
The Host-shell request carries a validated presentation hint for that session;
arbitrary environment forwarding and persisted guest configuration are excluded.

## Remaining scope

Other command families and lower-level Environment diagnostics still need catalog migration. Standard-library flag parse-error details, original Git/SSH/OS errors, structured logs, and controller diagnostic summary/action text remain unchanged; further localized explanations around those details are follow-up work.

WSL-to-Host normalized presentation handoff and automatic normal Windows entry
selection are implemented. Packaged language acceptance and full command coverage
remain pending. This partial implementation must not close Issue #577.

## Validation

Selector/catalog tests cover locale precedence, malformed values, English fallback, Japanese/English placeholder parity, duplicate IDs across catalogs, literal caller IDs, argument preservation, and independent parallel language values. The locale parser also has fuzz coverage.

Product and approval regression tests cover language-independent JSON, exit codes, approval scope and default denial, nested prompts, terminal escaping, and display failures. Additional Environment tests cover list ordering without mutation, status values, identical diagnostic/copy JSON, unchanged probe execution, original error preservation, transfer usage, English action compatibility, and failure at each diagnostic write boundary.

Record full-repository CI, documentation checks, and native acceptance separately in the associated PR. Isolated selector/catalog race tests and vet in a partial checkout do not establish product-package compilation, whole-repository validation, or installed Windows/WSL behavior.

## Main integration candidate

This candidate reuses #580's language catalog and #583's vertical command help.
It retains current main's human-default output and explicit `--json`, portless SSH,
Workspace prepare/fork, automatic Git connection and Experimental VS Code entry.
Both `haco` and `haco-host` render hierarchical help locally before controller setup.
Ordinary diagnostic guidance is `haco doctor <environment>`.

Help describes the current flags; removed SSH port options are not advertised.
Uncertain Environment state keeps its inspection guidance and never recommends
starting an unknown runtime. Normal Windows/WSL entry now selects the display
language and forwards it to the trusted Host; the installer preserves OS locale.
Full result translation and packaged language acceptance remain. Validation is recorded in
[acceptance evidence](../status/acceptance-evidence.md#main-cli-language).

## Detailed command guidance

Required inputs, defaults and advertised options now have English/Japanese
explanations before controller connection. Current JSON opt-in, portless SSH and
Host-only recipe reapply/result options remain. Host setup applies saved recipes
only when unapplied; it is not an unconditional replay.

Base, snapshot, repository, Workspace and OCI results use shared catalogs.
Retained-data deletion shares one client confirmation helper. Failed warning or
prompt delivery cannot authorize deletion, even with `--yes`; controller ownership
and lifecycle checks remain authoritative. Original tool errors and JSON remain unchanged.

## Reclamation results

Reclamation start/confirmation, saved result, measured allocation, incomplete/failed
stages and explicit review use the shared English/Japanese catalog. Protocol state
values, byte counts, operation identities and confirmation scope remain unchanged.
Numeric Windows errors remain diagnostic codes; the surrounding explanation and
next action follow the selected language. The public Windows journey recognizes
both presentations while still checking exact worker receipts for completion.

## Setup outcomes

**Implemented:** Host/project setup completion, saved-script removal, missing recipes,
truncation and failure recovery guidance use the shared English/Japanese catalog.
`haco setup --help` uses the shared vertical command page. Script stdout/stderr,
request IDs, progress state/reason tokens and structured logging remain unchanged.
Language selection cannot replay a recipe or change a failure into success.

## Network operation guidance

Host service registration/removal, rule-saving and connection revocation results,
empty lists, listener readiness and next actions use the shared catalog. Human
notices distinguish a prepared listener or registered service from permission to
connect. JSON, target identities, rule scope/expiry, error details and structured
logs remain unchanged. Setup and network validation share the general logging
configuration message instead of maintaining duplicate translations.

## Configuration guidance

`haco config` uses shared English/Japanese help, inspection/save guidance and
unconfirmed-save/retained-editor notices. Policy values, JSON and original error
details stay unchanged; a failed result write cannot replay the operation.

## SSH and editor entry

Command lists keep an explicit space between every command name and its
explanation, including long names such as `experimental edit vscode` and
`ssh cleanup`. Both language renderers retain the existing 60-column wrapping.

Ordinary `haco ssh setup` and `haco open` use shared English/Japanese Environment
selection, preparation, editor-launch/retry and cleanup notices. Invalid preview
port/client combinations explain the correction before Workspace preparation.
SSH aliases, generated configuration, selection/cancellation, raw client errors,
progress tokens and operation results are unchanged. The separate `haco-vscode`
adapter and lower-level SSH errors retain their own presentation.
