# Release Process

This document describes how to create a new release of Chatty.

## Prerequisites

- Ensure all tests pass: `make test`
- Ensure the build works: `make build`
- Update version in relevant files if needed
- Update CHANGELOG or release notes

## Creating a Release

### Option 1: Full Release Process (Recommended)

Use the interactive release command:

```bash
make release
```

This will:
- Run all tests to ensure code quality
- Show the current version and commit
- Prompt for documentation update confirmation
- Prompt you for a new version (e.g., `v0.3.0`)
- Validate version format (must be `vX.Y.Z`)
- Check if tag already exists
- Create an annotated git tag
- Show you the command to push the tag

### Option 2: Quick Tag (Skip Checks)

If you've already run tests and updated docs:

```bash
make tag
```

This creates a tag without running tests or prompts.

### Push the Tag

Push the tag to GitHub to trigger the release workflow:

```bash
git push origin v0.3.0
```

### 3. Automated Release Process

Once the tag is pushed, GitHub Actions will automatically:

1. **Build** stripped binaries for all platforms:
   - Linux (amd64, arm64)
   - macOS (arm64)
   - Windows (amd64, arm64)

2. **Create archives** with the binary, LICENSE, and README.md:
   - `.tar.gz` for Unix-like systems
   - `.zip` for Windows

3. **Generate checksums** (SHA256) for all archives

4. **Create a GitHub Release** with:
   - All binary archives
   - Checksums file
   - Auto-generated release notes

## Manual Release Build

To test the release build locally:

```bash
make build-release
```

This creates stripped binaries in the `dist/` directory using the same flags as the CI/CD pipeline.

## Binary Stripping

Release binaries are built with `-ldflags="-s -w"` which:
- `-s` - Omits the symbol table and debug information
- `-w` - Omits the DWARF symbol table

This reduces binary size by approximately 28% (from ~25MB to ~18MB).

## Archive Contents

Each release archive contains:
- The platform-specific binary (e.g., `chatty-linux-amd64`)
- `LICENSE` - MIT License
- `README.md` - Full documentation
- `config.example.yaml` - Example configuration file

## Verifying a Release

Users can verify their download using the checksums file:

```bash
# Download the archive and checksums.txt
sha256sum -c checksums.txt
```

## Rollback

If a release has issues:

1. Delete the GitHub release
2. Delete the tag locally and remotely:
   ```bash
   git tag -d v0.3.0
   git push origin :refs/tags/v0.3.0
   ```
3. Fix the issues
4. Create a new patch version (e.g., v0.3.1)

## Version Numbering

Follow [Semantic Versioning](https://semver.org/):

- **MAJOR** version (v1.0.0 → v2.0.0): Incompatible API changes
- **MINOR** version (v0.1.0 → v0.2.0): New functionality, backwards compatible
- **PATCH** version (v0.1.0 → v0.1.1): Bug fixes, backwards compatible

## GitHub Actions Workflows

### Release Workflow (`.github/workflows/release.yml`)
- Triggered by: Pushing a tag matching `v*`
- Builds all platforms
- Creates release archives
- Publishes GitHub release

### Test Workflow (`.github/workflows/test.yml`)
- Triggered by: Push to main/master, pull requests
- Runs tests and linting
- Verifies builds for all platforms
