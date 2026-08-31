$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$root = Split-Path -Parent $PSScriptRoot
. (Join-Path $root "scripts/install-windows.ps1")

$tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("hacocoon-windows-provenance-test-" + [guid]::NewGuid().ToString("N"))
$bin = Join-Path $tempRoot "bin"
New-Item -ItemType Directory -Path $bin | Out-Null
$asset = Join-Path $tempRoot "install.sh"
Set-Content -LiteralPath $asset -Value "echo fixture" -NoNewline

$ghPath = Join-Path $bin "gh"
$ghScript = @'
#!/bin/sh
set -eu
if [ "${1:-}" = "attestation" ] && [ "${2:-}" = "verify" ] && [ "${3:-}" = "--help" ]; then
  [ "${HACO_TEST_PROVENANCE_MODE:-ok}" = "help-fail" ] && exit 1
  exit 0
fi
if [ "${1:-}" = "release" ] && [ "${2:-}" = "view" ]; then
  printf '%s\n' "${HACO_TEST_LATEST_TAG:-v1.2.3}"
  exit 0
fi
if [ "${1:-}" = "attestation" ] && [ "${2:-}" = "verify" ]; then
  predicate=0
  signer=0
  source_ref=0
  deny_self_hosted=0
  for arg in "$@"; do
    case "$arg" in
      --predicate-type) predicate=1 ;;
      SLktEx/Hacocoon/.github/workflows/release.yml) signer=1 ;;
      refs/heads/main) source_ref=1 ;;
      --deny-self-hosted-runners) deny_self_hosted=1 ;;
    esac
  done
  [ "$signer" -eq 1 ] || exit 70
  [ "$source_ref" -eq 1 ] || exit 71
  [ "$deny_self_hosted" -eq 1 ] || exit 72
  if [ "$predicate" -eq 0 ]; then
    [ "${HACO_TEST_PROVENANCE_MODE:-ok}" = "generic-fail" ] && exit 1
    exit 0
  fi
  [ "${HACO_TEST_PROVENANCE_MODE:-ok}" = "binding-fail" ] && exit 1
  tag="${HACO_TEST_BINDING_TAG:-v1.2.3}"
  printf '[{"verificationResult":{"statement":{"predicate":{"tag":"%s"}}}}]\n' "$tag"
  exit 0
fi
exit 73
'@
# PowerShell source files intentionally use CRLF on Windows. This fixture is a
# Unix executable, so normalize the embedded script before writing its shebang.
$ghScript = $ghScript -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($ghPath, $ghScript, [System.Text.UTF8Encoding]::new($false))
& chmod +x $ghPath
if ($LASTEXITCODE -ne 0) {
    throw "failed to make fake gh executable"
}

$oldPath = $env:PATH
$oldMode = $env:HACO_TEST_PROVENANCE_MODE
$oldTag = $env:HACO_TEST_BINDING_TAG
$oldLatest = $env:HACO_TEST_LATEST_TAG
$env:PATH = "$bin$([System.IO.Path]::PathSeparator)$oldPath"

function Assert-Throws([scriptblock]$Action, [string]$Needle) {
    try {
        & $Action
    } catch {
        if ($_.Exception.Message -notlike "*$Needle*") {
            throw "expected error containing '$Needle', got '$($_.Exception.Message)'"
        }
        return
    }
    throw "expected action to fail with '$Needle'"
}

try {
    $env:HACO_TEST_PROVENANCE_MODE = "ok"
    $env:HACO_TEST_BINDING_TAG = "v1.2.3"
    $env:HACO_TEST_LATEST_TAG = "v1.2.3"

    $resolved = Resolve-ReleaseVersion "latest"
    if ($resolved -ne "v1.2.3") {
        throw "latest did not resolve to explicit tag: $resolved"
    }

    Assert-TrustedReleaseAsset $asset "v1.2.3"

    $env:HACO_TEST_PROVENANCE_MODE = "generic-fail"
    Assert-Throws { Assert-TrustedReleaseAsset $asset "v1.2.3" } "Trusted build provenance verification failed"

    $env:HACO_TEST_PROVENANCE_MODE = "ok"
    $env:HACO_TEST_BINDING_TAG = "v9.9.9"
    Assert-Throws { Assert-TrustedReleaseAsset $asset "v1.2.3" } "expected tag v1.2.3"

    $env:HACO_TEST_BINDING_TAG = "v1.2.3"
    $env:HACO_TEST_PROVENANCE_MODE = "binding-fail"
    Assert-Throws { Assert-TrustedReleaseAsset $asset "v1.2.3" } "Signed release-binding verification failed"

    $env:HACO_TEST_PROVENANCE_MODE = "help-fail"
    Assert-Throws { Get-GhCommand | Out-Null } "gh attestation verify"

    Write-Host "PASS: Windows installer fails closed on standalone installer provenance"
} finally {
    $env:PATH = $oldPath
    $env:HACO_TEST_PROVENANCE_MODE = $oldMode
    $env:HACO_TEST_BINDING_TAG = $oldTag
    $env:HACO_TEST_LATEST_TAG = $oldLatest
    Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
}

# Expected negative native-command cases leave $LASTEXITCODE non-zero even
# though Assert-Throws handled them successfully. Do not leak that into CI.
exit 0
