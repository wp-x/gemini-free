param(
    [string]$Source,
    [switch]$Headless
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
[Console]::OutputEncoding = [Text.Encoding]::UTF8
$startupTimeoutSeconds = 25
$requestTimeoutSeconds = 5
$pollMilliseconds = 200
$exitTimeoutMilliseconds = 3000
if (!$Source) {
    $Source = Join-Path $PSScriptRoot "..\dist\windows-amd64\Gemini Free.exe"
}
if (!(Test-Path -LiteralPath $Source -PathType Leaf)) {
    throw "Windows executable source does not exist"
}
$taskDirectory = Join-Path $env:TEMP ("GeminiFreeValidation-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $taskDirectory | Out-Null
$executable = Join-Path $taskDirectory "GeminiFree.exe"
$configPath = Join-Path $taskDirectory "config.json"
$controlPath = Join-Path $taskDirectory "control.json"
Copy-Item -LiteralPath $Source -Destination $executable
$arguments = @("-config", "`"$configPath`"", "-no-start", "-control-file", "`"$controlPath`"")
if ($Headless) { $arguments += "-headless" }
$process = $null
$result = $null
try {
    $process = Start-Process -FilePath $executable -WorkingDirectory $taskDirectory -ArgumentList $arguments `
        -RedirectStandardOutput (Join-Path $taskDirectory "stdout.log") `
        -RedirectStandardError (Join-Path $taskDirectory "stderr.log") -PassThru
    $deadline = [DateTime]::UtcNow.AddSeconds($startupTimeoutSeconds)
    while (!(Test-Path -LiteralPath $controlPath)) {
        $process.Refresh()
        if ($process.HasExited) { throw "Application exited before management became ready (exit $($process.ExitCode))" }
        if ([DateTime]::UtcNow -gt $deadline) { throw "Management startup timed out after $startupTimeoutSeconds seconds" }
        Start-Sleep -Milliseconds $pollMilliseconds
    }
    $control = Get-Content -LiteralPath $controlPath -Raw | ConvertFrom-Json
    $uri = [Uri]$control.url
    $base = $uri.GetLeftPart([UriPartial]::Authority)
    $headers = @{ "X-Desktop-Token" = $uri.Fragment.Substring(1) }
    $state = Invoke-RestMethod -Uri "$base/control/state" -Headers $headers -TimeoutSec $requestTimeoutSeconds
    $process.Refresh()
    if ($process.HasExited) { throw "Application exited after management startup" }
    $windowDeadline = [DateTime]::UtcNow.AddSeconds($requestTimeoutSeconds)
    while (!$Headless -and $process.MainWindowHandle -eq 0 -and [DateTime]::UtcNow -lt $windowDeadline) {
        Start-Sleep -Milliseconds $pollMilliseconds
        $process.Refresh()
    }
    if (!$Headless -and $process.MainWindowHandle -eq 0) { throw "Management is ready but no native application window was found" }
    if (!$Headless -and !$process.Responding) { throw "Native application window is not responding" }
    $result = [PSCustomObject]@{
        process_running_at_probe = !$process.HasExited
        process_responding = $process.Responding
        native_window_found = $process.MainWindowHandle -ne 0
        main_window = $process.MainWindowTitle
        headless = [bool]$Headless
        mode = $state.config.mode
        model_count = $state.models.Count
        service_running = $state.service.running
        account_count = $state.accounts.Count
        executable_sha256 = (Get-FileHash -LiteralPath $executable -Algorithm SHA256).Hash.ToLowerInvariant()
        evidence_directory = $taskDirectory
    }
}
finally {
    if ($process -and !$process.HasExited) {
        Stop-Process -Id $process.Id -Force
        if (!$process.WaitForExit($exitTimeoutMilliseconds)) { throw "Validation process did not terminate" }
    }
    if (Test-Path -LiteralPath $controlPath) { Remove-Item -LiteralPath $controlPath -Force }
}
$result | Add-Member -NotePropertyName "validation_process_cleaned" -NotePropertyValue $true
$result | ConvertTo-Json -Compress
