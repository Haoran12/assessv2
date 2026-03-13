param(
    [ValidateSet("dev", "release")]
    [string]$Mode = "dev",
    [switch]$SkipTests,
    [switch]$SkipBackend,
    [switch]$SkipFrontend,
    [switch]$SkipDesktop,
    [switch]$SkipTauri
)

$ErrorActionPreference = "Stop"

function Write-Step {
    param([string]$Message)
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Require-Command {
    param([string]$Name)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command: $Name"
    }
}

function Invoke-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )
    Write-Step $Name
    & $Action
}

$SkipDesktopEffective = $SkipDesktop -or $SkipTauri

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

function Sync-DesktopRuntimeAssets {
    param(
        [string]$DesktopDir
    )

    $desktopBinDir = Join-Path $DesktopDir "build/bin"
    New-Item -ItemType Directory -Path $desktopBinDir -Force | Out-Null

    $migrationsSource = Resolve-Path (Join-Path $DesktopDir "../migrations")
    $migrationsTarget = Join-Path $desktopBinDir "migrations"
    New-Item -ItemType Directory -Path $migrationsTarget -Force | Out-Null
    Copy-Item -Path (Join-Path $migrationsSource "*.sql") -Destination $migrationsTarget -Force
    Write-Host "Output: backend/desktop/build/bin/migrations"

    $frontendDistDir = Resolve-Path (Join-Path $DesktopDir "../../frontend/dist")
    $frontendTargetDir = Join-Path $desktopBinDir "frontend/dist"
    New-Item -ItemType Directory -Path $frontendTargetDir -Force | Out-Null
    Copy-Item -Path (Join-Path $frontendDistDir "*") -Destination $frontendTargetDir -Recurse -Force
    Write-Host "Output: backend/desktop/build/bin/frontend/dist"
}

Invoke-Step "Build mode: $Mode" {
    Write-Host "SkipTests=$SkipTests SkipBackend=$SkipBackend SkipFrontend=$SkipFrontend SkipDesktop=$SkipDesktopEffective"
    if ($SkipTauri) {
        Write-Host "SkipTauri is deprecated and mapped to SkipDesktop." -ForegroundColor Yellow
    }
}

if (-not $SkipBackend) {
    Require-Command "go"
    Push-Location "backend"
    try {
        if (-not $SkipTests) {
            Invoke-Step "Backend tests" {
                go test ./...
            }
        }

        Invoke-Step "Backend build" {
            New-Item -ItemType Directory -Path "bin" -Force | Out-Null
            $outputName = if ($Mode -eq "release") { "assessv2-server.exe" } else { "assessv2-server-dev.exe" }
            go build -o ("bin/" + $outputName) ./cmd/server
            Write-Host "Output: backend/bin/$outputName"
        }
    } finally {
        Pop-Location
    }
}

if (-not $SkipFrontend) {
    Require-Command "npm"
    Push-Location "frontend"
    try {
        if (-not (Test-Path "node_modules")) {
            Invoke-Step "Frontend install" {
                npm ci
            }
        }

        if (-not $SkipTests) {
            Invoke-Step "Frontend unit tests" {
                npm run test:unit
            }
        }

        Invoke-Step "Frontend build" {
            npm run build
            Write-Host "Output: frontend/dist"
        }
    } finally {
        Pop-Location
    }
}

if (-not $SkipDesktopEffective) {
    Require-Command "go"
    Push-Location "backend/desktop"
    try {
        if ($Mode -eq "release") {
            $wailsCommand = Resolve-WailsCommand
            Invoke-Step "Desktop release build (wails build -clean -s)" {
                & $wailsCommand build -clean -s
                Write-Host "Output: backend/desktop/build/bin"
            }
        } else {
            Invoke-Step "Desktop compile check (go build)" {
                New-Item -ItemType Directory -Path "build/bin" -Force | Out-Null
                go build -o "build/bin/assessv2-desktop-dev.exe" .
                Write-Host "Output: backend/desktop/build/bin/assessv2-desktop-dev.exe"
            }
        }

        Invoke-Step "Sync desktop runtime assets (frontend dist + migrations)" {
            Sync-DesktopRuntimeAssets -DesktopDir (Get-Location).Path
        }
    } finally {
        Pop-Location
    }
}

Write-Step "Build completed successfully"
