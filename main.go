package main

import (
	"context"
	"fmt"
	"home-sentry/assets"
	"home-sentry/pkg/config"
	"home-sentry/pkg/logger"
	"home-sentry/pkg/network"
	"home-sentry/pkg/sentry"
	"home-sentry/pkg/startup"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fyne.io/systray"
)

// Version is set via ldflags at build time
var Version = "dev"

var (
	sentryManager   *sentry.SentryManager
	mStatus         *systray.MenuItem
	mLocation       *systray.MenuItem
	mWiFi           *systray.MenuItem
	mPhoneMAC       *systray.MenuItem
	mPause          *systray.MenuItem
	mAutoStart      *systray.MenuItem
	mCancelShutdown *systray.MenuItem
	deviceSubmenus  []*systray.MenuItem
	ctx             context.Context
	cancel          context.CancelFunc
)

func main() {
	// Initialize logger
	logDir := logger.GetLogDir()
	if err := logger.Init(logDir, logger.INFO); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		// Continue without file logging
	}

	logger.Info("Home Sentry v%s starting", Version)

	if len(os.Args) < 2 {
		runWithTray()
		return
	}

	command := os.Args[1]

	switch command {
	case "scan":
		runScan()
	case "wifi":
		runWifiScan()
	case "status":
		runStatus()
	case "set-home":
		if len(os.Args) < 3 {
			fmt.Println("Usage: home-sentry set-home <ssid>")
			return
		}
		runSetHome(os.Args[2])
	case "set-device":
		if len(os.Args) < 3 {
			fmt.Println("Usage: home-sentry set-device <mac>")
			fmt.Println("Format: AA:BB:CC:DD:EE:FF or AA-BB-CC-DD-EE-FF")
			return
		}
		runSetDevice(os.Args[2])
	case "pause":
		runSetPaused(true)
	case "resume":
		runSetPaused(false)
	case "run":
		runWithTray()
	case "version":
		fmt.Printf("Home Sentry v%s\n", Version)
	case "logs":
		runShowLogs()
	default:
		printHelp()
	}
}

