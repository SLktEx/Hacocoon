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

On Windows, setting `$env:LANG` alone does **not** establish forwarding through the Windows launcher in this slice. An explicit invocation can set the locale for the Linux CLI instead:

```powershell
wsl -d Hacocoon --exec env LC_ALL=ja_JP.UTF-8 haco help
```

Use the actual distribution name in place of `Hacocoon` when different. This command is a configuration example, not evidence of native Windows acceptance. No persistent OS, WSL, Environment, or child-process locale changes are introduced by the CLI selector. The separate installer behavior described in [trusted Host entry](../design/trusted-host.md#host-entry-language) is unchanged.

## Implemented surfaces

- Top-level help, unknown-command and top-level usage errors, and human-readable `haco version` output.
- The trusted Host entry notice, preserving terminal coloring and plain redirected output.
- `haco approve` help, request selection, terminal approval options, outcomes, and saved Policy explanations.
- Root `haco env` usage, create/SSH flag descriptions, and its directly emitted controller/output diagnostics.
- `haco doctor` usage, human report heading, and next-action label.

## Unchanged contracts

Command and flag names, accepted input, identifiers, paths, configuration keys, JSON output and values, exit codes, and stdout/stderr routing do not change. This includes Environment operation messages that are currently encoded as JSON strings, approval listings/receipts, and `version --json`. The compact `--version` output stays unchanged.

Approval decisions still use the same `y`/`yes`, `N`, and numbered choices. Empty or unknown input does not authorize an operation. Saving an ask-every-time Policy still requires a separate one-shot answer. Rendering preserves existing terminal escaping; display failure cannot grant authority. The stdio approval adapter receives a language value explicitly and retains it for nested prompts. Its existing constructor remains English for other callers.

The shared catalog translates trusted message IDs before substituting values. It is not an output-filtering writer and never translates external process output, arbitrary errors, resource names, or controller/protocol data. No mutable process-wide language or controller language state is introduced.

## Remaining scope

Other command families and Environment detail/import/export/copy output still need catalog migration. Standard-library flag parse-error details, original Git/SSH/OS errors, structured logs, and controller diagnostic summary/action text remain unchanged; localized explanations around the original details are follow-up work.

Windows launcher to WSL/Host language handoff is not implemented. No transport field, environment forwarding, or guest configuration is added here. Native Windows/WSL and Incus acceptance is pending. This slice must not close Issue #577.

## Validation

The selector/catalog and standalone help/flag adapter have local race-test coverage. Catalog tests check Japanese/English placeholder parity, missing entries, caller message IDs, malformed locales, precedence, fallback, quoted arguments, and independent parallel language values. The locale parser also has fuzz coverage.

Product and approval regression tests cover language-independent JSON, exit codes, approval scope and default denial, nested prompts, escaped untrusted text, and display failure. Full repository execution of those tests, documentation checks, and native acceptance must be recorded separately; the isolated local checks do not establish those results.
