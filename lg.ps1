Set-Location e:\WORK\Web-RTMP
$git = "C:\Program Files\Git\cmd\git.exe"
& $git log --oneline -3 *> lg.txt
Get-Content lg.txt -Encoding Unicode
Remove-Item cl.txt, st.txt, build-final.ps1, lg.txt -ErrorAction SilentlyContinue
Write-Output "==== END ===="
