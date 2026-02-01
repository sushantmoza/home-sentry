# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.4.0] - 2026-02-01

### Added
- **Security: AES-256 Encryption** - All sensitive settings (MAC, SSID, PIN, IP, ntfy topic) are now encrypted with AES-GCM
  - Encryption key stored securely in user's profile directory
  - Automatic migration from unencrypted legacy settings
  - Transparent encryption/decryption - no user action required
- **Security: PIN Confirmation** - Optional PIN requirement before shutdown
  - Configure 4-8 digit PIN in settings
  - Extra confirmation window when PIN is enabled
  - Prevents accidental or malicious shutdowns
- **Security: Input Validation** - Comprehensive validation for all user inputs
  - SSID sanitization (removes dangerous characters)
  - MAC address validation with multiple format support
  - IP address validation
  - PIN validation (digits only)
  - Topic name validation for ntfy
- **Multiple Shutdown Actions** - Choose what happens when protection triggers
  - Shutdown (default) - Full system shutdown
  - Hibernate - Save state and hibernate
  - Sleep - Sleep mode (maintains RAM state)
  - Lock - Lock the workstation (no data loss)
- **State Persistence** - phoneEverSeen state is now saved to disk
  - Prevents false triggers after app restart
  - Grace period only activates after phone was detected once
- **Retry Logic** - Network operations automatically retry with exponential backoff
  - 3 retry attempts with configurable delay
  - Improves reliability on unstable networks
- **Centralized Constants** - All magic numbers extracted to `pkg/config/constants.go`
  - Easier configuration management
  - Better maintainability

### Changed
- Standardized all logging to use custom logger package (replaced 22 log.Printf/log.Println calls)
- Improved error messages with user-friendly validation errors
- Settings validation enforces min/max values for all numeric fields

### Security
- Settings file now encrypted - no plaintext sensitive data
- Input sanitization prevents injection attacks
- Constant-time PIN comparison to prevent timing attacks

## [1.3.6] - 2026-01-09

### Fixed
- **Detection Logic**: Fixed stale ARP cache causing phone to appear online when disconnected
  - Now actively deletes cached ARP entry before each check
  - Pings last known IP to force fresh ARP lookup
  - Only trusts MAC presence after active verification

## [1.3.4] - 2026-01-09

### Fixed
- **Tray Icon**: Converted icons from PNG to ICO format (Windows systray requires ICO, not PNG)

## [1.3.3] - 2026-01-09

### Fixed
- **Tray Icon**: Resized all icons to standard 64x64 size to fix invisible tray icon issue (previous 2048x2048 icons were too large for Windows API)

## [1.3.2] - 2026-01-09

### Updated
- **Assets**: Updated application icons with new PNGs

## [1.3.1] - 2026-01-09

### Fixed
- **Tray Icon**: Fixed missing system tray icon (icons were incorrectly formatted as JPEGs, converted to valid PNGs)

## [1.3.0] - 2026-01-09

### Added
- **Auto-start on boot**: Toggle from tray menu to start Home Sentry with Windows
- **Sound alerts**: Warning beeps play every 2 seconds during shutdown countdown
- **Location status**: Tray shows "🏠 At Home" or "📍 Roaming" based on WiFi
- **Improved scan UI**: Better device listing with emoji icons, device count header, and clearer tooltips

### Changed
- All menu items now have emoji icons for better visual clarity
- Scan results show "📱 Hostname (IP)" format with MAC in tooltip
- Status messages use emoji indicators (⏳, ✅, ❌)

## [1.2.0] - 2026-01-09

### Added
- **MAC-based detection**: Phone detection now uses MAC address lookup in ARP table instead of ICMP ping
- Works even when phone IP changes (DHCP)
- Works even if phone blocks ping
- No need for static IP or DHCP reservation
- New `detection_type` setting in config (defaults to "mac")

### Changed
- `set-device` CLI command now accepts MAC address instead of IP
- Device selection in tray saves MAC address instead of IP
- Status display shows Phone MAC instead of Phone IP
- Default detection type changed from "ip" to "mac"

### Migration
- Existing users with IP-only configuration will need to re-select their device
- Run `home-sentry scan` and select your phone to save its MAC address

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
