param(
    [int]$BackendPort = 8080,
    [int]$FrontendPort = 5173,
    [int]$TimeoutSec = 90,
    [switch]$DesktopMode,
    [switch]$SkipBackend,
    [switch]$SkipFrontend,
    [switch]$SkipDesktop
)

$ErrorActionPreference = "Stop"

function Step([string]$Message) {
    Write-Host ""
    Write-Host "==> $Message" -ForegroundColor Cyan
}

function Require-Command([string]$Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "Missing required command: $Name"
    }
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

function Kill-ByPort([int]$Port) {
    $listeners = Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $_.LocalPort -eq $Port } |
        Select-Object -ExpandProperty OwningProcess -Unique

    foreach ($pidValue in $listeners) {
        try {
            Stop-Process -Id $pidValue -Force -ErrorAction Stop
            Write-Host "Stopped PID $pidValue (port $Port)"
        } catch {
            Write-Host "Failed to stop PID $pidValue (port $Port): $($_.Exception.Message)" -ForegroundColor Yellow
        }
    }
}

function Kill-ByCommandLine([string[]]$Patterns) {
    $targets = Get-CimInstance Win32_Process |
        Where-Object { $_.CommandLine } |
        Where-Object {
            $cmd = $_.CommandLine.ToLowerInvariant()
            foreach ($p in $Patterns) {
                if ($cmd.Contains($p.ToLowerInvariant())) {
                    return $true
                }
            }
            return $false
        }

    foreach ($proc in $targets) {
        try {
            Stop-Process -Id $proc.ProcessId -Force -ErrorAction Stop
            Write-Host "Stopped PID $($proc.ProcessId): $($proc.Name)"
        } catch {
            Write-Host "Failed to stop PID $($proc.ProcessId): $($_.Exception.Message)" -ForegroundColor Yellow
        }
    }
}

function Wait-Port([int]$Port, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $listener = Get-NetTCPConnection -State Listen -ErrorAction SilentlyContinue |
            Where-Object { $_.LocalPort -eq $Port } |
            Select-Object -First 1
        if ($listener) {
            return $listener
        }
        Start-Sleep -Seconds 1
    }
    return $null
}

$repoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
$backendDir = Join-Path $repoRoot "backend"
$frontendDir = Join-Path $repoRoot "frontend"
$desktopDir = Join-Path $backendDir "desktop"
$logDir = Join-Path $repoRoot "logs"

if (-not (Test-Path $logDir)) {
    New-Item -Path $logDir -ItemType Directory | Out-Null
}

if ($DesktopMode -and -not $SkipDesktop) {
    Step "Stopping existing desktop/backend/frontend processes"
    Kill-ByCommandLine -Patterns @("wails dev", "\backend\desktop")
    Kill-ByPort -Port $BackendPort
    Kill-ByPort -Port $FrontendPort
    Kill-ByCommandLine -Patterns @("assessv2-server-dev.exe", "go run ./cmd/server", "\backend\cmd\server", "npm run dev -- --host 127.0.0.1 --port $FrontendPort", "vite.js --host 127.0.0.1 --port $FrontendPort")

    Step "Starting desktop app (Wails dev)"
    $wailsCommand = Resolve-WailsCommand

    $desktopLog = Join-Path $logDir "desktop-dev.log"
    Start-Process -FilePath "cmd.exe" `
        -ArgumentList "/c", ('"{0}" dev > ..\..\logs\desktop-dev.log 2>&1' -f $wailsCommand) `
        -WorkingDirectory $desktopDir `
        -WindowStyle Hidden | Out-Null

    Start-Sleep -Seconds 3
    $desktopProc = Get-CimInstance Win32_Process |
        Where-Object { $_.CommandLine -and $_.CommandLine.ToLowerInvariant().Contains("wails dev") } |
        Select-Object -First 1
    if (-not $desktopProc) {
        throw "Desktop process not detected after startup. Check $desktopLog"
    }

    Write-Host "Desktop dev running (PID $($desktopProc.ProcessId))"
    Write-Host "Desktop log: $desktopLog"
    Step "Restart completed"
    return
}

if (-not $SkipBackend) {
    Step "Stopping existing backend processes"
    Kill-ByPort -Port $BackendPort
    Kill-ByCommandLine -Patterns @("assessv2-server-dev.exe", "go run ./cmd/server", "\backend\cmd\server")

    Step "Starting backend service"
    $backendLog = Join-Path $logDir "backend-dev.log"
    Start-Process -FilePath "cmd.exe" `
        -ArgumentList "/c", "go run ./cmd/server > ..\logs\backend-dev.log 2>&1" `
        -WorkingDirectory $backendDir `
        -WindowStyle Hidden | Out-Null

    $backendListener = Wait-Port -Port $BackendPort -TimeoutSeconds $TimeoutSec
    if (-not $backendListener) {
        throw "Backend start timeout: port $BackendPort not listening in $TimeoutSec seconds. Check $backendLog"
    }
    Write-Host "Backend listening on 127.0.0.1:$BackendPort (PID $($backendListener.OwningProcess))"
    Write-Host "Backend log: $backendLog"
}

if (-not $SkipFrontend) {
    Step "Stopping existing frontend processes"
    Kill-ByPort -Port $FrontendPort
    Kill-ByCommandLine -Patterns @("npm run dev -- --host 127.0.0.1 --port $FrontendPort", "vite.js --host 127.0.0.1 --port $FrontendPort")

    Step "Starting frontend service"
    $frontendLog = Join-Path $logDir "frontend-dev.log"
    Start-Process -FilePath "cmd.exe" `
        -ArgumentList "/c", "npm run dev -- --host 127.0.0.1 --port $FrontendPort > ..\logs\frontend-dev.log 2>&1" `
        -WorkingDirectory $frontendDir `
        -WindowStyle Hidden | Out-Null

    $frontendListener = Wait-Port -Port $FrontendPort -TimeoutSeconds $TimeoutSec
    if (-not $frontendListener) {
        throw "Frontend start timeout: port $FrontendPort not listening in $TimeoutSec seconds. Check $frontendLog"
    }
    Write-Host "Frontend listening on http://127.0.0.1:$FrontendPort (PID $($frontendListener.OwningProcess))"
    Write-Host "Frontend log: $frontendLog"
}

Step "Restart completed"
