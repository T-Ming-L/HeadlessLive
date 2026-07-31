@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"

echo =============================================
echo   HeadlessLive - Build for x86_64 Linux
echo =============================================
echo.

:: ====== Set PATH ======
set "PATH=C:\Go\bin;C:\Git\mingw64\bin;C:\Git\cmd;C:\Windows\System32;%PATH%"

go version >nul 2>&1 || (echo [ERROR] Go not found! & pause & exit /b 1)
echo [OK] Go:
go version

:: ====== Go module proxy ======
set GOPROXY=https://goproxy.cn,direct
set GOFLAGS=-mod=mod

echo.
echo [1/3] Building frontend (Vue3 + Vite)...
if exist "web\package.json" (
    cd web
    call npm install
    if %ERRORLEVEL% NEQ 0 (
        echo [ERROR] npm install failed!
        pause & exit /b 1
    )
    call npm run build
    if %ERRORLEVEL% NEQ 0 (
        echo [ERROR] npm run build failed!
        pause & exit /b 1
    )
    cd ..
)
echo [OK] Frontend done.

echo.
echo [2/3] go mod tidy...
go mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] go mod tidy failed!
    pause & exit /b 1
)
echo [OK] Done.

echo.
echo [3/3] Cross-compiling linux/amd64...
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
go build -ldflags="-s -w" -o HeadlessLive .
if %ERRORLEVEL% NEQ 0 (
    echo [ERROR] Build failed!
    pause & exit /b 1
)

echo.
echo =============================================
echo   BUILD SUCCESS! (x86_64 Linux, static)
echo =============================================
for %%f in (HeadlessLive) do echo   %%f  -  %%~zf bytes
echo.
echo Deploy: scp HeadlessLive user@^<server^>:~/
echo Run:    ssh user@^<server^> "chmod +x ~/HeadlessLive ^&^& ./HeadlessLive"
echo Open:   http://^<server^>:8080
pause
