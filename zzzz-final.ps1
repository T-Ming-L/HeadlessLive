Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
& $git clean -fd -e zzzz-final.ps1 *> c.txt
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 清理临时脚本（记录删除）" *> c2.txt
& $git status --short *> c3.txt
Get-Content c.txt -Encoding Unicode
Get-Content c2.txt -Encoding Unicode
Write-Output "==== status ===="
Get-Content c3.txt -Encoding Unicode
Write-Output "==== END ===="
