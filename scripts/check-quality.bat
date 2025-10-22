@echo off
REM Quality Check Script for Resizr (Windows)
REM Checks: Tests, Coverage, Formatting, and Linting
REM
REM Usage:
REM   - From command line: scripts\check-quality.bat [--verbose]
REM   - Double-click this file to run (window will stay open)

setlocal enabledelayedexpansion

REM Change to project root directory (parent of scripts directory)
cd /d "%~dp0.."

set VERBOSE=0
if "%1"=="--verbose" set VERBOSE=1
if "%1"=="-v" set VERBOSE=1

set PASSED=0
set FAILED=0
set COVERAGE_THRESHOLD=45.0

echo.
echo ================================================================
echo            Resizr Quality Check Script
echo ================================================================
echo.

REM ============================================================================
REM 1. CHECK GO FMT
REM ============================================================================
echo [CHECK] Go Formatting (gofmt)
echo.

gofmt -s -l . > %TEMP%\gofmt-check.txt 2>&1
for /f %%A in (%TEMP%\gofmt-check.txt) do set UNFORMATTED=%%A

if not defined UNFORMATTED (
    echo [OK] All files are properly formatted
    set /a PASSED+=1
) else (
    echo [FAIL] Some files are not properly formatted:
    type %TEMP%\gofmt-check.txt
    echo.
    echo   Run 'gofmt -s -w .' to fix formatting
    set /a FAILED+=1
)
del %TEMP%\gofmt-check.txt 2>nul
echo.

REM ============================================================================
REM 2. CHECK LINTER
REM ============================================================================
echo [CHECK] Go Linter (golangci-lint)
echo.

where golangci-lint >nul 2>&1
if errorlevel 1 (
    echo [FAIL] golangci-lint is not installed
    echo   Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    set /a FAILED+=1
) else (
    golangci-lint run ./... > %TEMP%\lint-output.txt 2>&1
    if errorlevel 1 (
        echo [FAIL] Linter errors found:
        if %VERBOSE%==1 (
            type %TEMP%\lint-output.txt
        ) else (
            for /f "tokens=1-10 delims=:" %%a in (%TEMP%\lint-output.txt) do (
                echo   %%a:%%b:%%c:%%d:%%e:%%f:%%g:%%h:%%i:%%j
            )
            echo   ... use --verbose to see all errors
        )
        set /a FAILED+=1
    ) else (
        echo [OK] No linter errors found
        set /a PASSED+=1
    )
    del %TEMP%\lint-output.txt 2>nul
)
echo.

REM ============================================================================
REM 3. RUN TESTS
REM ============================================================================
echo [CHECK] Running Tests
echo.

go test ./... > %TEMP%\test-output.txt 2>&1
if errorlevel 1 (
    echo [FAIL] Some tests failed
    if %VERBOSE%==1 (
        type %TEMP%\test-output.txt
    ) else (
        findstr /I "FAIL" %TEMP%\test-output.txt
    )
    set /a FAILED+=1
) else (
    for /f %%i in ('findstr /R /C:"^ok" %TEMP%\test-output.txt ^| find /c /v ""') do set TOTAL_TESTS=%%i
    echo [OK] All tests passed (!TOTAL_TESTS! packages)
    set /a PASSED+=1

    if %VERBOSE%==1 (
        echo.
        echo   Test summary:
        findstr /R /C:"^ok" %TEMP%\test-output.txt
    )
)
del %TEMP%\test-output.txt 2>nul
echo.

REM ============================================================================
REM 4. CHECK COVERAGE
REM ============================================================================
echo [CHECK] Code Coverage
echo.
echo   Running tests with coverage analysis...

set COVERAGE_FILE=.coverage-check.out
go test -coverprofile=%COVERAGE_FILE% -covermode=atomic ./... >nul 2>&1

if not exist %COVERAGE_FILE% (
    echo [FAIL] Failed to generate coverage report
    set /a FAILED+=1
) else (
    REM Get overall coverage
    for /f "tokens=*" %%i in ('go tool cover -func^=%COVERAGE_FILE% ^| findstr "total:"') do (
        set COVERAGE_LINE=%%i
    )
    REM Extract the last token (percentage) from the line
    for %%a in (!COVERAGE_LINE!) do set COVERAGE_WITH_PERCENT=%%a
    REM Remove the % sign
    set COVERAGE=!COVERAGE_WITH_PERCENT:~0,-1!

    echo.
    echo   Overall Coverage: !COVERAGE!%%

    REM Simple threshold check using string comparison (works for integers)
    set COVERAGE_INT=!COVERAGE:.=!
    set THRESHOLD_INT=%COVERAGE_THRESHOLD:.=%

    if !COVERAGE_INT! GEQ !THRESHOLD_INT! (
        echo [OK] Coverage meets threshold ^(^>=%COVERAGE_THRESHOLD%%%^)
        set /a PASSED+=1
    ) else (
        echo [FAIL] Coverage below threshold: !COVERAGE!%% ^< %COVERAGE_THRESHOLD%%%
        set /a FAILED+=1
    )

    echo.
    echo   Coverage by Package:
    echo.
    echo   Package                          Coverage    Status
    echo   ----------------------------------------------------------------

    REM Run tests again to get per-package coverage
    go test ./... -cover > %TEMP%\package-coverage.txt 2>&1

    REM Only process lines starting with "ok" and containing "coverage:"
    for /f "tokens=2,5" %%a in ('findstr "^ok.*coverage:" %TEMP%\package-coverage.txt') do (
        set PACKAGE=%%a
        set PERCENT=%%b

        REM Remove % sign if present
        set PERCENT=!PERCENT:%%=!

        REM Extract just the package name (last part after /)
        for %%p in (!PACKAGE!) do set PKG_NAME=%%~nxp

        REM Determine status based on coverage
        set PERCENT_INT=!PERCENT:.=!
        if !PERCENT_INT! GEQ 900 (
            echo   !PKG_NAME!                          !PERCENT!%%    [OK] Excellent
        ) else if !PERCENT_INT! GEQ 700 (
            echo   !PKG_NAME!                          !PERCENT!%%    [OK] Good
        ) else if !PERCENT_INT! GEQ 500 (
            echo   !PKG_NAME!                          !PERCENT!%%    [!] Moderate
        ) else (
            echo   !PKG_NAME!                          !PERCENT!%%    [X] Low
        )
    )

    del %TEMP%\package-coverage.txt 2>nul
    del %COVERAGE_FILE% 2>nul
)

echo.

REM ============================================================================
REM SUMMARY
REM ============================================================================
echo.
echo ================================================================
echo                          SUMMARY
echo ================================================================
echo.

set /a TOTAL_CHECKS=PASSED+FAILED

if !FAILED!==0 (
    echo [OK] All checks passed! ^(!PASSED!/!TOTAL_CHECKS!^)
    echo.
    echo   Your code is ready to commit!
    set EXIT_CODE=0
) else (
    echo [FAIL] Some checks failed ^(!FAILED!/!TOTAL_CHECKS! failed, !PASSED!/!TOTAL_CHECKS! passed^)
    echo.
    echo   Please fix the issues above before committing.
    set EXIT_CODE=1
)

REM Pause if the script was double-clicked (not run from command line)
echo %CMDCMDLINE% | findstr /i /c:"/c" >nul
if %ERRORLEVEL% equ 0 (
    echo.
    echo Press any key to close...
    pause >nul
)

exit /b %EXIT_CODE%
