# Experimental VS Code configuration

[日本語](experimental-vscode.ja.md) | English

Status: **implemented repository slice; actual VS Code/Marketplace and Windows/WSL acceptance remain unverified**.
Configuration structure may change incompatibly during the Experimental period.

Run in the same trusted Linux/WSL Host user account used for `haco open`:

```bash
haco experimental edit vscode
haco experimental edit vscode --file vscode.yaml
haco experimental edit vscode --json
haco experimental edit vscode --json \
  | jq '.settings["editor.formatOnSave"] = true' \
  | haco experimental edit vscode --json -
```

Without flags, the command opens only the `experimental.vscode` subtree using
`EDITOR`, then `VISUAL`, then `vi`. `--file` reads a YAML subtree and replaces it.
`--json` prints the subtree without changing anything; `--json -` replaces it
from JSON stdin. `--file` and `--json` cannot be combined. Apply diagnostics go
to stderr; JSON apply emits no stdout. There is no CLI-specific database.

The canonical YAML is `$XDG_CONFIG_HOME/hacocoon/config.yaml`, or
`~/.config/hacocoon/config.yaml` when XDG_CONFIG_HOME is unset. This is trusted
Host user configuration, separate from `haco config` and its approval Policy.
The full file may contain:

```yaml
experimental:
  vscode:
    settings:
      editor.formatOnSave: true
      files.trimTrailingWhitespace: true
    extensions:
      minReleaseAge: 30d
      preRelease: deny
      install:
        - id: ms-python.python
        - id: golang.go
        - id: rust-lang.rust-analyzer
          preRelease: allow
        - id: some.extension
          version: 1.2.3
```

`some.extension` is a placeholder: replace it with a real extension/version.
The editor and `--file` take the contents below `vscode`, starting with `settings`
and/or `extensions`, without the `experimental` or `vscode` wrapper.

| Field | Behavior |
|---|---|
| `settings` | Object of VS Code setting names and JSON-compatible values; common Remote settings for the whole Env/Workspace |
| `extensions.minReleaseAge` | Defaults to `30d`; whole days (`d` = 24 hours) or nonnegative durations such as `12h`, `90m`; `0d` permits newly published releases |
| `extensions.preRelease` | `deny` by default; `allow` permits both stable and pre-release candidates |
| `extensions.install[].id` | Required `publisher.name`; duplicate IDs, ignoring case, are rejected |
| `extensions.install[].preRelease` | Optional `allow`/`deny` override for this extension |
| `extensions.install[].version` | Exact semantic version; bypasses age and pre-release filters, never platform or installation validation |

Automatic selection chooses the highest semantic version satisfying age and
pre-release constraints for the Env's Linux x64/arm64 platform. Age uses the
Marketplace version's `lastUpdated` timestamp, conservatively treating a changed
artifact as newly released. The platform-specific artifact takes precedence over
the universal artifact. The boundary is inclusive: exactly 30 days qualifies.
There is no `force`, version range, latest fallback or per-extension age override.
Unknown fields, duplicate YAML keys, extra documents and malformed input fail
before saving. Settings accept JSON values, not YAML-specific non-string keys or
non-finite numbers. Quote values intended as strings under YAML 1.1 rules.

Saving preserves other configuration subtrees and values, but reserializes YAML
and does not retain comments or original formatting. Concurrent CLI edits are
serialized and stale edits are rejected. Failed editor files are retained with
their location reported. Direct YAML editing with `yq` is supported; coordinate
concurrent direct writes with the adjacent `.config.lock` using `flock`, or edit
while no CLI writer is running. Inspect the file after an ambiguous save error.

## Apply to an Environment

Run `haco open <env>` (or ordinary `haco-vscode open <workspace>`) after editing.
If the subtree is absent, opening retains its existing behavior. SSH-only and
`--no-launch` modes do not apply VS Code configuration. Use `{}` as the subtree
to remove previously managed settings on the next open.

The adapter launches stable desktop VS Code, waits up to two minutes for its
matching Remote-SSH server, and writes Remote settings under the Env user's
`~/.vscode-server/data/Machine/settings.json`. No `.vscode` file or workspace
file is generated in a repository. Existing unrelated JSON settings are retained;
an ownership comment records which setting keys Hacocoon manages. Removing a key
from YAML removes that managed key on the next application. Unmanaged JSONC
comments/trailing commas are currently rejected without overwriting the file;
convert that Remote settings file to strict JSON first. Standard VS Code scope
and precedence still apply, including any existing repository settings.

The Host queries Marketplace metadata. Exact selected versions, including
dependencies and Extension Packs under the global policy, are installed by the
Env's VS Code Server CLI. Explicit install entries also override dependencies.
Server-side implicit dependency resolution is disabled. No Host credential or
management socket is passed to the server. Guest downloads still require the
Env's existing egress authorization; this feature grants no network Policy.
VS Code validates engine compatibility. Missing candidates, incompatible versions,
installation errors and version mismatches fail; there is no automatic fallback.

Applications are serialized per Env. A failure may leave settings and earlier
extensions applied; it does not delete the Env or roll back installed extensions.
Removing an install entry does not uninstall it. Versions are reconciled when
opening through Hacocoon; this is not a continuous security enforcement mechanism
against VS Code UI changes, automatic updates or untrusted guest processes.

Env rootfs deletion removes the applied settings/extensions. Host YAML survives
and can be reapplied after recreation. No credentials or private keys are copied
into settings. Repository-specific Folder Settings, VS Code UI for Env/repo/Git/
SSH Agent/remote/Approval state, and Agent Host adapter integration remain future
work. Custom server paths and Insiders are not supported by this first version.

See [ownership and failure semantics](../design/experimental-vscode.md).
