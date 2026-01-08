# Home Sentry 🏠🛡️

**Protect your laptop when you leave home.** Home Sentry monitors your home WiFi and phone presence - if your phone leaves but your laptop stays, it can trigger a shutdown to protect your data.

![Status](https://img.shields.io/badge/status-active-brightgreen)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![License](https://img.shields.io/badge/license-MIT-green)

## Features

- 🟢 **Safe** - Phone detected on home WiFi
- 🟡 **Warning** - Phone missing, grace period active
- 🔴 **Shutdown** - Grace period expired, protect your data
- ⏸️ **Pause** - Temporarily disable protection
- 📱 **Device Selection** - Scan and select your phone from network
- 🌐 **WiFi Detection** - Auto-detect home network
- 🛑 **Cancel Shutdown** - Abort pending shutdown
- 📝 **File Logging** - Daily log rotation with auto-cleanup

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
│  2. If on Home WiFi → Ping your phone                       │
│  3. Phone responding? → Safe (🟢)                            │
│  4. Phone missing? → Grace Period (🟡)                       │
│  5. Missing for 5 checks? → 10s Countdown → Shutdown (🔴)    │
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

# Set monitored device
home-sentry set-device 192.168.1.100

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

Settings are stored in `%APPDATA%\HomeSentry\settings.json`:

```json
{
  "home_ssid": "MyHomeWiFi",
  "phone_ip": "192.168.1.100",
  "is_paused": false,
  "grace_checks": 5,
  "poll_interval_sec": 10,
  "ping_timeout_ms": 500
}
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `home_ssid` | "" | Your home WiFi network name |
| `phone_ip` | "" | IP address of your phone to monitor |
| `is_paused` | false | Whether protection is paused |
| `grace_checks` | 5 | Number of failed pings before shutdown |
| `poll_interval_sec` | 10 | Seconds between each check |
| `ping_timeout_ms` | 500 | Ping timeout in milliseconds |

### File Locations

| File | Location |
|------|----------|
| Settings | `%APPDATA%\HomeSentry\settings.json` |
| Logs | `%APPDATA%\HomeSentry\logs\home-sentry-YYYY-MM-DD.log` |

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
- Ensure your phone has a **static IP** or **DHCP reservation**
- Disable "Private WiFi Address" on iPhone (Settings → WiFi → [network] → Private Address OFF)
- Some phones block ICMP ping - try disabling firewall temporarily
- Increase `ping_timeout_ms` to 1000 or higher in settings.json

### App shows warning even when phone is connected?
- The first ping after setup may fail - wait 10-20 seconds
- Check if phone IP is correct with `home-sentry status`
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
