param(
    [switch]$RunE2E
)

$ErrorActionPreference = "Stop"

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

Step "Initialize database schema and baseline data"
Push-Location "backend"
try {
    go run ./cmd/migrate -action up -seed=true
} finally {
    Pop-Location
}

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

Step "M0 acceptance checks passed"
