# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https.md://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https.md://semver.org/spec/v2.0.0.html).

## [0.4.0] - 2025-11-21

### Added

-   Enhanced CLI mode with direct command support (`/list`, `/load`, `/help`)
-   Improved session management with better error handling
-   Enhanced security validation for API keys and user inputs

### Changed

-   Refactored storage initialization to use default paths when configuration is empty
-   Improved TUI with better command handling and user experience
-   Enhanced error handling and validation throughout the application
-   Updated dependencies and build process optimizations

### Fixed

-   Fixed "/list" command not working when storage path was empty in configuration
-   Resolved issues with session persistence and loading
-   Improved markdown rendering and code block formatting
-   Fixed various edge cases in command processing

## [0.3.1] - 2025-11-16

### Added

-   API response caching to speed up repeated prompts.
-   Comprehensive configuration validation on startup.
-   `CHANGELOG.md` to track changes between versions.

### Changed

-   Optimized database queries with prepared statements for faster performance.
-   Improved streaming output buffer for lower latency.
-   Enhanced `Makefile` with more granular build targets and optimizations.
-   Updated all Go modules to their latest versions.

### Fixed

-   Corrected minor formatting issues in markdown rendering.
-   Improved error handling for API and database operations.

## [0.3.0] - 2025-11-15

-   Initial public release.