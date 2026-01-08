# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2026-01-08

### Added
- **Configurable settings**: Grace checks, poll interval, and ping timeout can now be customized in settings.json
- **APPDATA storage**: Settings are now stored in `%APPDATA%\HomeSentry\` for better reliability
- **Shutdown countdown**: 10-second countdown before shutdown with cancel option
- **Cancel shutdown**: New menu item to cancel pending shutdown
- **Windows notifications**: Toast notification before shutdown
- **Waiting for phone status**: New status when phone hasn't been detected yet
- **Version display**: Version shown in CLI and tray menu (`version` command)
- **Logs command**: New `logs` CLI command to view recent logs
- **File logging**: Logs written to `%APPDATA%\HomeSentry\logs\` with daily rotation
- **Graceful shutdown**: Proper handling of SIGINT/SIGTERM signals
- **IP validation**: Invalid IP addresses are rejected with clear error messages

### Changed
- Improved grace period logic: Only triggers after phone was initially detected
- Settings file location moved from app directory to APPDATA
- Default ping timeout increased from 200ms to 500ms
- Status command now shows all configuration values

### Fixed
- Fixed broken update loop that stopped after 100 iterations
- Removed dead `showDeviceSelector()` function
- Fixed Go version in go.mod (was incorrectly set to future version)

## [1.0.0] - 2026-01-07

### Added
- Initial release
- System tray application with status icons
- Network device scanning
- WiFi network detection
- Phone presence monitoring
- Auto-shutdown when phone leaves home network
- Pause/resume protection
- CLI commands: scan, wifi, status, set-home, set-device, pause, resume
