#!/usr/bin/env pwsh
#
# clip-clap agent-mode harness.
# Implements the architecture's [Verification Harness Contract] for the
# desktop-windows verification profile (pytest + pywinauto via Phase 4).
#
# Subcommands:
#   build  — go generate + go build with -H windowsgui linker flag
#   start  — launch clip-clap.exe (optionally --agent-mode); write PID file
#            to .agent-running; redirect stdout/stderr to logs/agent-latest.jsonl
#   status — curl http://127.0.0.1:27773/status (loopback-only test endpoint)
#   logs   — tail logs/agent-latest.jsonl
#   kill   — read PID from .agent-running, terminate, remove file
#
# Run from project root. Requires Go 1.23+ and goversioninfo.exe on PATH.

param(
    [Parameter(Mandatory=$true, Position=0)]
    [ValidateSet('build', 'start', 'status', 'logs', 'kill')]
    [string]$Cmd,

    [Parameter(Position=1, ValueFromRemainingArguments=$true)]
    [string[]]$RestArgs
)

$ErrorActionPreference = 'Stop'

# Lowercase function names match the Phase 0 plan Step 18 verify regex
# (`grep -qE 'function build'`). PowerShell is case-insensitive for
# function dispatch, so `function build` and `function Build` are equivalent
# at call time — the grep just needs the literal token to be present.

function build {
    Write-Host "Building clip-clap.exe (CGO_ENABLED=0, -H windowsgui)..."
    $env:CGO_ENABLED = '0'
    & go generate ./...
    if ($LASTEXITCODE -ne 0) { throw "go generate failed (exit $LASTEXITCODE)" }
    & go build -ldflags="-H windowsgui -s -w" -o clip-clap.exe ./cmd/clip-clap
    if ($LASTEXITCODE -ne 0) { throw "go build failed (exit $LASTEXITCODE)" }
    Write-Host "Built: clip-clap.exe ($(Get-Item clip-clap.exe).Length bytes)"
}

function start {
    if (-not (Test-Path './clip-clap.exe')) {
        throw "clip-clap.exe not found — run 'agent-run.ps1 build' first"
    }
    $logsDir = 'logs'
    if (-not (Test-Path $logsDir)) {
        New-Item -ItemType Directory -Path $logsDir | Out-Null
    }
    $logFile = Join-Path $logsDir 'agent-latest.jsonl'

    $exeArgs = @()
    if ($RestArgs -and ($RestArgs -contains '--agent-mode')) {
        $exeArgs += '--agent-mode'
    }

    Write-Host "Starting clip-clap.exe $($exeArgs -join ' ') (logs → $logFile)"
    $proc = Start-Process -FilePath './clip-clap.exe' `
                          -ArgumentList $exeArgs `
                          -RedirectStandardOutput $logFile `
                          -RedirectStandardError "$logFile.err" `
                          -PassThru -NoNewWindow
    # PID file format: single decimal integer on one line, no trailing whitespace
    # (architecture §[Verification Harness Contract]).
    Set-Content -Path '.agent-running' -Value "$($proc.Id)" -NoNewline
    Write-Host "PID: $($proc.Id) (.agent-running written)"
}

function status {
    try {
        $resp = Invoke-RestMethod -Uri 'http://127.0.0.1:27773/status' `
                                  -Method Get -TimeoutSec 5
        $resp | ConvertTo-Json
    } catch {
        Write-Host "Status endpoint unreachable: $($_.Exception.Message)" -ForegroundColor Yellow
        exit 1
    }
}

function logs {
    if (Test-Path 'logs/agent-latest.jsonl') {
        Get-Content 'logs/agent-latest.jsonl' -Tail 50
    } else {
        Write-Host "No log file at logs/agent-latest.jsonl"
    }
}

function kill {
    if (-not (Test-Path '.agent-running')) {
        Write-Host "No .agent-running PID file"
        return
    }
    $pidStr = (Get-Content '.agent-running' -Raw).Trim()
    if ([string]::IsNullOrWhiteSpace($pidStr)) {
        Write-Host ".agent-running is empty"
        Remove-Item '.agent-running' -Force
        return
    }
    $procId = [int]$pidStr
    Write-Host "Killing PID $procId..."
    try {
        Stop-Process -Id $procId -Force -ErrorAction Stop
        Remove-Item '.agent-running' -Force
        Write-Host "Killed PID $procId"
    } catch {
        Write-Host "Process $procId not found or already gone: $($_.Exception.Message)"
        Remove-Item '.agent-running' -Force -ErrorAction SilentlyContinue
    }
}

# Dispatch
switch ($Cmd) {
    'build'  { build }
    'start'  { start }
    'status' { status }
    'logs'   { logs }
    'kill'   { kill }
}
