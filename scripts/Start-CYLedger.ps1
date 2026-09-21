[CmdletBinding()]
param(
    [int]$Port = 8080,
    [string]$BindAddress = '127.0.0.1',
    [string]$RuntimeDirectory = '',
    [switch]$Build,
    [switch]$DirectNpm,
    [switch]$ConfigureOnly
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
if (-not $RuntimeDirectory) { $RuntimeDirectory = Join-Path $projectRoot '.runtime' }
$runtimePath = [IO.Path]::GetFullPath($RuntimeDirectory)
New-Item -ItemType Directory -Path $runtimePath -Force | Out-Null

$goBin = 'D:\tools\cyledger\go\bin'
$gccBin = 'D:\tools\cyledger\mingw64\bin'
if (Test-Path -LiteralPath $goBin) { $env:Path = "$goBin;$env:Path" }
if (Test-Path -LiteralPath $gccBin) { $env:Path = "$gccBin;$env:Path" }
$pythonExe = (Get-Command python -ErrorAction Stop).Source
$binaryPath = Join-Path $runtimePath 'cyledger.exe'
$configPath = Join-Path $runtimePath 'cyledger.ini'

Push-Location $projectRoot
try {
    & $pythonExe (Join-Path $PSScriptRoot 'runtime_config.py') --root $projectRoot --runtime $runtimePath --port $Port --bind $BindAddress
    if ($LASTEXITCODE -ne 0) { throw 'Runtime configuration failed.' }
    if ($ConfigureOnly) { return }
    if ($Build) {
        $env:CGO_ENABLED = '1'
        if (Test-Path -LiteralPath (Join-Path $gccBin 'gcc.exe')) { $env:CC = Join-Path $gccBin 'gcc.exe' }
        $goExe = (Get-Command go -ErrorAction Stop).Source
        & $goExe build -trimpath -ldflags '-s -w -X main.Version=CYLedger-0.1.0' -o $binaryPath ezbookkeeping.go
        if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
        $npmExe = (Get-Command npm.cmd -ErrorAction Stop).Source
        $npmArguments = @()
        if ($DirectNpm) {
            $npmConfigPath = Join-Path $runtimePath 'npm-direct.npmrc'
            [IO.File]::WriteAllText($npmConfigPath, "registry=https://registry.npmjs.org/`nproxy=null`nhttps-proxy=null`n", [Text.UTF8Encoding]::new($false))
            $npmArguments = @('--userconfig', $npmConfigPath)
        }
        & $npmExe @npmArguments ci --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) { throw 'npm dependency installation failed.' }
        & $npmExe @npmArguments run build
        if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed.' }
    }
    if (-not (Test-Path -LiteralPath $binaryPath)) { throw 'Binary missing. Run this script with -Build first.' }
    if (-not (Test-Path -LiteralPath (Join-Path $projectRoot 'dist/index.html'))) { throw 'Frontend missing. Run this script with -Build first.' }
    $env:EBK_WORK_DIR = $projectRoot
    Write-Host "CYLedger: http://localhost:$Port/ (press Ctrl+C to stop)"
    Write-Host "Private runtime: $runtimePath"
    & $binaryPath --conf-path $configPath server run
    if ($LASTEXITCODE -ne 0) { throw "CYLedger exited with code $LASTEXITCODE." }
} finally {
    Pop-Location
}
