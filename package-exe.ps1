param(
    [switch]$SkipTests,
    [switch]$SkipFrontendBuild,
    [switch]$SkipNpmInstall,
    [switch]$Clean
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Invoke-External {
    param(
        [Parameter(Mandatory = $true)]
        [string]$FilePath,
        [Parameter(ValueFromRemainingArguments = $true)]
        [string[]]$Arguments
    )

    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed ($LASTEXITCODE): $FilePath $($Arguments -join ' ')"
    }
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command: $Name"
    }
}

function Resolve-NpmCommand {
    $npmCmd = Get-Command "npm.cmd" -ErrorAction SilentlyContinue
    if ($npmCmd) {
        return $npmCmd.Source
    }

    $npm = Get-Command "npm" -ErrorAction SilentlyContinue
    if ($npm) {
        return $npm.Source
    }

    throw "Missing required command: npm (or npm.cmd)"
}

function Resolve-WailsCommand {
    $wailsCmd = Get-Command "wails" -ErrorAction SilentlyContinue
    if ($wailsCmd) {
        return $wailsCmd.Source
    }

    $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
    if ($goCmd) {
        $goPath = (& go env GOPATH).Trim()
        if ($goPath) {
            $candidate = Join-Path $goPath "bin/wails.exe"
            if (Test-Path $candidate) {
                return $candidate
            }
        }
    }

    throw "Missing required command: wails (or `%GOPATH%\\bin\\wails.exe`)"
}

function Test-FilePath {
    param([string]$Path)
    return (Test-Path -Path $Path -PathType Leaf)
}

function Copy-SqliteBundle {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SourceMain,
        [Parameter(Mandatory = $true)]
        [string]$TargetMain
    )

    New-Item -ItemType Directory -Path (Split-Path -Parent $TargetMain) -Force | Out-Null
    Copy-Item -Path $SourceMain -Destination $TargetMain -Force
    foreach ($suffix in @("-wal", "-shm")) {
        $source = "$SourceMain$suffix"
        if (Test-FilePath $source) {
            Copy-Item -Path $source -Destination "$TargetMain$suffix" -Force
        }
    }
}

function Move-SqliteBundle {
    param(
        [Parameter(Mandatory = $true)]
        [string]$SourceMain,
        [Parameter(Mandatory = $true)]
        [string]$TargetMain
    )

    New-Item -ItemType Directory -Path (Split-Path -Parent $TargetMain) -Force | Out-Null
    Move-Item -Path $SourceMain -Destination $TargetMain -Force
    foreach ($suffix in @("-wal", "-shm")) {
        $source = "$SourceMain$suffix"
        if (Test-FilePath $source) {
            Move-Item -Path $source -Destination "$TargetMain$suffix" -Force
        }
    }
}

function Resolve-PreferredYear {
    param([string]$DataRoot)

    $preferredFile = Join-Path $DataRoot ".assessment_year"
    if (Test-FilePath $preferredFile) {
        $raw = (Get-Content -Path $preferredFile -Raw).Trim()
        if ($raw -match '^\d{4}$') {
            return [int]$raw
        }
    }

    $yearDirs = @(
        Get-ChildItem -Path $DataRoot -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -match '^\d{4}$' } |
            Sort-Object { [int]$_.Name }
    )
    if ($yearDirs.Count -gt 0) {
        return [int]$yearDirs[-1].Name
    }

    return (Get-Date).Year
}

function Repair-LegacyDataLayout {
    param([string]$DataRoot)

    if (-not (Test-Path $DataRoot -PathType Container)) {
        return
    }

    $year = Resolve-PreferredYear -DataRoot $DataRoot
    $yearDir = Join-Path $DataRoot $year
    New-Item -ItemType Directory -Path $yearDir -Force | Out-Null

    $legacyAssessMain = Join-Path $DataRoot "assess.db"
    $yearAssessMain = Join-Path $yearDir "assess.db"
    if (Test-FilePath $legacyAssessMain) {
        if (-not (Test-FilePath $yearAssessMain)) {
            Move-SqliteBundle -SourceMain $legacyAssessMain -TargetMain $yearAssessMain
            Write-Host "Migrated legacy assessment DB to yearly layout: data\\$year\\assess.db"
        } else {
            $backupDir = Join-Path $DataRoot ("legacy\\" + (Get-Date -Format "yyyyMMddHHmmss"))
            New-Item -ItemType Directory -Path $backupDir -Force | Out-Null
            Move-SqliteBundle -SourceMain $legacyAssessMain -TargetMain (Join-Path $backupDir "assess.db")
            Write-Host "Moved duplicate legacy assessment DB to backup: $backupDir"
        }
    }

    $accountsDir = Join-Path $DataRoot "accounts"
    $accountsMain = Join-Path $accountsDir "accounts.db"
    New-Item -ItemType Directory -Path $accountsDir -Force | Out-Null

    if (-not (Test-FilePath $accountsMain)) {
        $legacyAccountsMain = Join-Path $DataRoot "accounts.db"
        if (Test-FilePath $legacyAccountsMain) {
            Move-SqliteBundle -SourceMain $legacyAccountsMain -TargetMain $accountsMain
            Write-Host "Migrated legacy accounts DB to: data\\accounts\\accounts.db"
        } elseif (Test-FilePath $yearAssessMain) {
            Copy-SqliteBundle -SourceMain $yearAssessMain -TargetMain $accountsMain
            Write-Host "Bootstrapped shared accounts DB from yearly DB: data\\accounts\\accounts.db"
        }
    }
}

