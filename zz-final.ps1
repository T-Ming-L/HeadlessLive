Set-Location e:\WORK\Web-RTMP
Remove-Item z-final.ps1, s.txt, clean.ps1 -ErrorAction SilentlyContinue
$git = "C:\Program Files\Git\cmd\git.exe"
& $git add -A 2>&1 | Out-Null
& $git commit -m "chore: 清理临时脚本" *> z.txt
& $git status --short *> zs.txt
Get-Content z.txt -Encoding Unicode
Write-Output "==== status ===="
Get-Content zs.txt -Encoding Unicode
Write-Output "==== END ===="
