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
    Copy-Item -Path (Join-Path $migrationsSource "*.sql") -Destination $migrationsTarget -Force
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

Write-Step "Done"
Write-Host "EXE: $rootExe" -ForegroundColor Green
Write-Host "Data directory pattern: .\\data\\{assessment_year}\\assess.db" -ForegroundColor Green