function Sync-DesktopEmbeddedAssets {
    param(
        [string]$ProjectRoot
    )

    $desktopDir = Join-Path $ProjectRoot "backend/desktop"

    $migrationsSource = Resolve-Path (Join-Path $ProjectRoot "backend/migrations")
    $migrationsTarget = Join-Path $desktopDir "runtime/migrations"
    if (Test-Path $migrationsTarget) {
        Remove-Item -Path $migrationsTarget -Recurse -Force
    }
    New-Item -ItemType Directory -Path $migrationsTarget -Force | Out-Null
    foreach ($domain in @("business", "accounts")) {
        $sourceDir = Join-Path $migrationsSource $domain
        if (-not (Test-Path $sourceDir -PathType Container)) {
            throw "Missing migration domain directory: $sourceDir"
        }
        $targetDir = Join-Path $migrationsTarget $domain
        New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
        Copy-Item -Path (Join-Path $sourceDir "*.sql") -Destination $targetDir -Force
        Set-Content -Path (Join-Path $targetDir ".gitkeep") -Value "" -NoNewline
    }
    Set-Content -Path (Join-Path $migrationsTarget ".gitkeep") -Value "" -NoNewline

    $frontendDist = Resolve-Path (Join-Path $ProjectRoot "frontend/dist")
    $frontendTarget = Join-Path $desktopDir "runtime/frontend/dist"
    if (Test-Path $frontendTarget) {
        Remove-Item -Path $frontendTarget -Recurse -Force
    }
    New-Item -ItemType Directory -Path $frontendTarget -Force | Out-Null
    Copy-Item -Path (Join-Path $frontendDist "*") -Destination $frontendTarget -Recurse -Force
    Set-Content -Path (Join-Path $frontendTarget ".gitkeep") -Value "" -NoNewline
}

function Stop-RunningExeAtPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$ExePath
    )

    if (-not (Test-Path $ExePath)) {
        return
    }

    $normalizedTarget = ([System.IO.Path]::GetFullPath($ExePath)).ToLowerInvariant()
    $targets = Get-CimInstance Win32_Process -Filter "Name='assessv2-desktop.exe'" |
        Where-Object { $_.ExecutablePath } |
        Where-Object {
            try {
                ([System.IO.Path]::GetFullPath($_.ExecutablePath)).ToLowerInvariant() -eq $normalizedTarget
            } catch {
                $false
            }
        }

    foreach ($proc in $targets) {
        try {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction Stop
            Write-Host "Stopped running process before overwrite: PID $($proc.ProcessId)"
        } catch {
            throw "Unable to stop running executable (PID $($proc.ProcessId)): $($_.Exception.Message)"
        }
    }
}

$projectRoot = $PSScriptRoot
$frontendDir = Join-Path $projectRoot "frontend"
$backendDir = Join-Path $projectRoot "backend"
$desktopDir = Join-Path $projectRoot "backend/desktop"
$rootExe = Join-Path $projectRoot "assessv2-desktop.exe"

Write-Step "Checking build toolchain"
Require-Command "go"
Require-Command "node"
$npmCommand = Resolve-NpmCommand
$wailsCommand = Resolve-WailsCommand

if ($Clean) {
    Write-Step "Cleaning previous desktop build outputs"
    $desktopBin = Join-Path $desktopDir "build/bin"
    if (Test-Path $desktopBin) {
        Remove-Item -Path $desktopBin -Recurse -Force
    }
    if (Test-Path $rootExe) {
        Remove-Item -Path $rootExe -Force
    }
}

if (-not $SkipTests) {
    Write-Step "Running backend tests"
    Push-Location $backendDir
    try {
        Invoke-External "go" "test" "./..."
    } finally {
        Pop-Location
    }
}

if (-not $SkipFrontendBuild) {
    Write-Step "Building frontend"
    Push-Location $frontendDir
    try {
        if ((-not $SkipNpmInstall) -and (-not (Test-Path "node_modules"))) {
            Invoke-External $npmCommand "ci"
        }
        if (-not $SkipTests) {
            Invoke-External $npmCommand "run" "test:unit"
        }
        Invoke-External $npmCommand "run" "build"
    } finally {
        Pop-Location
    }
}

Write-Step "Syncing embedded desktop runtime assets"
Sync-DesktopEmbeddedAssets -ProjectRoot $projectRoot

Write-Step "Building desktop release executable"
Push-Location $desktopDir
try {
    Invoke-External $wailsCommand "build" "-clean" "-s"
} finally {
    Pop-Location
}

Write-Step "Publishing executable to project root"
$desktopExe = Join-Path $desktopDir "build/bin/assessv2-desktop.exe"
if (-not (Test-Path $desktopExe)) {
    throw "Desktop executable not found: $desktopExe"
}
Stop-RunningExeAtPath -ExePath $rootExe
Copy-Item -Path $desktopExe -Destination $rootExe -Force

Write-Step "Repairing legacy data directory layout"
Repair-LegacyDataLayout -DataRoot (Join-Path $projectRoot "data")

Write-Step "Done"
Write-Host "EXE: $rootExe" -ForegroundColor Green
Write-Host "Data directory pattern: .\\data\\{assessment_year}\\assess.db" -ForegroundColor Green
Write-Host "Shared accounts DB: .\\data\\accounts\\accounts.db" -ForegroundColor Green
