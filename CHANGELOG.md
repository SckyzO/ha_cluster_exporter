# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- Added `--collector.<name>` flags to enable/disable specific collectors.
- Added `--collector.timeout` flag to configure the execution timeout for external commands (default: 10s).
- Implemented global timeout context for all external command executions (`crm_mon`, `cibadmin`, `corosync-*`, `sbd`, `drbdsetup`).
- Added graceful failure: exporter now starts even if collector binaries are missing (logs a warning).

### Changed
- Refactored `main.go` to remove deprecated code.
- Removed deprecated flags: `--address`, `--port`, `--log-level`, `--enable-timestamps`.
- Improved help output by removing clutter.
