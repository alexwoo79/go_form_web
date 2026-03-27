@echo off
setlocal enabledelayedexpansion
set "DIR=%~dp0"
if not exist "%DIR%data" mkdir "%DIR%data"
"%DIR%go-web.exe" -config "%DIR%config.yaml"
