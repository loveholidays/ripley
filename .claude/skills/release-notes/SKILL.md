---
name: release-notes
description: Generate release notes from git commits since the last tag
disable-model-invocation: true
---

# Generate Release Notes

Generate release notes summarizing changes since the last git tag.

## Arguments

- `$ARGUMENTS`: Optional tag or commit range override (e.g., `v1.2.0`, `v1.1.0..v1.2.0`). If empty, uses the latest tag to HEAD.

## Workflow

1. Find the latest tag: `git describe --tags --abbrev=0`
2. Get commits since that tag: `git log <tag>..HEAD --oneline --no-merges`
3. Also check merge commits for PR context: `git log <tag>..HEAD --merges --oneline`
4. Categorize changes into:
   - **Features**: New functionality
   - **Fixes**: Bug fixes
   - **Performance**: Performance improvements
   - **Documentation**: Doc changes
   - **Maintenance**: Dependencies, CI, refactoring
5. Output in this format:

```markdown
## What's Changed

### Features
- Description of feature (#PR)

### Fixes
- Description of fix (#PR)

### Maintenance
- Description of maintenance item (#PR)

**Full Changelog**: `<tag>...HEAD`
```

## Notes

- Keep descriptions concise — one line per change
- Include PR numbers where available (extract from merge commit messages)
- Skip empty categories
- If there are no commits since the last tag, say so
