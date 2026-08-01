Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
# 删除误提交的临时文件（保留历史，删除文件）
& $git rm -f a.txt ab.ps1 b.txt buildlog.txt c.txt clean3.ps1 fx.ps1 lg.ps1 push.ps1 rbuild-check.txt rclean.txt rlog2.txt t1-exit.txt t1.ps1 t1.txt 2>&1 | Out-Null
# 删除工作区未跟踪残留
Remove-Item checktmp.ps1, f1.txt, f2.txt, v.ps1 -ErrorAction SilentlyContinue
# 提交 + 推送
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 删除误提交的调试临时文件（ps1/txt）" *> c1.txt
& $git push origin main *> c2.txt
$code = $LASTEXITCODE
Write-Output "push exit=$code"
Get-Content c2.txt -Encoding utf8 | Select-Object -Last 3
Write-Output "==== 剩余跟踪的 ps1/txt ===="
& $git ls-files "*.ps1" "*.txt" *> c3.txt
Get-Content c3.txt -Encoding utf8
Write-Output "==== 状态 ===="
& $git status -sb *> c4.txt
Get-Content c4.txt -Encoding utf8
