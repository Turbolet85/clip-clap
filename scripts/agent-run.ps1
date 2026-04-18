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

function startAgent {
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

    # v1.0.8: clip-clap's default log path moved to
    # %USERPROFILE%\Pictures\clip-clap\logs\agent-latest.jsonl. This harness
    # expects logs in the CWD-relative ./logs/ for deterministic testing, so
    # force the app to use our path via the CLIP_CLAP_LOG_PATH env var
    # (inherited by Start-Process child). Without this, agent-run.ps1 logs
    # command reads an empty CWD path while the real app log lives under
    # USERPROFILE.
    $absLogFile = Join-Path (Get-Location) $logFile
    $env:CLIP_CLAP_LOG_PATH = $absLogFile

    Write-Host "Starting clip-clap.exe $($exeArgs -join ' ') (logs → $logFile)"
    $proc = Start-Process -FilePath './clip-clap.exe' `
                          -ArgumentList $exeArgs `
                          -RedirectStandardOutput "$logFile.out" `
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

function killAgent {
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

    # Look up the process. If already gone, clean up the PID file and return.
    $proc = Get-Process -Id $procId -ErrorAction SilentlyContinue
    if ($null -eq $proc) {
        Write-Host "Process $procId not found (already exited); removing stale PID file"
        Remove-Item '.agent-running' -Force -ErrorAction SilentlyContinue
        return
    }

    # Stage 1: graceful close via `taskkill /PID` (no /F). Docs:
    # taskkill without /F sends a WM_CLOSE message to the process's
    # top-level windows. Clip-clap's wndProc on the HWND_MESSAGE window
    # is registered at that level, so the WM_CLOSE reaches it and
    # triggers status.Shutdown → PostQuitMessage → clean exit.
    #
    # We use taskkill instead of CloseMainWindow because the latter
    # only targets windows with WS_VISIBLE style in the current desktop
    # — tray/message-only apps return false from CloseMainWindow even
    # though they CAN receive WM_CLOSE.
    Write-Host "Sending WM_CLOSE to PID $procId..."
    & taskkill /PID $procId | Out-Null

    # Stage 2: wait up to 3 seconds for the process to exit gracefully.
    # Shorter than the 5s AC budget so the fallback path still fits
    # within 5s total (3s wait + up to 2s for taskkill /F + reap).
    if ($proc.WaitForExit(3000)) {
        Remove-Item '.agent-running' -Force -ErrorAction SilentlyContinue
        Write-Host "PID $procId exited gracefully"
        return
    }

    # Stage 3: graceful path timed out — emit the fallback message to
    # stderr (per plan §Step 9 Security note; test_agent_run_ps1_kill_falls_back_to_taskkill
    # asserts on this literal) and force-kill via taskkill /F.
    [Console]::Error.WriteLine("falling back to taskkill /PID $procId /F")
    & taskkill /PID $procId /F | Out-Null
    # Give the OS a moment to reap the process before cleanup.
    Start-Sleep -Milliseconds 500

    # Cleanup: PID file removal is idempotent (tolerates file already gone —
    # e.g., if the Go process itself removed it via status.Shutdown's
    # pidfile.DeletePIDFile call before we got here).
    Remove-Item '.agent-running' -Force -ErrorAction SilentlyContinue
    Write-Host "PID $procId is gone"
}

# Dispatch
switch ($Cmd) {
    'build'  { build }
    'start'  { startAgent }
    'status' { status }
    'logs'   { logs }
    'kill'   { killAgent }
}
