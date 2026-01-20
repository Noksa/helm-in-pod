---
inclusion: fileMatch
fileMatchPattern: "**/*.go"
---

# Go Development Standards

## REQUIRED: Code Modernization

After completing Go code changes, you MUST:

2. Run the modernize tool to get diff if there would be changed:
   ```bash
   go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -diff -fix ./...
   ```
   
3. Check the changes and highlight them if they are important, then run again to apply the fixes
   ```bash
   go run golang.org/x/tools/go/analysis/passes/modernize/cmd/modernize@latest -fix ./...
   ```

4. If a Makefile exists with `lint` or `test` targets, run them to check if something is broken

## Timing Rule

For multi-step implementations: Run modernize/lint/test ONLY after ALL changes are complete, not after each incremental change.

## Environment

- Current Go version: 1.25 (latest stable) - check go.mod in specific projects which version is actually is used
- Use Go 1.25 features and idioms in all code when go.mod contains 1.25+ version
