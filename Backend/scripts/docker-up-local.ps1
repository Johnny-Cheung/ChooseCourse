$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

Write-Host "Building Linux binaries with local Go toolchain..."
$env:CGO_ENABLED = "0"
$env:GOOS = "linux"
$env:GOARCH = "amd64"

New-Item -ItemType Directory -Force .\bin | Out-Null

go build -o .\bin\server .\cmd\server
go build -o .\bin\worker .\cmd\worker
go build -o .\bin\migrate .\cmd\migrate

Write-Host "Starting Docker Compose with local-binary override..."
docker compose -f docker-compose.yml -f docker-compose.local.yml up --build -d
