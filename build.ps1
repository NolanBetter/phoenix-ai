# Builds build\PhoenixAI.exe, and the installer too if NSIS is installed
# (https://nsis.sourceforge.io). Run from the project folder: .\build.ps1
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

$version = (Get-Content VERSION -Raw).Trim()
Write-Host "Building Phoenix AI $version"

go run ./tools/mkres
if ($LASTEXITCODE) { exit $LASTEXITCODE }

$env:CGO_ENABLED = "0"
go build -trimpath -tags desktop,production -ldflags "-w -s -H windowsgui -X phoenixai/backend.Version=$version" -o build\PhoenixAI.exe .
if ($LASTEXITCODE) { exit $LASTEXITCODE }
Write-Host "Built build\PhoenixAI.exe"

$makensis = (Get-Command makensis -ErrorAction SilentlyContinue).Source
if (-not $makensis) {
  foreach ($p in @("${env:ProgramFiles(x86)}\NSIS\makensis.exe", "$env:ProgramFiles\NSIS\makensis.exe")) {
    if (Test-Path $p) { $makensis = $p; break }
  }
}
if (-not $makensis) {
  Write-Host "NSIS isn't installed, so no installer was built. Get it from https://nsis.sourceforge.io"
  exit 0
}

& $makensis -V2 "-DVERSION=$version" build\installer\phoenix.nsi
if ($LASTEXITCODE) { exit $LASTEXITCODE }
$setup = "PhoenixAI-Setup-$version.exe"
$hash = (Get-FileHash "build\$setup" -Algorithm SHA256).Hash.ToLower()
Set-Content -NoNewline -Path "build\$setup.sha256" -Value "$hash  $setup`n"
Write-Host "Built build\$setup"
