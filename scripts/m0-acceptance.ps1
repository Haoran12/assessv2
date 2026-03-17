param(
    [switch]$RunE2E,
    [switch]$SkipDesktop
)

$ErrorActionPreference = "Stop"

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

Step "Initialize database schema and baseline data"
Push-Location "backend"
try {
    go run ./cmd/migrate -action up -target all -seed=true
} finally {
    Pop-Location
}

Step "Run schema gate checks"
& (Join-Path $PSScriptRoot "schema-gate.ps1")

Step "Run backend tests (unit + API integration)"
Push-Location "backend"
try {
    go test ./...
} finally {
    Pop-Location
}

Step "Run frontend unit tests"
Push-Location "frontend"
try {
    if (-not (Test-Path "node_modules")) {
        npm ci
    }
    npm run test:unit

    if ($RunE2E) {
        Step "Run frontend E2E smoke tests"
        npm run test:e2e
    }
} finally {
    Pop-Location
}

if (-not $SkipDesktop) {
    Step "Run desktop compile check (Wails shell)"
    Push-Location "backend/desktop"
    try {
        go build ./...
    } finally {
        Pop-Location
    }
}

Step "M0 acceptance checks passed"
