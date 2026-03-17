param(
    [string]$BusinessDBPath = "",
    [string]$AccountsDBPath = "",
    [string]$BackupDir = "",
    [switch]$SkipBackup
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Resolve-DefaultPath([string]$Candidate, [string]$Fallback) {
    if ([string]::IsNullOrWhiteSpace($Candidate)) {
        return $Fallback
    }
    $resolved = Resolve-Path -LiteralPath $Candidate -ErrorAction SilentlyContinue
    if ($resolved) {
        return $resolved.Path
    }
    return $Candidate
}

function Copy-SqliteBundleIfExists {
    param(
        [Parameter(Mandatory = $true)][string]$SourceMain,
        [Parameter(Mandatory = $true)][string]$TargetMain
    )

    if (-not (Test-Path $SourceMain -PathType Leaf)) {
        return
    }
    New-Item -ItemType Directory -Path (Split-Path -Parent $TargetMain) -Force | Out-Null
    Copy-Item -Path $SourceMain -Destination $TargetMain -Force
    foreach ($suffix in @("-wal", "-shm")) {
        $sourceSidecar = "$SourceMain$suffix"
        if (Test-Path $sourceSidecar -PathType Leaf) {
            Copy-Item -Path $sourceSidecar -Destination "$TargetMain$suffix" -Force
        }
    }
}

function Remove-SqliteBundleIfExists {
    param([Parameter(Mandatory = $true)][string]$MainPath)

    foreach ($path in @($MainPath, "$MainPath-wal", "$MainPath-shm")) {
        if (Test-Path $path -PathType Leaf) {
            Remove-Item -Path $path -Force
        }
    }
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$defaultBusinessPath = Join-Path $repoRoot "data/assess.db"
$defaultAccountsPath = Join-Path $repoRoot "data/accounts/accounts.db"
$BusinessDBPath = Resolve-DefaultPath -Candidate $BusinessDBPath -Fallback $defaultBusinessPath
$AccountsDBPath = Resolve-DefaultPath -Candidate $AccountsDBPath -Fallback $defaultAccountsPath

if ([string]::IsNullOrWhiteSpace($BackupDir)) {
    $BackupDir = Join-Path $repoRoot "data/rebuild-backups"
}

Step "Rebuild parameters"
Write-Host "Business DB : $BusinessDBPath"
Write-Host "Accounts DB : $AccountsDBPath"
Write-Host "Backup dir  : $BackupDir"

if (-not $SkipBackup) {
    $stamp = Get-Date -Format "yyyyMMddHHmmss"
    $snapshotDir = Join-Path $BackupDir $stamp
    Step "Backup existing database files"
    Copy-SqliteBundleIfExists -SourceMain $BusinessDBPath -TargetMain (Join-Path $snapshotDir "business/assess.db")
    Copy-SqliteBundleIfExists -SourceMain $AccountsDBPath -TargetMain (Join-Path $snapshotDir "accounts/accounts.db")
    Write-Host "Backup snapshot: $snapshotDir"
}

Step "Delete current databases"
Remove-SqliteBundleIfExists -MainPath $BusinessDBPath
Remove-SqliteBundleIfExists -MainPath $AccountsDBPath

Step "Initialize clean split schema + seed baseline"
Push-Location (Join-Path $repoRoot "backend")
try {
    $env:ASSESS_SQLITE_PATH = $BusinessDBPath
    $env:ASSESS_ACCOUNTS_SQLITE_PATH = $AccountsDBPath
    go run ./cmd/migrate -action up -target all -seed=true
    if ($LASTEXITCODE -ne 0) {
        throw "cmd/migrate failed with exit code $LASTEXITCODE"
    }

    Step "Run schema audit gate"
    go run ./cmd/schema-audit
    if ($LASTEXITCODE -ne 0) {
        throw "cmd/schema-audit failed with exit code $LASTEXITCODE"
    }
} finally {
    Pop-Location
}

Step "Rebuild completed"