func runWithTray() {
	// Setup graceful shutdown
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		logger.Info("Received signal %v, shutting down", sig)
		cancel()
		systray.Quit()
	}()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(assets.IconGreen)
	systray.SetTitle("Home Sentry")
	systray.SetTooltip("Home Sentry - Monitoring")

	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	logger.Info("Tray ready. SSID: %s, Home: %s, Phone MAC: %s", currentSSID, settings.HomeSSID, settings.PhoneMAC)

	// Status info
	mStatus = systray.AddMenuItem("Status: Starting...", "Current status")
	mStatus.Disable()

	// Location status (At Home / Roaming)
	locationText := "📍 Roaming"
	if currentSSID == settings.HomeSSID && settings.HomeSSID != "" {
		locationText = "🏠 At Home"
	}
	mLocation = systray.AddMenuItem(locationText, "Current location")
	mLocation.Disable()

	mWiFi = systray.AddMenuItem(fmt.Sprintf("📶 WiFi: %s", currentSSID), "Current WiFi network")
	mWiFi.Disable()

	phoneDisplay := "Not Set"
	if settings.PhoneMAC != "" {
		phoneDisplay = settings.PhoneMAC
	}
	mPhoneMAC = systray.AddMenuItem(fmt.Sprintf("📱 Phone: %s", phoneDisplay), "Monitored device MAC")
	mPhoneMAC.Disable()

	mVersion := systray.AddMenuItem(fmt.Sprintf("ℹ️ Version: %s", Version), "Application version")
	mVersion.Disable()

	systray.AddSeparator()

	// Actions
	mSetHome := systray.AddMenuItem("🏠 Set Current WiFi as Home", "Use current network as home")
	mSelectDevice := systray.AddMenuItem("📱 Select Monitored Device", "Choose device from network")
	mScanDevices := mSelectDevice.AddSubMenuItem("🔄 Scan Network...", "Scan for devices on your network")

	systray.AddSeparator()

	mPause = systray.AddMenuItem("⏸️ Pause Protection", "Temporarily disable protection")

	// Auto-start toggle
	autoStartText := "🚀 Enable Auto-Start"
	if startup.IsEnabled() {
		autoStartText = "✅ Auto-Start Enabled"
	}
	mAutoStart = systray.AddMenuItem(autoStartText, "Start Home Sentry when Windows starts")

	mCancelShutdown = systray.AddMenuItem("⚠️ Cancel Shutdown", "Cancel pending shutdown")
	mCancelShutdown.Hide()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("❌ Quit", "Exit Home Sentry")

	// Start sentry in background
	sentryManager = sentry.NewSentryManager()
	sentryManager.SetStatusCallback(onStatusChange)
	go sentryManager.StartMonitor()

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-mSetHome.ClickedCh:
				ssid := network.GetCurrentSSID()
				if err := config.Update(ssid, ""); err != nil {
					logger.Error("Failed to set home SSID: %v", err)
				} else {
					logger.Info("Home SSID set to: %s", ssid)
				}
				updateInfoDisplay()
			case <-mScanDevices.ClickedCh:
				scanAndPopulateDevices(mSelectDevice)
			case <-mPause.ClickedCh:
				settings, _ := config.Load()
				if settings.IsPaused {
					config.SetPaused(false)
					mPause.SetTitle("⏸️ Pause Protection")
					logger.Info("Protection resumed")
				} else {
					config.SetPaused(true)
					mPause.SetTitle("▶️ Resume Protection")
					logger.Info("Protection paused")
				}
			case <-mAutoStart.ClickedCh:
				enabled, err := startup.Toggle()
				if err != nil {
					logger.Error("Failed to toggle auto-start: %v", err)
				} else {
					if enabled {
						mAutoStart.SetTitle("✅ Auto-Start Enabled")
						logger.Info("Auto-start enabled")
					} else {
						mAutoStart.SetTitle("🚀 Enable Auto-Start")
						logger.Info("Auto-start disabled")
					}
				}
			case <-mCancelShutdown.ClickedCh:
				if sentryManager.CancelShutdown() {
					mCancelShutdown.Hide()
					if mStatus != nil {
						mStatus.SetTitle("Status: Shutdown Cancelled")
					}
					logger.Info("Shutdown cancelled by user")
				}
			case <-mQuit.ClickedCh:
				logger.Info("User requested quit")
				systray.Quit()
			}
		}
	}()

	// Update display periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				updateInfoDisplay()
			}
		}
	}()
}

func updateInfoDisplay() {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	// Update location status
	if mLocation != nil {
		if currentSSID == settings.HomeSSID && settings.HomeSSID != "" {
			mLocation.SetTitle("🏠 At Home")
		} else {
			mLocation.SetTitle("📍 Roaming")
		}
	}

	if mWiFi != nil {
		mWiFi.SetTitle(fmt.Sprintf("📶 WiFi: %s", currentSSID))
	}
	if mPhoneMAC != nil {
		if settings.PhoneMAC != "" {
			mPhoneMAC.SetTitle(fmt.Sprintf("📱 Phone: %s", settings.PhoneMAC))
		} else {
			mPhoneMAC.SetTitle("📱 Phone: Not Set")
		}
	}

	if sentryManager != nil && mCancelShutdown != nil {
		if sentryManager.IsShutdownPending() {
			mCancelShutdown.Show()
		} else {
			mCancelShutdown.Hide()
		}
	}
}

