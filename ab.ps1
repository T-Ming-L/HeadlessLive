Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
$env:GOOS = "windows"; $env:GOARCH = "amd64"
& "C:\Go\bin\go.exe" test ./internal/scene/ ./internal/model/ *> a.txt
$t = $LASTEXITCODE
$env:CGO_ENABLED = "0"; $env:GOOS = "linux"
& "C:\Go\bin\go.exe" build -ldflags="-s -w" -o "$env:USERPROFILE\Desktop\HeadlessLive-linux-amd64" . *> b.txt
$b = $LASTEXITCODE
& $git add -A 2>&1 | Out-Null
& $git commit -m "fix: 文字源透明背景——color 画布显式 format=rgba，overlay 才能正确混合" *> c.txt
$c = $LASTEXITCODE
Write-Output "test=$t build=$b commit=$c"
