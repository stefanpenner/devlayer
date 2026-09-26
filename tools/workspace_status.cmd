@echo off
setlocal EnableExtensions
REM Native cmd. Bazel runs workspace_status_command via cmd.exe /S /C.
REM git describe --dirty waits on the index (run 35561178234, 205/206).
set GIT_TERMINAL_PROMPT=0
set GCM_INTERACTIVE=Never
set "VER=dev"
git --no-optional-locks -c core.fsmonitor=false describe --tags --always <NUL >"%TEMP%\devlayer-ws.txt" 2>NUL
if not errorlevel 1 set /p VER=<"%TEMP%\devlayer-ws.txt"
if "%VER%"=="" set "VER=dev"
echo STABLE_VERSION %VER%
exit /b 0
