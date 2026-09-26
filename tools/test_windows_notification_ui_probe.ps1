#Requires -Version 5.1
param(
    [Parameter(Mandatory=$true)]
    [string]$OutputPath
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$token = [guid]::NewGuid().ToString('N')
$suffix = $token.Substring(0, 8)
$appId = "Hacocoon.NotificationUIProbe.$token"
$title = "Hacocoon UI Probe $suffix"
$allowLabel = "Allow $suffix"
$denyLabel = "Deny $suffix"
$group = 'HacocoonNotificationUIProbe'
$tag = $suffix
$registryPath = "Software\Classes\AppUserModelId\$appId"
$started = [Diagnostics.Stopwatch]::StartNew()

$result = [ordered]@{
    version = 1
    supported = $false
    stage = 'start'
    session_id = (Get-Process -Id $PID).SessionId
    explorer_in_session = $false
    notifier_setting = $null
    history_visible = $false
    title_visible = $false
    allow_visible = $false
    deny_visible = $false
    allow_invoke_pattern = $false
    deny_invoke_pattern = $false
    duration_ms = 0
}

function Find-UIElementByName([string]$Name) {
    $condition = [System.Windows.Automation.PropertyCondition]::new(
        [System.Windows.Automation.AutomationElement]::NameProperty,
        $Name
    )
    return [System.Windows.Automation.AutomationElement]::RootElement.FindFirst(
        [System.Windows.Automation.TreeScope]::Descendants,
        $condition
    )
}

function Supports-Invoke([System.Windows.Automation.AutomationElement]$Element) {
    if ($null -eq $Element) { return $false }
    $pattern = $null
    return $Element.TryGetCurrentPattern(
        [System.Windows.Automation.InvokePattern]::Pattern,
        [ref]$pattern
    )
}

$exitCode = 1
try {
    $result.stage = 'session'
    $sessionId = $result.session_id
    $result.explorer_in_session = @(
        Get-Process explorer -ErrorAction SilentlyContinue |
            Where-Object { $_.SessionId -eq $sessionId }
    ).Count -gt 0

    $result.stage = 'register'
    $classes = [Microsoft.Win32.Registry]::CurrentUser.CreateSubKey($registryPath)
    if ($null -eq $classes) { throw 'registration unavailable' }
    try {
        $classes.SetValue('DisplayName', 'Hacocoon notification UI probe')
    } finally {
        $classes.Dispose()
    }

    $result.stage = 'show'
    [Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime] > $null
    [Windows.UI.Notifications.ToastNotification,Windows.UI.Notifications,ContentType=WindowsRuntime] > $null
    [Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom,ContentType=WindowsRuntime] > $null

    $xmlText = @"
<toast duration="long">
  <visual>
    <binding template="ToastGeneric">
      <text>$title</text>
      <text>GitHub-hosted Windows UI Automation feasibility probe</text>
    </binding>
  </visual>
  <actions>
    <action content="$allowLabel" arguments="allow" />
    <action content="$denyLabel" arguments="deny" />
  </actions>
</toast>
"@

    $xml = New-Object Windows.Data.Xml.Dom.XmlDocument
    $xml.LoadXml($xmlText)
    $toast = New-Object Windows.UI.Notifications.ToastNotification $xml
    $toast.Tag = $tag
    $toast.Group = $group
    $toast.ExpirationTime = [DateTimeOffset]::Now.AddMinutes(2)

    $notifier = [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($appId)
    $result.notifier_setting = [int]$notifier.Setting
    if ($result.notifier_setting -ne 0) { throw 'notifications disabled' }
    $notifier.Show($toast)

    $result.stage = 'history'
    $historyDeadline = [DateTime]::UtcNow.AddSeconds(10)
    while ([DateTime]::UtcNow -lt $historyDeadline) {
        $items = @([Windows.UI.Notifications.ToastNotificationManager]::History.GetHistory($appId))
        if (@($items | Where-Object { $_.Tag -eq $tag -and $_.Group -eq $group }).Count -eq 1) {
            $result.history_visible = $true
            break
        }
        Start-Sleep -Milliseconds 250
    }
    if (-not $result.history_visible) { throw 'toast history unavailable' }

    $result.stage = 'uia'
    $uiaDeadline = [DateTime]::UtcNow.AddSeconds(15)
    $titleElement = $null
    $allowElement = $null
    $denyElement = $null

    while ([DateTime]::UtcNow -lt $uiaDeadline) {
        if ($null -eq $titleElement) { $titleElement = Find-UIElementByName $title }
        if ($null -eq $allowElement) { $allowElement = Find-UIElementByName $allowLabel }
        if ($null -eq $denyElement) { $denyElement = Find-UIElementByName $denyLabel }

        if ($null -ne $titleElement -and $null -ne $allowElement -and $null -ne $denyElement) {
            break
        }
        Start-Sleep -Milliseconds 250
    }

    $result.title_visible = $null -ne $titleElement
    $result.allow_visible = $null -ne $allowElement
    $result.deny_visible = $null -ne $denyElement
    $result.allow_invoke_pattern = Supports-Invoke $allowElement
    $result.deny_invoke_pattern = Supports-Invoke $denyElement

    if (-not $result.title_visible) { throw 'toast title not visible through UI Automation' }
    if (-not $result.allow_visible -or -not $result.deny_visible) { throw 'toast actions not visible through UI Automation' }
    if (-not $result.allow_invoke_pattern -or -not $result.deny_invoke_pattern) { throw 'toast actions do not expose InvokePattern' }

    $result.stage = 'complete'
    $result.supported = $true
    $exitCode = 0
} catch {
    Write-Host ("Notification UI probe stopped at stage=" + $result.stage + "; exception_type=" + $_.Exception.GetType().FullName)
} finally {
    try {
        [Windows.UI.Notifications.ToastNotificationManager]::History.RemoveGroup($group, $appId)
    } catch {
        Write-Host 'Notification history cleanup unavailable.'
    }
    try {
        [Microsoft.Win32.Registry]::CurrentUser.DeleteSubKeyTree($registryPath, $false)
    } catch {
        Write-Host 'Notification registry cleanup unavailable.'
    }

    $started.Stop()
    $result.duration_ms = [int64]$started.ElapsedMilliseconds
    $directory = Split-Path -Parent $OutputPath
    if ($directory -and -not (Test-Path -LiteralPath $directory)) {
        [void][IO.Directory]::CreateDirectory($directory)
    }
    [IO.File]::WriteAllText(
        $OutputPath,
        (($result | ConvertTo-Json -Depth 4 -Compress) + [Environment]::NewLine),
        [Text.UTF8Encoding]::new($false)
    )
    Write-Host (Get-Content -Raw -LiteralPath $OutputPath)
}

exit $exitCode
