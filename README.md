# Home Sentry 🏠🛡️

**Protect your laptop when you leave home.** Home Sentry monitors your home WiFi and phone presence - if your phone leaves but your laptop stays, it can trigger a shutdown to protect your data.

![Status](https://img.shields.io/badge/status-active-brightgreen)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- 🔐 **Encrypted Storage** - All sensitive data encrypted with AES-256-GCM
- 🔢 **PIN Protection** - Optional PIN confirmation before shutdown
- 🟢 **Safe** - Phone detected on home WiFi
- 🟡 **Warning** - Phone missing, grace period active
- 🔴 **Shutdown** - Grace period expired, protect your data
- ⏸️ **Pause** - Temporarily disable protection
- 📱 **Device Selection** - Scan and select your phone from network
- 🌐 **WiFi Detection** - Auto-detect home network
- 🛑 **Cancel Shutdown** - Abort pending shutdown with sound alert
- 🔊 **Sound Alerts** - Warning beeps during shutdown countdown
- 🚀 **Auto-Start** - Optionally start with Windows
- 🏠 **Location Status** - Shows "At Home" or "Roaming" in tray
- 📝 **File Logging** - Daily log rotation with auto-cleanup
- 🔄 **Retry Logic** - Automatic retries for network operations
- 💾 **State Persistence** - Phone detection state survives app restart
- 🛡️ **Input Validation** - All inputs sanitized and validated

## Quick Start

1. Download `home-sentry.exe` from [Releases](../../releases)
2. Run it - appears in system tray
3. Right-click tray icon:
   - Click "Set Current WiFi as Home"
   - Click "Select Monitored Device" → "🔄 Scan Network" → Choose your phone
4. Done! The app will monitor your phone's presence.

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                    HOME SENTRY                              │
├─────────────────────────────────────────────────────────────┤
│  Every 10 seconds (configurable):                           │
│  1. Check current WiFi network                              │
│  2. If on Home WiFi → Check ARP table for phone MAC         │
│  3. Phone MAC found? → Safe (🟢)                            │
│  4. Phone MAC missing? → Grace Period (🟡)                  │
│  5. Missing for 5 checks? → 10s Countdown → Shutdown (🔴)   │
│  6. Cancel available during countdown!                      │
└─────────────────────────────────────────────────────────────┘
```

## CLI Commands

```bash
# Show current status and all settings
home-sentry status

# Scan for network devices
home-sentry scan

# Scan for WiFi networks
home-sentry wifi

# Set home network
home-sentry set-home "MyWiFi"

# Set monitored device (MAC address)
home-sentry set-device AA:BB:CC:DD:EE:FF

# Pause/Resume protection
home-sentry pause
home-sentry resume

# Show version
home-sentry version

# View recent logs
home-sentry logs

# Run with system tray (default)
home-sentry
```

## Configuration

Settings are stored in `%APPDATA%\HomeSentry\settings.json` (automatically encrypted):

```json
{
  "home_ssid": "MyHomeWiFi",
  "phone_mac": "aa-bb-cc-dd-ee-ff",
  "detection_type": "mac",
  "is_paused": false,
  "grace_checks": 5,
  "poll_interval_sec": 10,
  "ping_timeout_ms": 500,
  "shutdown_action": "shutdown",
  "require_pin": false,
  "shutdown_pin": "",
  "confirmation_delay_sec": 10
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `home_ssid` | "" | Your home WiFi network name (encrypted) |
| `phone_mac` | "" | MAC address of your phone (AA:BB:CC:DD:EE:FF) (encrypted) |
| `detection_type` | "mac" | Detection method: "mac" (recommended) or "ip" |
| `is_paused` | false | Whether protection is paused |
| `grace_checks` | 5 | Number of failed checks before shutdown (1-100) |
| `poll_interval_sec` | 10 | Seconds between each check (1-300) |
| `ping_timeout_ms` | 500 | Ping timeout in milliseconds (100+) |
| `shutdown_action` | "shutdown" | Action on trigger: shutdown, hibernate, sleep, lock |
| `require_pin` | false | Require PIN for **manual** shutdown via UI (not automatic) |
| `shutdown_pin` | "" | 4-8 digit PIN (encrypted) |
| `confirmation_delay_sec` | 10 | Extra delay when using PIN for manual shutdown |

### File Locations

| File | Location |
|------|----------|
| Settings | `%APPDATA%\HomeSentry\settings.json` (encrypted) |
| State | `%APPDATA%\HomeSentry\sentry-state.json` |
| Logs | `%APPDATA%\HomeSentry\logs\home-sentry-YYYY-MM-DD.log` |
| Encryption Key | `%APPDATA%\HomeSentry\.key` |

### Security Features

- **AES-256-GCM Encryption** - All sensitive data is encrypted at rest
- **Input Validation** - All user inputs are validated and sanitized
- **PIN Protection** - Optional PIN for **manual** shutdown via UI only (automatic shutdown still works when you're not home)
- **State Persistence** - Phone detection state survives app restarts
- **Retry Logic** - Network operations retry automatically for reliability

## Building from Source

### Requirements
- Go 1.21+
- GCC (for CGO - required by systray)
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or MinGW-w64
- Make (optional)

### Build

```bash
# Using Make
make build

# Or manually
go build -ldflags="-H windowsgui -s -w -X main.Version=1.1.0" -o home-sentry.exe

# Run tests
make test
# or
go test -v ./...
```

## Troubleshooting

### Phone not detected?
- MAC detection works even if ping is blocked or IP changes
- Ensure your phone is connected to WiFi (not mobile data)
- Disable "Private WiFi Address" on iPhone (Settings → WiFi → [network] → Private Address OFF)
- Run `home-sentry scan` to verify your phone appears
- Check if MAC address format is correct (AA:BB:CC:DD:EE:FF)

### App shows warning even when phone is connected?
- The first check after setup may fail - wait 10-20 seconds
- Check if phone MAC is correct with `home-sentry status`
- View logs with `home-sentry logs` for debugging

### Where are my logs?
- Run `home-sentry logs` to view recent entries
- Full logs at: `%APPDATA%\HomeSentry\logs\`
- Logs older than 7 days are automatically deleted

## Development

```bash
# Run tests
make test

# Run tests with coverage
make test-coverage

# Lint code (requires golangci-lint)
make lint

# Format code
make fmt
```

## License

MIT License - See [LICENSE](LICENSE)

## Contributing

Pull requests welcome! Please:
1. Open an issue first to discuss major changes
2. Run `make test` and `make lint` before submitting
3. Update CHANGELOG.md with your changes

See [CHANGELOG.md](CHANGELOG.md) for version history.
