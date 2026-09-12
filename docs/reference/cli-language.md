# CLI language

[日本語](cli-language.ja.md) | English

Status: **partial**. The shared selector and the surfaces listed below are implemented. This is not complete localization of the shipped CLI or Windows language propagation. Issue #577 remains open.

## Select a language

The product CLI selects the first nonempty `LC_ALL`, `LC_MESSAGES`, or `LANG`, in that order. Japanese locales such as `ja`, `ja_JP.UTF-8`, and `ja-JP` select Japanese. English, `C`, `C.UTF-8`, `POSIX`, unsupported or malformed values, and an entirely unset locale select English. An unsupported higher-priority value does not fall through to a lower-priority Japanese value. Empty strings are skipped; whitespace is not an empty string.

No installed OS locale is required for message selection. To select Japanese for one Linux/WSL invocation, clearing higher-priority overrides:

```bash
env LC_ALL= LC_MESSAGES= LANG=ja_JP.UTF-8 haco help
```

To force English for one invocation:

```bash
LC_ALL=C haco help
```

On Windows, setting `$env:LANG` alone does **not** establish automatic forwarding in this slice. An explicit invocation can set the locale for the Linux CLI instead:

```powershell
wsl -d Hacocoon --exec env LC_ALL=ja_JP.UTF-8 haco help
```

Use the actual distribution name in place of `Hacocoon` when different. This command is a configuration example, not evidence of native Windows acceptance. The selector does not set environment variables or persist OS, WSL, Environment, or child-process locale changes. The separate installer behavior described in [trusted Host entry](../design/trusted-host.md#host-entry-language) is unchanged.

## Implemented surfaces

- Top-level help, unknown-command and top-level usage errors, and human-readable `haco version` output.
- The trusted Host entry notice, preserving terminal coloring and plain redirected output.
- `haco approve` help, request selection, terminal approval options, outcomes, and saved Policy explanations.
- Root `haco env` usage, create/SSH flag descriptions, and its directly emitted controller/output diagnostics.
- Human-readable `haco env list` and `haco env status`: headings, empty-list guidance, connection commands, and retained-Workspace notice for a stopped Environment.
- Human-readable `haco env doctor`: headings, next actions for known local checks, connection labels, and the explicit limit of what the diagnostic tests. It does not run additional probes or repair anything when rendering another language.
- `haco env copy`, `haco env import`, and `haco env export`: usage, JSON-option descriptions, direct controller/output diagnostics, completion and retained-data notices, and localized failure explanations around the original error.
- `haco doctor` usage, human report heading, and next-action label. Its controller-provided detail remains unchanged.

## Compatibility and boundaries

Command and flag names, accepted input, resource identifiers, configuration keys, JSON output and values, operation decisions, and stdout/stderr routing do not change with the selected language. This includes Environment operation messages currently encoded as JSON strings, approval listings/receipts, and `version --json`. The compact `--version` output and generated SSH configuration stay unchanged.

Environment state/access values and diagnostic check/status tokens are not translated. The Environment diagnostic's exported `action` and `scope` text remains in its existing English form for JSON. Private rendering metadata selects Japanese explanations without changing those report fields. Unknown report details retain their original text instead of being guessed from string contents.

Human-readable Environment/transfer output applies terminal escaping to nonprintable characters in names, paths and displayed result fields; it does not rename a resource or rewrite its JSON representation. List rendering sorts a copy and does not reorder caller-owned data. Original Git/SSH/OS/backend errors remain intact as error details; they are not passed through a translation filter.

Locale selection does not alter command success/failure or approval exit codes. One separate output-error correction accompanies Environment diagnostic rendering: a failed write after the first heading now returns the existing failure code `1` in both languages, instead of being ignored.

Approval decisions still use the same `y`/`yes`, `N`, and numbered choices. Empty or unknown input does not authorize an operation. Saving an ask-every-time Policy still requires a separate one-shot answer. Rendering preserves existing terminal escaping; display failure cannot grant authority. The stdio approval adapter receives a language value explicitly and retains it for nested prompts. Its existing constructor remains English for other callers.

The shared catalogs translate trusted message IDs before substituting values. They are not output-filtering writers. No mutable process-wide language, controller language state, transport field, environment forwarding, or guest configuration is introduced.

## Remaining scope

Other command families and lower-level Environment diagnostics still need catalog migration. Standard-library flag parse-error details, original Git/SSH/OS errors, structured logs, and controller diagnostic summary/action text remain unchanged; further localized explanations around those details are follow-up work.

Automatic Windows-to-WSL/Host language handoff is not implemented. Native Windows/WSL and Incus acceptance, full command coverage, and the repository-wide implementation-status/index integration remain pending. This partial implementation must not close Issue #577.

## Validation

Selector/catalog tests cover locale precedence, malformed values, English fallback, Japanese/English placeholder parity, duplicate IDs across catalogs, literal caller IDs, argument preservation, and independent parallel language values. The locale parser also has fuzz coverage.

Product and approval regression tests cover language-independent JSON, exit codes, approval scope and default denial, nested prompts, terminal escaping, and display failures. Additional Environment tests cover list ordering without mutation, status values, identical diagnostic/copy JSON, unchanged probe execution, original error preservation, transfer usage, English action compatibility, and failure at each diagnostic write boundary.

Record full-repository CI, documentation checks, and native acceptance separately in the associated PR. Isolated selector/catalog race tests and vet in a partial checkout do not establish product-package compilation, whole-repository validation, or installed Windows/WSL behavior.
