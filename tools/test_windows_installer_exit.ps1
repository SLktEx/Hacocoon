param([string]$PausePython = "")
# Run the shipped BAT against a disposable native PowerShell stand-in. This is
# an exit-propagation component test, not Windows/WSL installation acceptance.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
# Process exit can precede release of the Windows directory sharing reference.
# Retry only that native sharing condition on this exact empty fixture directory.
function Remove-ExitFixtureDirectory([string]$Path) {
    $deadline = [DateTime]::UtcNow.AddSeconds(10)
    while ($true) {
        try { [IO.Directory]::Delete($Path, $false); return }
        catch {
            $cause = $_.Exception
            while ($null -ne $cause.InnerException) { $cause = $cause.InnerException }
            $nativeCode = $cause.HResult -band 0xffff
            if ($cause -isnot [IO.IOException] -or $nativeCode -notin @(32,33) -or [DateTime]::UtcNow -ge $deadline) { throw }
            Start-Sleep -Milliseconds 100
        }
    }
}
$fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) ('haco-bat-exit-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($fixtureRoot) | Out-Null
$files = @('install-windows.bat', 'install-windows.ps1', 'powershell.exe', 'native-exit.txt', 'release-hold.txt', 'hold-ready.txt')
try {
    [IO.File]::Copy((Join-Path $PSScriptRoot '../scripts/install-windows.bat'), (Join-Path $fixtureRoot 'install-windows.bat'))
    [IO.File]::WriteAllText((Join-Path $fixtureRoot 'install-windows.ps1'), '# Native boundary stand-in only')
    Add-Type -OutputType ConsoleApplication -OutputAssembly (Join-Path $fixtureRoot 'powershell.exe') -TypeDefinition @'
using System;
using System.IO;
class NativeExit {
    static int Main(string[] args) {
        if (args.Length == 1 && args[0] == "hold") {
            string release = Path.Combine(Path.GetDirectoryName(typeof(NativeExit).Assembly.Location), "release-hold.txt");
            File.WriteAllText(Path.Combine(Path.GetDirectoryName(release), "hold-ready.txt"), "ready");
            for (int i = 0; i < 100 && !File.Exists(release); i++) System.Threading.Thread.Sleep(100);
            return File.Exists(release) ? 0 : 1;
        }
        Console.WriteLine("Native boundary completed");
        string root = Path.GetDirectoryName(typeof(NativeExit).Assembly.Location);
        return int.Parse(File.ReadAllText(Path.Combine(root, "native-exit.txt")));
    }
}
'@
    foreach ($code in @(0, 1, 37, 3010)) {
        [IO.File]::WriteAllText((Join-Path $fixtureRoot 'native-exit.txt'), [string]$code)
        $info = [Diagnostics.ProcessStartInfo]::new()
        $info.FileName = Join-Path ([Environment]::SystemDirectory) 'cmd.exe'
        $info.Arguments = '/d /c install-windows.bat'
        $info.WorkingDirectory = $fixtureRoot
        $info.UseShellExecute = $false
        $info.CreateNoWindow = $true
        $info.EnvironmentVariables['HACO_INSTALL_NO_PAUSE'] = '1'
        $info.RedirectStandardOutput = $info.RedirectStandardError = $true
        $process = [Diagnostics.Process]::Start($info)
        try {
            $stdout = $process.StandardOutput.ReadToEndAsync()
            $stderr = $process.StandardError.ReadToEndAsync()
            if (-not $process.WaitForExit(10000)) { $process.Kill(); [void]$process.WaitForExit(10000); throw 'BAT fixture timed out' }
            $output = $stdout.GetAwaiter().GetResult() + $stderr.GetAwaiter().GetResult()
            if ($process.ExitCode -ne $code) { throw "BAT lost native exit $code" }
            if (($output.Contains('Windows installation complete.')) -ne ($code -eq 0)) { throw "False BAT completion for $code" }
            if (($output.Contains('paused until Windows restarts')) -ne ($code -eq 3010)) { throw "Wrong BAT restart classification for $code" }
            if (($output.Contains('installation failed')) -ne ($code -notin @(0,3010))) { throw "Wrong BAT failure classification for $code" }
        } finally { $process.Dispose() }
    }
    # Early prerequisite failures share the same final result and no-pause path.
    foreach ($missing in @('install-windows.ps1', 'powershell.exe')) {
        $original = Join-Path $fixtureRoot $missing
        $saved = Join-Path $fixtureRoot ($missing + '.saved')
        [IO.File]::Move($original, $saved)
        try {
            $info = [Diagnostics.ProcessStartInfo]::new()
            $info.FileName = Join-Path ([Environment]::SystemDirectory) 'cmd.exe'
            $info.Arguments = '/d /c install-windows.bat'
            $info.WorkingDirectory = $fixtureRoot
            $info.UseShellExecute = $false
            $info.CreateNoWindow = $true
            $info.EnvironmentVariables['HACO_INSTALL_NO_PAUSE'] = '1'
            $info.EnvironmentVariables['PATH'] = [Environment]::SystemDirectory
            $info.RedirectStandardOutput = $info.RedirectStandardError = $true
            $process = [Diagnostics.Process]::Start($info)
            try {
                $stdout = $process.StandardOutput.ReadToEndAsync()
                $stderr = $process.StandardError.ReadToEndAsync()
                if (-not $process.WaitForExit(10000)) { $process.Kill(); throw 'Early BAT failure paused' }
                $output = $stdout.GetAwaiter().GetResult() + $stderr.GetAwaiter().GetResult()
                if ($process.ExitCode -ne 1 -or -not $output.Contains('installation failed with exit code 1') -or $output.Contains('Press any key')) { throw "Early failure result lost: $missing" }
            } finally { $process.Dispose() }
        } finally { [IO.File]::Move($saved, $original) }
    }

    # PAUSE reads the console; redirected pipes do not model a Windows terminal.
    [IO.File]::WriteAllText((Join-Path $fixtureRoot 'native-exit.txt'), '37')
    if ($PausePython) {
        & $PausePython (Join-Path $PSScriptRoot 'test_windows_installer_pause.py') $fixtureRoot
        if ($LASTEXITCODE -ne 0) { throw 'ConPTY installer final-wait test failed' }
    } else {
        Write-Host 'SKIP: final keypress requires -PausePython with pywinpty; exit and prerequisite checks still run.'
    }
    # A live native process holds this otherwise empty working directory. Its
    # eventual release must succeed, while unrelated deletion errors stay errors.
    $heldDirectory = Join-Path $fixtureRoot 'held-directory'
    [void][IO.Directory]::CreateDirectory($heldDirectory)
    $hold = [Diagnostics.ProcessStartInfo]::new()
    $hold.FileName = Join-Path $fixtureRoot 'powershell.exe'
    $hold.Arguments = 'hold'
    $hold.WorkingDirectory = $heldDirectory
    $hold.UseShellExecute = $false
    $hold.CreateNoWindow = $true
    $holdingProcess = [Diagnostics.Process]::Start($hold)
    try {
        $readyFile = Join-Path $fixtureRoot 'hold-ready.txt'
        $readyDeadline = [DateTime]::UtcNow.AddSeconds(15)
        while (-not [IO.File]::Exists($readyFile) -and [DateTime]::UtcNow -lt $readyDeadline -and -not $holdingProcess.HasExited) { Start-Sleep -Milliseconds 50 }
        if (-not [IO.File]::Exists($readyFile)) { throw 'Native sharing fixture did not become ready' }
        $sharingObserved = $false
        try { [IO.Directory]::Delete($heldDirectory, $false) }
        catch {
            $cause = $_.Exception
            while ($null -ne $cause.InnerException) { $cause = $cause.InnerException }
            $sharingObserved = ($cause -is [IO.IOException] -and ($cause.HResult -band 0xffff) -in @(32,33))
        }
        if (-not $sharingObserved) { throw 'Native working-directory sharing fixture not established' }
        [IO.File]::WriteAllText((Join-Path $fixtureRoot 'release-hold.txt'), 'release')
        Remove-ExitFixtureDirectory $heldDirectory
        if (-not $holdingProcess.WaitForExit(10000)) { throw 'Native sharing fixture did not exit' }
        if ($holdingProcess.ExitCode -ne 0) { throw 'Native sharing fixture failed' }
        if ([IO.Directory]::Exists($heldDirectory)) { throw 'Shared fixture was not removed after release' }
    } finally {
        if (-not $holdingProcess.HasExited) { $holdingProcess.Kill(); [void]$holdingProcess.WaitForExit(10000) }
        $holdingProcess.Dispose()
        if ([IO.Directory]::Exists($heldDirectory)) { Remove-ExitFixtureDirectory $heldDirectory }
    }
    $notEmpty = Join-Path $fixtureRoot 'nonempty-directory'
    [void][IO.Directory]::CreateDirectory($notEmpty)
    $retained = Join-Path $notEmpty 'retained'
    [IO.File]::WriteAllText($retained, 'preserve')
    try {
        $rejected = $false
        try { Remove-ExitFixtureDirectory $notEmpty } catch { $rejected = $true }
        if (-not $rejected -or [IO.File]::ReadAllText($retained) -cne 'preserve') { throw 'Nonempty directory was not preserved and refused' }
    } finally {
        [IO.File]::Delete($retained)
        Remove-ExitFixtureDirectory $notEmpty
    }
} finally {
    foreach ($file in $files) { [IO.File]::Delete((Join-Path $fixtureRoot $file)) }
    Remove-ExitFixtureDirectory $fixtureRoot
}
$global:LASTEXITCODE = 0
Write-Host 'WINDOWS BAT EXIT PROPAGATION OK'
