# Run the shipped BAT against a disposable native PowerShell stand-in. This is
# an exit-propagation component test, not Windows/WSL installation acceptance.
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) ('haco-bat-exit-' + [guid]::NewGuid())
[IO.Directory]::CreateDirectory($fixtureRoot) | Out-Null
$files = @('install-windows.bat', 'install-windows.ps1', 'powershell.exe', 'native-exit.txt')
try {
    [IO.File]::Copy((Join-Path $PSScriptRoot '../scripts/install-windows.bat'), (Join-Path $fixtureRoot 'install-windows.bat'))
    [IO.File]::WriteAllText((Join-Path $fixtureRoot 'install-windows.ps1'), '# Native boundary stand-in only')
    Add-Type -OutputType ConsoleApplication -OutputAssembly (Join-Path $fixtureRoot 'powershell.exe') -TypeDefinition @'
using System;
using System.IO;
class NativeExit {
    static int Main() {
        Console.WriteLine("Native boundary completed");
        string root = Path.GetDirectoryName(typeof(NativeExit).Assembly.Location);
        return int.Parse(File.ReadAllText(Path.Combine(root, "native-exit.txt")));
    }
}
'@
    foreach ($code in @(0, 1, 3010)) {
        [IO.File]::WriteAllText((Join-Path $fixtureRoot 'native-exit.txt'), [string]$code)
        $info = [Diagnostics.ProcessStartInfo]::new()
        $info.FileName = Join-Path ([Environment]::SystemDirectory) 'cmd.exe'
        $info.Arguments = '/d /c install-windows.bat'
        $info.WorkingDirectory = $fixtureRoot
        $info.UseShellExecute = $false
        $info.CreateNoWindow = $true
        $info.RedirectStandardOutput = $info.RedirectStandardError = $true
        $process = [Diagnostics.Process]::Start($info)
        try {
            $stdout = $process.StandardOutput.ReadToEndAsync()
            $stderr = $process.StandardError.ReadToEndAsync()
            if (-not $process.WaitForExit(10000)) { $process.Kill(); throw 'BAT fixture timed out' }
            $output = $stdout.GetAwaiter().GetResult() + $stderr.GetAwaiter().GetResult()
            if ($process.ExitCode -ne $code) { throw "BAT lost native exit $code" }
            if (($output.Contains('Windows installation complete.')) -ne ($code -eq 0)) { throw "False BAT completion for $code" }
            if (($output.Contains('paused until Windows restarts')) -ne ($code -eq 3010)) { throw "Wrong BAT restart classification for $code" }
            if (($output.Contains('installation failed')) -ne ($code -eq 1)) { throw "Wrong BAT failure classification for $code" }
        } finally { $process.Dispose() }
    }
} finally {
    foreach ($file in $files) { [IO.File]::Delete((Join-Path $fixtureRoot $file)) }
    [IO.Directory]::Delete($fixtureRoot)
}
$global:LASTEXITCODE = 0
Write-Host 'WINDOWS BAT EXIT PROPAGATION OK'
