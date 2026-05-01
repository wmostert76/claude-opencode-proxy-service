$ErrorActionPreference = "Stop"

$Repo = "wmostert76/claude-go"
$OS = "windows"
$ARCH = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "amd64" }

$BinDir = "$env:LOCALAPPDATA\bin"
New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

$Url = "https://github.com/${Repo}/releases/latest/download/claude-go-${OS}-${ARCH}.exe"
Write-Host "Downloading Claude Go for ${OS}/${ARCH}..."
Invoke-WebRequest -Uri $Url -OutFile "$BinDir\claude-go.exe"

Write-Host ""
Write-Host "Claude Go installed to $BinDir\claude-go.exe"
Write-Host ""
Write-Host "Next steps:"
Write-Host "  claude-go install               # Install Claude Code locally"
Write-Host "  claude-go --api <key>           # Store your OpenCode Go API key"
Write-Host "  claude-go                       # Start!"
