@echo off
cd /d "%~dp0"
"%~dp0IA-Locale-Studio.exe"
if errorlevel 1 pause
