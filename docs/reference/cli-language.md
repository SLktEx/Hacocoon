# CLI language

[日本語](cli-language.ja.md) | English

Status: **partial**. The shared selector and listed surfaces are implemented, including automatic presentation selection at normal Windows/WSL entry. Packaged English Windows selection passed; Japanese Windows and remaining result/diagnostic translations are incomplete. Issue #577 remains open.

## Select a language

`HACO_UI_LANGUAGE=en` or `HACO_UI_LANGUAGE=ja` selects Hacocoon presentation
without changing OS locale. A nonempty unsupported value selects English. When
this override is empty, normal interactive Windows/WSL login reads the Windows
display language (Japanese selects `ja`, other languages `en`). This bounded
read uses the system PowerShell at `/mnt/c/Windows`; missing interop, other system
paths or failure fall back to the POSIX selection below. Other CLI invocations
use POSIX selection directly. Host entry forwards
only the resolved `en`/`ja` for that shell session; ordinary Environment shells
receive no such forwarding. See [ADR 0065](../adr/0065-host-presentation-language.md).

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

## Retained-data operation messages

Workspace, source-repository, Base, OCI Store and individual-image review tables,
deletion consequences, confirmation, refusal and completion messages use the
shared English/Japanese catalog. Snapshot list/create/delete/restore headings
and direct result diagnostics, and Workspace fork completion guidance are also
localized. Names, ownership identities, state/role tokens and JSON remain literal;
backend errors remain original details.

The five reviewed deletion clients share one confirmation function. It retains
the existing terminal requirement, 128-byte input bound, `y`/`yes` answers,
default refusal and explicit `--yes` behavior. Failure to display the warning or
prompt stops before deletion, including warning failure with `--yes`. Output
failure after an actual deletion returns failure without retrying or undoing it;
inspect the current inventory before retrying. Reference and ownership checks
remain in the canonical controller operations.

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

The product's public command hierarchy and individual help now describe arguments,
options, defaults and prerequisites in both languages. Every product FlagSet
description uses the same message catalogs. `setup`, `config`, `approve`, `doctor`,
`reclaim` and `version` help also return before controller creation. The trusted
Host's individual help shares this renderer. Descriptions wrap to 60 display
columns; copyable examples and option tokens keep their literal text.

Human result/error messages in remaining command families and lower-level
Environment diagnostics still need catalog migration. Standard-library flag
parse-error details, original Git/SSH/OS errors, structured logs, and controller
diagnostic summary/action text remain unchanged. Existing JSON values, including
messages encoded as JSON strings, are not translated.

WSL-to-Host normalized presentation handoff and automatic normal Windows entry
selection are implemented. English packaged selection passed at `0c79f820`;
Japanese Windows and full result-message coverage remain pending. This partial
implementation must not close Issue #577.

## Validation

Selector/catalog tests cover locale precedence, malformed values, English fallback, Japanese/English placeholder parity, duplicate IDs across catalogs, literal caller IDs, argument preservation, and independent parallel language values. The locale parser also has fuzz coverage.

Product and approval regression tests cover language-independent JSON, exit codes, approval scope and default denial, nested prompts, terminal escaping, and display failures. Additional Environment tests cover list ordering without mutation, status values, identical diagnostic/copy JSON, unchanged probe execution, original error preservation, transfer usage, English action compatibility, and failure at each diagnostic write boundary.

Record full-repository CI, documentation checks, and native acceptance separately in the associated PR. Isolated selector/catalog race tests and vet in a partial checkout do not establish product-package compilation, whole-repository validation, or installed Windows/WSL behavior.

## Additional integrated candidate coverage

The M0 candidate reuses #580's catalogs and adds hierarchical help plus daily
failure, retained-data and resume guidance. Help retains `haco open .`, Workspace
prepare/fork and TCP/UDP additions. The actual Environment diagnostic command is
`haco doctor <environment>`. This does not complete automatic Windows/WSL/Host
language acceptance or localization of every shipped command.

The product and trusted Host client now use one hierarchical help renderer.
`haco-host --help` and every public Host subcommand help return locally before
controller construction. This expands help coverage, not translation of every
Host result. Windows installation no longer derives and persists the OS locale
from its display language. The normalized Host-session handoff is implemented;
automatic Windows selection at normal interactive entry is now implemented.