func scanAndPopulateDevices(parentMenu *systray.MenuItem) {
	// Clear previous device entries
	for _, item := range deviceSubmenus {
		item.Hide()
	}
	deviceSubmenus = nil

	if mStatus != nil {
		mStatus.SetTitle("⏳ Scanning network...")
	}
	logger.Info("Starting network scan")

	devices := network.ScanNetworkDevices()
	logger.Info("Found %d devices", len(devices))

	if len(devices) == 0 {
		noDevices := parentMenu.AddSubMenuItem("❌ No devices found", "Try again or check WiFi connection")
		noDevices.Disable()
		deviceSubmenus = append(deviceSubmenus, noDevices)
		if mStatus != nil {
			mStatus.SetTitle("Status: No devices found")
		}
		return
	}

	// Add header showing device count
	header := parentMenu.AddSubMenuItem(fmt.Sprintf("── Found %d devices ──", len(devices)), "")
	header.Disable()
	deviceSubmenus = append(deviceSubmenus, header)

	for _, device := range devices {
		// Format: "📱 Hostname (192.168.1.x)"
		// Or just IP if hostname unknown
		var deviceName string
		if device.Hostname != "Unknown" && device.Hostname != "" {
			deviceName = fmt.Sprintf("📱 %s (%s)", device.Hostname, device.IP)
		} else {
			deviceName = fmt.Sprintf("📱 %s", device.IP)
		}

		// Tooltip shows MAC address
		tooltip := fmt.Sprintf("Click to monitor • MAC: %s", device.MAC)
		deviceItem := parentMenu.AddSubMenuItem(deviceName, tooltip)
		deviceSubmenus = append(deviceSubmenus, deviceItem)

		// Capture values for the goroutine
		deviceMAC := device.MAC
		deviceHostname := device.Hostname
		if deviceHostname == "Unknown" || deviceHostname == "" {
			deviceHostname = device.IP
		}

		go func(mac string, name string, item *systray.MenuItem) {
			<-item.ClickedCh
			if err := config.Update("", mac); err != nil {
				logger.Error("Failed to set device MAC: %v", err)
			} else {
				logger.Info("Device MAC set to: %s (%s)", mac, name)
			}
			updateInfoDisplay()
			if mStatus != nil {
				mStatus.SetTitle(fmt.Sprintf("✅ Monitoring: %s", name))
			}
		}(deviceMAC, deviceHostname, deviceItem)
	}

	if mStatus != nil {
		mStatus.SetTitle(fmt.Sprintf("Found %d devices - select one", len(devices)))
	}
}

func onStatusChange(status sentry.SentryStatus) {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	logger.Debug("Status changed to: %s", status)

	switch status {
	case sentry.StatusMonitoring:
		systray.SetIcon(assets.IconGreen)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - Safe\nWiFi: %s\nPhone MAC: %s", currentSSID, settings.PhoneMAC))
		systray.SetTitle("🟢")
		if mStatus != nil {
			mStatus.SetTitle("Status: Safe 🟢")
		}
	case sentry.StatusGracePeriod:
		systray.SetIcon(assets.IconYellow)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - WARNING\nPhone not detected!\nWiFi: %s", currentSSID))
		systray.SetTitle("🟡")
		if mStatus != nil {
			mStatus.SetTitle("Status: Warning 🟡")
		}
	case sentry.StatusShutdownImminent:
		systray.SetIcon(assets.IconRed)
		systray.SetTooltip("Home Sentry - DANGER\nShutdown imminent!")
		systray.SetTitle("🔴")
		if mStatus != nil {
			mStatus.SetTitle("Status: SHUTDOWN 🔴")
		}
		if mCancelShutdown != nil {
			mCancelShutdown.Show()
		}
	case sentry.StatusPaused:
		systray.SetIcon(assets.IconYellow)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - Paused\nProtection disabled\nWiFi: %s", currentSSID))
		systray.SetTitle("⏸")
		if mStatus != nil {
			mStatus.SetTitle("Status: Paused ⏸")
		}
	case sentry.StatusWaitingForPhone:
		systray.SetIcon(assets.IconYellow)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - Waiting\nWaiting for phone...\nWiFi: %s", currentSSID))
		systray.SetTitle("📱")
		if mStatus != nil {
			mStatus.SetTitle("Status: Waiting for Phone 📱")
		}
	default:
		systray.SetIcon(assets.IconGreen)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - Roaming\nWiFi: %s", currentSSID))
		systray.SetTitle("🌐")
		if mStatus != nil {
			mStatus.SetTitle("Status: Roaming")
		}
	}
}

func onExit() {
	logger.Info("Home Sentry shutting down")
	if cancel != nil {
		cancel()
	}
}

