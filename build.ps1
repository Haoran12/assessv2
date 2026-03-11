param(
    [ValidateSet("dev", "release")]
    [string]$Mode = "dev",
    [switch]$SkipTests,
    [switch]$SkipBackend,
    [switch]$SkipFrontend,
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

Invoke-Step "Build mode: $Mode" {
    Write-Host "SkipTests=$SkipTests SkipBackend=$SkipBackend SkipFrontend=$SkipFrontend SkipTauri=$SkipTauri"
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

        Invoke-Step "Frontend build" {
            npm run build
            Write-Host "Output: frontend/dist"
        }
    } finally {
        Pop-Location
    }
}

if (-not $SkipTauri) {
    Require-Command "cargo"
    Push-Location "src-tauri"
    try {
        if ($Mode -eq "release") {
            Invoke-Step "Tauri release build (cargo build --release)" {
                cargo build --release
                Write-Host "Output: src-tauri/target/release"
            }
        } else {
            Invoke-Step "Tauri compile check" {
                cargo check
            }
        }
    } finally {
        Pop-Location
    }
}

Write-Step "Build completed successfully"
