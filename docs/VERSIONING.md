# Automatic Semantic Versioning

This project uses an automatic semantic versioning system that creates and pushes version tags whenever changes are merged to the main branch.

## How It Works

### GitHub Actions Workflow

The automatic versioning is handled by a GitHub Actions workflow (`.github/workflows/auto-version.yml`) that:

1. **Triggers on**:
   - Direct pushes to the `main` branch
   - Merged pull requests to the `main` branch

2. **Analyzes commits** since the last tag to determine the version bump type:
   - **Major** (X.0.0): Breaking changes, marked with `!` or `BREAKING`
   - **Minor** (0.X.0): New features, commits starting with `feat:`, `feature:`, or `add:`
   - **Patch** (0.0.X): Bug fixes, documentation, refactoring, etc.

3. **Creates and pushes** a new semantic version tag
4. **Creates a GitHub release** with changelog
5. **Updates** the version in `go.mod` if present

### Conventional Commits

The system follows [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Supported types**:
- `feat:` - New features (minor bump)
- `fix:` - Bug fixes (patch bump)
- `docs:` - Documentation changes (patch bump)
- `style:` - Code style changes (patch bump)
- `refactor:` - Code refactoring (patch bump)
- `perf:` - Performance improvements (patch bump)
- `test:` - Adding or updating tests (patch bump)
- `chore:` - Maintenance tasks (patch bump)
- `build:` - Build system changes (patch bump)
- `ci:` - CI/CD changes (patch bump)

**Breaking changes**:
- Add `!` after the type: `feat!: breaking change`
- Add `BREAKING CHANGE:` in the footer
- Use `BREAKING` in the commit message

## Local Development

### Using the Version Script

The `scripts/version.sh` script provides local version management:

```bash
# Show current version
./scripts/version.sh current

# Show next version
./scripts/version.sh next minor

# Bump version and create tag
./scripts/version.sh bump patch

# Create release (requires GitHub CLI)
./scripts/version.sh release major

# Check what version bump is needed
./scripts/version.sh check

# Validate commit messages
./scripts/version.sh validate
```

### Using Make Commands

```bash
# Show current version
make version-current

# Show next version
make version-next TYPE=minor

# Bump version
make version-bump TYPE=patch

# Create release
make version-release TYPE=major

# Check version bump
make version-check

# Validate commits
make version-validate
```

## Configuration

### Version Configuration (`.versionrc`)

```json
{
  "version": "0.16.6",
  "conventionalCommits": true,
  "commitTypes": {
    "major": ["breaking", "!", "BREAKING"],
    "minor": ["feat", "feature", "add"],
    "patch": ["fix", "bug", "patch", "docs", "style", "refactor", "perf", "test", "chore", "build", "ci"]
  },
  "releaseNotes": {
    "enabled": true,
    "template": "## Changes in this release\n\n### Commits since {previousVersion}:\n{commits}\n\n### Version Bump Type\n- **{bumpType}** version increment"
  },
  "git": {
    "tagPrefix": "v",
    "commitMessageTemplate": "chore: release {version}"
  },
  "github": {
    "release": true,
    "draft": false,
    "prerelease": false
  }
}
```

## Examples

### Commit Messages

```bash
# Minor version bump (new feature)
git commit -m "feat: add user authentication system"

# Patch version bump (bug fix)
git commit -m "fix: resolve memory leak in database connection"

# Major version bump (breaking change)
git commit -m "feat!: change API response format"

# Or with BREAKING CHANGE footer
git commit -m "feat: add new API endpoint

BREAKING CHANGE: The /api/v1/users endpoint now requires authentication"
```

### Version Bumps

```bash
# Current version: v1.2.3

# After feat: commit -> v1.3.0 (minor bump)
# After fix: commit -> v1.2.4 (patch bump)
# After feat!: commit -> v2.0.0 (major bump)
```

## Manual Version Management

If you need to manually manage versions:

```bash
# Create a tag manually
git tag -a v1.2.3 -m "Release v1.2.3"
git push origin v1.2.3

# Create a release manually
gh release create v1.2.3 --title "Release v1.2.3" --notes "Release notes here"
```

## Troubleshooting

### Common Issues

1. **No version bump detected**: Ensure commit messages follow conventional commit format
2. **Wrong bump type**: Check commit message prefixes and breaking change indicators
3. **GitHub Actions not running**: Verify workflow file is in `.github/workflows/` and has correct permissions
4. **Tag not created**: Check if there are actually new commits since the last tag

### Debugging

```bash
# Check commits since last tag
git log --oneline $(git describe --tags --abbrev=0)..HEAD

# Validate commit messages
./scripts/version.sh validate

# Check what version bump would be applied
./scripts/version.sh check
```

## Integration with CI/CD

The versioning system integrates with:

- **GitHub Actions**: Automatic versioning on merge
- **GitHub Releases**: Automatic release creation
- **Go Modules**: Version tracking in `go.mod`
- **Git Tags**: Semantic version tags for releases

## Best Practices

1. **Use conventional commits** for consistent version bumping
2. **Test locally** before pushing to main branch
3. **Review version bumps** in pull requests
4. **Document breaking changes** clearly
5. **Keep commit messages descriptive** and concise
6. **Use semantic versioning** consistently across the project

## Dependencies

- **Git**: For version control and tagging
- **GitHub CLI** (optional): For automatic release creation
- **GitHub Actions**: For automated versioning workflow

## Support

For issues with the versioning system:

1. Check the GitHub Actions workflow logs
2. Validate commit messages with `./scripts/version.sh validate`
3. Review the configuration in `.versionrc`
4. Ensure proper Git permissions for tag creation
