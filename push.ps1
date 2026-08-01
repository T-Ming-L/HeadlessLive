Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
# 清理临时文件
Remove-Item dlffmpeg.ps1, clean2.ps1, g1.txt, g2.txt, g3.txt, g4.txt, gits.ps1, x1.txt, x2.txt, x3.txt -ErrorAction SilentlyContinue
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 清理临时文件" 2>&1 | Out-Null
Write-Output "==== 开始推送 22+ 个提交到 GitHub ===="
& $git push origin main *> push.txt
$code = $LASTEXITCODE
Write-Output "push exit=$code"
Get-Content push.txt -Encoding utf8 | Select-Object -Last 15
