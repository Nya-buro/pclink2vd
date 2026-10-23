# rebuild pclink2vd.exe (single binary, GUI subsystem, embedded manifest)
$ErrorActionPreference = 'Stop'

# generate resource .syso next to the main package (required for embedding)
go run github.com/tc-hib/go-winres@latest make --in=winres/winres.json --out=cmd/pclink2vd/rsrc

New-Item -ItemType Directory -Force dist | Out-Null
go build -ldflags "-H windowsgui -s -w" -o dist/pclink2vd.exe ./cmd/pclink2vd
if (Test-Path dist/pclink2vd.exe) {
    "OK: dist/pclink2vd.exe ({0:N1} MB)" -f ((Get-Item dist/pclink2vd.exe).Length / 1MB)
}
