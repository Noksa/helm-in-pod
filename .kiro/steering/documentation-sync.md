---
inclusion: always
---

# Documentation Synchronization

## CRITICAL: Always Update Documentation After Code Changes

After completing ANY code changes, you MUST:

1. **List all markdown files in project root**:
   ```bash
   ls *.md
   ```

2. **Review each markdown file** to determine if changes are needed based on:
   - New features added
   - Changed functionality
   - New flags or options
   - Modified behavior
   - Updated examples

3. **Update relevant documentation** to reflect the changes

4. **Ask user about version bump**: If core functionality was changed (new features, bug fixes, breaking changes), ask the user if the version in `plugin.yaml` should be bumped

## Common Documentation Files to Check

- `README.md` - Main project documentation, usage examples
- `DAEMON.md` - Daemon mode specific documentation
- `CONTRIBUTING.md` - Contribution guidelines
- `CHANGELOG.md` - Version history and changes
- Any other `.md` files in the root directory

## When to Update

- ✅ New command flags added
- ✅ Command behavior changed
- ✅ New features implemented
- ✅ Examples need updating
- ✅ Configuration options modified

## Version Bumping

Ask user if version should be bumped in `plugin.yaml` when:
- ✅ New features added
- ✅ Bug fixes implemented
- ✅ Breaking changes made
- ✅ New commands or flags added
- ✅ Behavior modifications

## Documentation Standards

- Keep examples accurate and tested
- Update all affected sections
- Maintain consistent formatting
- Include practical use cases
- Keep language clear and concise
