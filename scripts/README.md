# Quality Check Scripts

Automated code quality verification scripts for the Resizr project.

## What Gets Checked

1. **Go Formatting** - All files properly formatted (gofmt)
2. **Linting** - No linter errors (golangci-lint)
3. **Tests** - All unit tests pass
4. **Coverage** - Meets 45% threshold

## Usage

### Linux/Unix/macOS
```bash
./scripts/check-quality.sh          # Run checks
./scripts/check-quality.sh -v       # Verbose output
```

### Windows
```cmd
scripts\check-quality.bat           # Run checks
scripts\check-quality.bat -v        # Verbose output
```

**Note:** Scripts auto-detect project root - run from anywhere in the project.

## Prerequisites

- **Go 1.25+**
- **golangci-lint** (recommended)
  ```bash
  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
  ```

## Example Output

```
================================================================
            Resizr Quality Check Script
================================================================

[CHECK] Go Formatting (gofmt)
[OK] All files are properly formatted

[CHECK] Go Linter (golangci-lint)
[OK] No linter errors found

[CHECK] Running Tests
[OK] All tests passed (8 packages)

[CHECK] Code Coverage
  Overall Coverage: 46.6%
[OK] Coverage meets threshold (>=45.0%)

  Package                          Coverage    Status
  ----------------------------------------------------------------
  config                          100.0%    [OK] Excellent
  middleware                       98.5%    [OK] Excellent
  models                           95.5%    [OK] Excellent
  handlers                         91.3%    [OK] Excellent
  service                          70.3%    [OK] Good
  storage                          36.1%    [X] Low

================================================================
                          SUMMARY
================================================================

[OK] All checks passed! (4/4)
  Your code is ready to commit!
```

## Configuration

### Coverage Threshold
Default: **45%** (realistic for infrastructure-dependent packages)

To change:
- `check-quality.sh`: Edit line 40
- `check-quality.bat`: Edit line 20

### Exit Codes
- **0** - All checks passed
- **1** - One or more checks failed

## CI/CD Integration

```bash
# Run before push
./scripts/check-quality.sh && git push
```

These scripts mirror `.github/workflows/test-and-build.yml` checks.

## Pre-commit Hook (Optional)

### Linux/macOS
```bash
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/bash
./scripts/check-quality.sh
EOF
chmod +x .git/hooks/pre-commit
```

### Windows (Git Bash)
```bash
cat > .git/hooks/pre-commit << 'EOF'
#!/bin/sh
cmd //c scripts\\check-quality.bat
EOF
chmod +x .git/hooks/pre-commit
```

## Troubleshooting

**golangci-lint not found:**
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
export PATH=$PATH:$(go env GOPATH)/bin  # Linux/Mac
```

**Permission denied (Linux/Mac):**
```bash
chmod +x scripts/check-quality.sh
```

**Windows batch closes immediately:**
- Double-click the `.bat` file - it will pause automatically
- Or run from command prompt for immediate exit

## Features

| Feature | Bash (.sh) | Batch (.bat) |
|---------|------------|--------------|
| Auto-detect project root | ✅ | ✅ |
| Verbose mode | ✅ | ✅ |
| Color output | ✅ | ❌ |
| Works in Git Bash | ✅ | ✅ |
| Double-click support | ✅ | ✅ |
