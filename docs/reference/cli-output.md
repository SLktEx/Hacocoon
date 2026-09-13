# CLI output policy

Product `haco` treats human-readable output as the default interface.

- Normal commands print concise text, tables, or key/value blocks for people.
- Machine-readable JSON is emitted only when the user explicitly passes `--json`.
- Successful command results are written to stdout.
- Errors, diagnostics, progress, warnings, and interactive prompts are written to stderr.
- Commands that already have a richer human presentation (for example list tables) keep that presentation instead of falling back to generic key/value output.

Scripts that parse command results should always request `--json` and must not depend on the default human-readable format.

Internal transport entry points may use fixed machine-readable formats when they are not part of the public daily CLI. In particular, `_reclaim-linux` remains an internal Windows/Linux handoff protocol; public `haco reclaim` output follows the normal human-readable CLI rules.

Human key/value output escapes nonprintable characters in external keys and values,
including embedded newlines and terminal control sequences. Explicit JSON preserves
the original data. This uses the same display boundary as Environment tables.
