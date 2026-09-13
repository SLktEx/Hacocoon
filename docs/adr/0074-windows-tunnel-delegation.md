# Automatic Windows tunnel delegation

Status: accepted; implementation candidate. [日本語](0074-windows-tunnel-delegation.ja.md)

## Decision

Ordinary WSL/trusted Host tunnel commands place their client listener on Windows.
Native Linux remains local. Use existing controller installation discovery,
normal Windows interop and the installed client companion. Never silently open a
listener in another namespace when that route fails.

Bind delegation to the WSL registration GUID, installation nonce, exact prepared
Env incarnation and original deadline. The companion verifies both identities
through the private controller before listening. Later streams retain canonical
generation/lease/namespace verification. Names and WSL environment hints grant
no authority. The Windows process uses the ordinary WSL user, never root fallback.

An owned input pipe carries one bounded request then remains open as a lifetime
lease. Parent loss or cancellation closes it; the child cancels its listener and
upstreams. The parent reaps that exact child, with a bounded forced-stop fallback.
No detached launcher, service, extra management listener or guest endpoint exists.

The Linux interop child has its own process group. Only the foreground CLI
receives the terminal's interrupt; it closes the lease and waits for the child.
Sharing the foreground group can kill the interop relay before pipe cancellation
completes. Moving the child out of that group does not detach it from its owner
or create a new session. Do not replace a canceled child's nonzero exit with
success: retain real failures and the existing bounded forced-stop fallback.

The installed binary runs as the existing Windows user, who already owns its
application directory. Linux path/record checks reject static foreign ownership;
they are not a Windows file-pinning or ACL guarantee. Normal Envs have no drive,
interop or management projection. No Windows disk-worker authority is imported.

## Rejected alternatives

Reselecting only a display name can target a different registration or Env.
Restarting duration on each side extends the user's requested lifetime. A
background PowerShell launcher hides child lifetime and can leave listeners
behind. Copying Host environments or reusable credentials is unnecessary.
Manual `/init`, custom binfmt and test-only interop repair do not replace the
normal installed route. See [transport](../design/controller-client-transport.md#automatic-windows-tunnel-entry).
