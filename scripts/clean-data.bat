@echo off
REM Clean all data directories for Resizr
echo Cleaning Resizr data directories...

if exist "..\data\cache" (
    echo Removing cache directory...
    rmdir /s /q "..\data\cache"
)

if exist "..\data\cache_queue" (
    echo Removing cache_queue directory...
    rmdir /s /q "..\data\cache_queue"
)

if exist "..\data\storage" (
    echo Removing storage directory...
    rmdir /s /q "..\data\storage"
)

echo.
echo Data directories cleaned successfully!
echo You can now start the server with fresh data.
echo.
pause
