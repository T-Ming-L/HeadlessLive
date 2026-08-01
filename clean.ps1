Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"

# 删除磁盘残留
Remove-Item tools\cef-osr-test\build-linux-cef.sh -Force -ErrorAction SilentlyContinue
Remove-Item tools\cef-osr-test -Force -ErrorAction SilentlyContinue
Remove-Item tools -Force -ErrorAction SilentlyContinue
Remove-Item final-status.txt, last-clean.ps1, diag.ps1 -ErrorAction SilentlyContinue

# 提交 README 格式 + 记录临时文件删除
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: README 表格格式整理；清理临时文件" *> clean.txt
$code = $LASTEXITCODE
& $git status --short *> clean-status.txt
Set-Content -Path clean-exit.txt -Value $code
Write-Output "DONE code=$code"
