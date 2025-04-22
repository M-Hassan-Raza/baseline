@echo off
echo Building Baseline...
go build -o baseline.exe

if %ERRORLEVEL% NEQ 0 (
    echo Build failed!
    pause
    exit /b %ERRORLEVEL%
)

echo Build successful!
echo.
echo Running Baseline...
echo If the TUI doesn't appear properly, check baseline_debug.log for details.
echo.
echo Press CTRL+C to force quit if necessary.
echo.
baseline.exe

if %ERRORLEVEL% NEQ 0 (
    echo.
    echo Application exited with error code %ERRORLEVEL%
    echo Check baseline_debug.log for details.
)

pause
