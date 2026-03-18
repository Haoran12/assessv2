param(
    [string]$DataRoot = "",
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
if ([string]::IsNullOrWhiteSpace($DataRoot)) {
    $DataRoot = Join-Path $repoRoot "data"
} else {
    $resolved = Resolve-Path -LiteralPath $DataRoot -ErrorAction SilentlyContinue
    if ($resolved) {
        $DataRoot = $resolved.Path
    }
}

Step "Prepare data structure migration"
Write-Host "Data root: $DataRoot"
Write-Host "Mode     : $(if ($DryRun) { 'dry-run (no write)' } else { 'apply (with backup)' })"
Write-Host "Note     : please close desktop/server process before apply mode to avoid file lock issues."

Push-Location (Join-Path $repoRoot "backend")
try {
    $args = @("./cmd/migrate-data", "-data-root", $DataRoot)
    if ($DryRun) {
        $args += "-dry-run"
    }
    go run @args
    if ($LASTEXITCODE -ne 0) {
        throw "cmd/migrate-data failed with exit code $LASTEXITCODE"
    }
}
finally {
    Pop-Location
}

Step "Data structure migration completed"