func printHelp() {
	fmt.Printf("Home Sentry v%s - CLI\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  (no args)         Start with system tray")
	fmt.Println("  scan              Scan local network for devices")
	fmt.Println("  wifi              Scan for available WiFi networks")
	fmt.Println("  status            Show current status and settings")
	fmt.Println("  set-home <ssid>   Set your home network SSID")
	fmt.Println("  set-device <mac>   Set monitored device MAC address")
	fmt.Println("  pause             Pause protection")
	fmt.Println("  resume            Resume protection")
	fmt.Println("  version           Show version")
	fmt.Println("  logs              Show recent log entries")
	fmt.Println("  run               Start with system tray")
}

func runScan() {
	fmt.Println("Scanning network (this may take a few seconds)...")
	devices := network.ScanNetworkDevices()

	fmt.Println("IP\t\t\tMAC\t\t\tHostname")
	fmt.Println("---------------------------------------------------------")
	for _, d := range devices {
		fmt.Printf("%-20s\t%-20s\t%s\n", d.IP, d.MAC, d.Hostname)
	}
}

func runWifiScan() {
	fmt.Println("Scanning WiFi networks...")
	ssids := network.ScanWifiNetworks()
	seen := make(map[string]bool)

	for _, ssid := range ssids {
		if !seen[ssid] {
			fmt.Println("- " + ssid)
			seen[ssid] = true
		}
	}
}

func runStatus() {
	settings, err := config.Load()
	if err != nil {
		fmt.Println("Error loading settings:", err)
		return
	}

	currentSSID := network.GetCurrentSSID()
	fmt.Printf("Home Sentry v%s\n", Version)
	fmt.Println("-------------------")
	fmt.Printf("Current SSID:   %s\n", currentSSID)
	fmt.Printf("Home SSID:      %s\n", settings.HomeSSID)
	fmt.Printf("Phone MAC:      %s\n", settings.PhoneMAC)
	fmt.Printf("Detection:      %s\n", settings.DetectionType)
	fmt.Printf("Paused:         %v\n", settings.IsPaused)
	fmt.Printf("Grace Checks:   %d\n", settings.GraceChecks)
	fmt.Printf("Poll Interval:  %ds\n", settings.PollInterval)
	fmt.Printf("Ping Timeout:   %dms\n", settings.PingTimeoutMs)
	fmt.Printf("Settings File:  %s\n", config.GetSettingsPath())
	fmt.Printf("Log Directory:  %s\n", logger.GetLogDir())

	if currentSSID == settings.HomeSSID {
		fmt.Println("Status:         AT HOME")
	} else {
		fmt.Println("Status:         ROAMING")
	}
}

func runSetHome(ssid string) {
	err := config.Update(ssid, "")
	if err != nil {
		fmt.Println("Error saving settings:", err)
		return
	}
	fmt.Printf("Home SSID updated to: %s\n", ssid)
	logger.Info("Home SSID set via CLI: %s", ssid)
}

func runSetDevice(mac string) {
	if !config.ValidateMAC(mac) {
		fmt.Printf("Error: Invalid MAC address: %s\n", mac)
		fmt.Println("Format: AA:BB:CC:DD:EE:FF or AA-BB-CC-DD-EE-FF")
		return
	}
	err := config.Update("", mac)
	if err != nil {
		fmt.Println("Error saving settings:", err)
		return
	}
	fmt.Printf("Monitored Device MAC updated to: %s\n", mac)
	logger.Info("Device MAC set via CLI: %s", mac)
}

func runSetPaused(paused bool) {
	err := config.SetPaused(paused)
	if err != nil {
		fmt.Println("Error saving settings:", err)
		return
	}
	if paused {
		fmt.Println("Protection PAUSED.")
		logger.Info("Protection paused via CLI")
	} else {
		fmt.Println("Protection RESUMED.")
		logger.Info("Protection resumed via CLI")
	}
}

func runShowLogs() {
	logs, err := logger.GetRecentLogs(20)
	if err != nil {
		fmt.Println("Error reading logs:", err)
		return
	}

	fmt.Printf("Recent logs from: %s\n", logger.GetLogDir())
	fmt.Println("-------------------")
	for _, line := range logs {
		if line != "" {
			fmt.Println(line)
		}
	}
}
