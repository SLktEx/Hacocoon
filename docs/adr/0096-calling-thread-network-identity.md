# Calling-thread network identity

[日本語](0096-calling-thread-network-identity.ja.md) | English

Status: implemented correction to the existing network dialer.

Linux network namespace membership belongs to a thread. The existing dialer pins
an OS thread, enters the verified guest namespace, creates the socket and destroys
that thread on exit. It never returns a guest thread to the controller pool.

The Host identity check must open `/proc/thread-self/ns/net` from the calling
controller thread. `/proc/self/ns/net` refers to the process leader, which can be
a dedicated dial thread while another controller thread prepares a connection.
Comparing with the leader can therefore mistake an Env namespace for the Host.
The descriptor opened by the syscall pins the caller's namespace even if Go later
schedules the goroutine elsewhere.

Keep the existing Host-namespace refusal, provider identity checks on both sides,
pinned process directory, repeated PID check and dedicated-thread lifetime.
Retrying a denied namespace, skipping the comparison or assuming every thread
shares the process leader's namespace are rejected.

A native regression creates a different namespace on a non-leader thread. The
old opener deterministically reports another thread's identity; the corrected
opener returns the actual calling thread. It needs existing namespace creation
authority and does not alter the leader, network policy or Host listeners.
This proves the defect and correction, not the cause of every earlier Windows
`stream_denied` failure. Installed parallel reconnect acceptance remains separate.
