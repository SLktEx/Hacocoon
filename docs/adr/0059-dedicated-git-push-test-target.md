# ADR 0059: Isolate real Git push acceptance from the product repository

Status: accepted.

Real push acceptance must never derive its mutation target from the repository
running CI. The fixed target is `https://github.com/SLktEx/Hacocoon-test.git`.
Run only through an explicit workflow dispatch on trusted main, using the dedicated
`HACO_TEST_REPOSITORY_TOKEN` secret. The product repository's Actions token has
read-only contents permission and is not passed to the push fixture.

If the dedicated credential is not configured, dependent steps are skipped and
the workflow reports SKIP. This is not successful push acceptance. Never expose
credential values or borrow product-repository write authority as a fallback.

Clone the target's current history as the test Workspace. Use per-run branches,
refuse existing names, and retain the resulting branch/commit for inspection.
Do not delete a branch based only on an expected name, including on a refused run.

The retained fixture exercises the legacy policy-controlled Git capability. It
does not prove the installed product remote helper, interactive approval, native
Environment or import/reconnection acceptance. Those remain separate gates and
must use the same dedicated target when implemented.