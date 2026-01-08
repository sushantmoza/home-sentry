# Home Sentry 🏠🛡️

**Protect your laptop when you leave home.** Home Sentry monitors your home WiFi and phone presence - if your phone leaves but your laptop stays, it can trigger a shutdown to protect your data.

![Status](https://img.shields.io/badge/status-active-brightgreen)
![Platform](https://img.shields.io/badge/platform-Windows-blue)
![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)

## Features

- 🟢 **Safe** - Phone detected on home WiFi
- 🟡 **Warning** - Phone missing, grace period active
- 🔴 **Shutdown** - Grace period expired, protect your data
- ⏸️ **Pause** - Temporarily disable protection
- 📱 **Device Selection** - Scan and select your phone from network
- 🌐 **WiFi Detection** - Auto-detect home network

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
│  Every 10 seconds:                                          │
│  1. Check current WiFi network                              │
│  2. If on Home WiFi → Ping your phone                       │
│  3. Phone responding? → Safe (🟢)                            │
│  4. Phone missing? → Grace Period (🟡)                       │
│  5. Missing for 5 checks? → Shutdown (🔴)                    │
└─────────────────────────────────────────────────────────────┘
```

## CLI Commands

```bash
# Show current status
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

# Run with system tray (default)
home-sentry
```

## Building from Source

### Requirements
- Go 1.21+
- GCC (for CGO - required by systray)
  - Windows: [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) or MinGW-w64

### Build
```bash
git clone https://github.com/yourusername/home-sentry.git
cd home-sentry
go build -ldflags="-H windowsgui" -o home-sentry.exe
```

## Configuration

Settings are stored in `settings.json` in the same directory:

```json
{
  "home_ssid": "MyHomeWiFi",
  "phone_ip": "192.168.1.100",
  "is_paused": false
}
```

## Troubleshooting

### Phone not detected?
- Ensure your phone has a **static IP** or **DHCP reservation**
- Disable "Private WiFi Address" on iPhone (Settings → WiFi → [network] → Private Address OFF)
- Some phones block ICMP ping - try disabling firewall temporarily

### App shows warning even when phone is connected?
- The first ping after setup may fail - wait 10-20 seconds
- Check if phone IP is correct with `home-sentry status`

## License

MIT License - See [LICENSE](LICENSE)

## Contributing

Pull requests welcome! Please open an issue first to discuss major changes.
