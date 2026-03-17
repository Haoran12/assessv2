param(
    [string]$BusinessDBPath = "",
    [string]$AccountsDBPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$backendDir = Join-Path $repoRoot "backend"

if ([string]::IsNullOrWhiteSpace($BusinessDBPath)) {
    $BusinessDBPath = Join-Path $repoRoot "data/assess.db"
}
if ([string]::IsNullOrWhiteSpace($AccountsDBPath)) {
    $AccountsDBPath = Join-Path $repoRoot "data/accounts/accounts.db"
}

Step "Run runtime schema audit"
Push-Location $backendDir
try {
    $env:ASSESS_SQLITE_PATH = $BusinessDBPath
    $env:ASSESS_ACCOUNTS_SQLITE_PATH = $AccountsDBPath
    go run ./cmd/schema-audit
    if ($LASTEXITCODE -ne 0) {
        throw "schema-audit failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}

Step "Verify checksum reconcile backdoor is absent from startup paths"
$reconcileEnvHits = rg -n "ASSESS_ALLOW_MIGRATION_CHECKSUM_RECONCILE" $repoRoot -g "!scripts/schema-gate.ps1" -g "!backend/cmd/schema-audit/main.go"
if ($LASTEXITCODE -eq 0 -and $reconcileEnvHits) {
    throw "Found forbidden ASSESS_ALLOW_MIGRATION_CHECKSUM_RECONCILE references:`n$reconcileEnvHits"
}

Push-Location $backendDir
try {
    $reconcileCallHits = rg -n -F "ReconcileChecksums(" . -g "!internal/migration/manager.go" -g "!internal/migration/manager_test.go"
    if ($LASTEXITCODE -eq 0 -and $reconcileCallHits) {
        throw "Found forbidden runtime ReconcileChecksums usage:`n$reconcileCallHits"
    }
} finally {
    Pop-Location
}

Step "Schema gate passed"
