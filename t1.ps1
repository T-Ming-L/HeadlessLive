# 测试文字源修复
Set-Location e:\WORK\Web-RTMP
$env:GOOS = "windows"
$env:GOARCH = "amd64"
& "C:\Go\bin\go.exe" test ./internal/scene/ *> t1.txt
$code = $LASTEXITCODE
Set-Content -Path t1-exit.txt -Value $code
Write-Output "DONE code=$code"
