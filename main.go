package main

import (
	"context"
	"fmt"
	"home-sentry/assets"
	"home-sentry/pkg/config"
	"home-sentry/pkg/logger"
	"home-sentry/pkg/network"
	"home-sentry/pkg/ntfy"
	"home-sentry/pkg/sentry"
	"home-sentry/pkg/startup"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
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
	mShutdownTimer  *systray.MenuItem
	mCancelShutdown *systray.MenuItem
	mNtfyEnabled    *systray.MenuItem
	mNtfyTopic      *systray.MenuItem
	mNtfyTest       *systray.MenuItem
	deviceSubmenus  []*systray.MenuItem
	cachedDevices   []network.NetworkDevice
	hasScanned      bool
	scanMutex       sync.Mutex
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
		if fyneApp != nil {
			fyneApp.Quit()
		}
		systray.Quit()
	}()

	// Initialize Fyne app and custom menu
	initFyneApp()

	// Run Fyne event loop in background
	go runFyneApp()

	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(assets.IconGreen)
	systray.SetTitle("Home Sentry")
	systray.SetTooltip("Home Sentry - Click to open menu")

	// Note: We still add a minimal native menu as backup
	// but the primary interaction is via the Fyne popup window

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
	mScanDevices := mSelectDevice.AddSubMenuItem("🔄 Scan Network...", "Refresh network device list")

	// Start auto-scan in background
	go func() {
		// Wait a moment for tray to settle
		time.Sleep(1 * time.Second)
		scanAndPopulateDevices(mSelectDevice, false)
	}()

	systray.AddSeparator()

	mPause = systray.AddMenuItem("⏸️ Pause Protection", "Temporarily disable protection")

	// Auto-start toggle
	autoStartText := "🚀 Enable Auto-Start"
	if startup.IsEnabled() {
		autoStartText = "✅ Auto-Start Enabled"
	}
	mAutoStart = systray.AddMenuItem(autoStartText, "Start Home Sentry when Windows starts")

	mShutdownTimer = systray.AddMenuItem("⏱ Shutdown Timer", "Set delay before shutdown")
	setupShutdownTimerMenu()

	mCancelShutdown = systray.AddMenuItem("⚠️ Cancel Shutdown", "Cancel pending shutdown")
	mCancelShutdown.Hide()

	// ntfy.sh notifications submenu
	mNtfy := systray.AddMenuItem("🔔 Phone Notifications", "Configure ntfy.sh notifications")
	ntfyEnabledText := "Enable Notifications"
	if settings.NtfyEnabled {
		ntfyEnabledText = "✅ Notifications Enabled"
	}
	mNtfyEnabled = mNtfy.AddSubMenuItem(ntfyEnabledText, "Toggle ntfy.sh notifications")
	topicDisplay := "Not Set"
	if settings.NtfyTopic != "" {
		topicDisplay = settings.NtfyTopic
	}
	mNtfyTopic = mNtfy.AddSubMenuItem(fmt.Sprintf("📝 Topic: %s", topicDisplay), "ntfy.sh topic name")
	mNtfyTopic.Disable()
	mNtfyTest = mNtfy.AddSubMenuItem("🧪 Send Test Notification", "Test that notifications work")
	if !settings.NtfyEnabled || settings.NtfyTopic == "" {
		mNtfyTest.Disable()
	}

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("❌ Quit", "Exit Home Sentry")

	// Start sentry in background
	sentryManager = sentry.NewSentryManager()
	sentryManager.SetStatusCallback(onStatusChange)
	go sentryManager.StartMonitor()

	// Start ntfy command listener if enabled
	if settings.NtfyEnabled && settings.NtfyTopic != "" {
		go startNtfyCommandListener(settings)
	}

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
				scanAndPopulateDevices(mSelectDevice, true)
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
			case <-mNtfyEnabled.ClickedCh:
				settings, _ := config.Load()
				if settings.NtfyEnabled {
					// Disable ntfy
					settings.NtfyEnabled = false
					config.Save(settings)
					mNtfyEnabled.SetTitle("Enable Notifications")
					mNtfyTest.Disable()
					logger.Info("ntfy notifications disabled")
				} else {
					// Enable ntfy - prompt for topic if not set
					if settings.NtfyTopic == "" {
						// Generate a random topic
						settings.NtfyTopic = fmt.Sprintf("home-sentry-%d", time.Now().UnixNano()%1000000)
						logger.Info("Generated ntfy topic: %s", settings.NtfyTopic)
					}
					settings.NtfyEnabled = true
					config.Save(settings)
					mNtfyEnabled.SetTitle("✅ Notifications Enabled")
					mNtfyTopic.SetTitle(fmt.Sprintf("📝 Topic: %s", settings.NtfyTopic))
					mNtfyTest.Enable()
					logger.Info("ntfy notifications enabled with topic: %s", settings.NtfyTopic)
				}
			case <-mNtfyTest.ClickedCh:
				settings, _ := config.Load()
				if settings.NtfyEnabled && settings.NtfyTopic != "" {
					client := ntfy.NewClient(settings.NtfyServer, settings.NtfyTopic)
					if err := client.SendTestNotification(); err != nil {
						logger.Error("Failed to send test notification: %v", err)
						if mStatus != nil {
							mStatus.SetTitle("❌ ntfy test failed")
						}
					} else {
						logger.Info("Test notification sent to topic: %s", settings.NtfyTopic)
						if mStatus != nil {
							mStatus.SetTitle("✅ Test notification sent!")
						}
					}
				}
			case <-mQuit.ClickedCh:
				logger.Info("User requested quit")
				systray.Quit()

			// Handle clicks on informational items (just logger debug)
			case <-mStatus.ClickedCh:
				logger.Debug("Status clicked")
			case <-mLocation.ClickedCh:
				logger.Debug("Location clicked")
			case <-mWiFi.ClickedCh:
				logger.Debug("WiFi clicked")
			case <-mPhoneMAC.ClickedCh:
				logger.Debug("Phone MAC clicked")
			case <-mVersion.ClickedCh:
				logger.Debug("Version clicked")
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

// startNtfyCommandListener starts an always-on listener for phone commands
func startNtfyCommandListener(settings config.Settings) {
	client := ntfy.NewClient(settings.NtfyServer, settings.NtfyTopic)

	err := client.StartCommandListener(func(cmd ntfy.Command) {
		logger.Info("Received ntfy command: %s", cmd)

		switch cmd {
		case ntfy.CmdPause:
			settings, _ := config.Load()
			if !settings.IsPaused {
				config.SetPaused(true)
				if mPause != nil {
					mPause.SetTitle("▶️ Resume Protection")
				}
				if mStatus != nil {
					mStatus.SetTitle("Status: Paused ⏸")
				}
				logger.Info("Protection paused via ntfy")
				// Send confirmation
				go client.SendPausedNotification()
			}

		case ntfy.CmdResume:
			settings, _ := config.Load()
			if settings.IsPaused {
				config.SetPaused(false)
				if mPause != nil {
					mPause.SetTitle("⏸️ Pause Protection")
				}
				if mStatus != nil {
					mStatus.SetTitle("Status: Resumed")
				}
				logger.Info("Protection resumed via ntfy")
				// Send confirmation
				go client.SendResumedNotification()
			}

		case ntfy.CmdStatus:
			settings, _ := config.Load()
			ssid := network.GetCurrentSSID()
			var status string
			if settings.IsPaused {
				status = "Paused"
			} else if ssid == settings.HomeSSID {
				status = "At Home - Monitoring"
			} else {
				status = "Roaming"
			}
			go client.SendStatusNotification(status, ssid, settings.PhoneMAC, settings.IsPaused)
			logger.Info("Status sent via ntfy")
		}
	})

	if err != nil {
		logger.Error("Failed to start ntfy command listener: %v", err)
	}
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

	if mShutdownTimer != nil {
		mShutdownTimer.SetTitle(fmt.Sprintf("⏱ Shutdown Timer (%ds)", settings.ShutdownDelay))
	}

	if sentryManager != nil && mCancelShutdown != nil {
		if sentryManager.IsShutdownPending() {
			mCancelShutdown.Show()
		} else {
			mCancelShutdown.Hide()
		}
	}
}

func setupShutdownTimerMenu() {
	delays := []struct {
		Seconds int
		Label   string
	}{
		{10, "10 Seconds"},
		{30, "30 Seconds"},
		{60, "1 Minute"},
		{300, "5 Minutes"},
	}

	for _, d := range delays {
		m := mShutdownTimer.AddSubMenuItem(d.Label, fmt.Sprintf("Wait %s before shutdown", d.Label))
		go func(val int, m *systray.MenuItem) {
			for range m.ClickedCh {
				config.SetShutdownDelay(val)
				updateInfoDisplay()
			}
		}(d.Seconds, m)
	}
}

func scanAndPopulateDevices(parentMenu *systray.MenuItem, forceRefresh bool) {
	scanMutex.Lock()
	defer scanMutex.Unlock()

	// Use cache if available and not forced
	if !forceRefresh && hasScanned && len(cachedDevices) > 0 {
		logger.Info("Using cached network devices")
		populateDeviceMenu(parentMenu, cachedDevices)
		return
	}

	// Helper to clear menu
	for _, item := range deviceSubmenus {
		item.Hide()
	}
	deviceSubmenus = nil

	if mStatus != nil {
		mStatus.SetTitle("⏳ Scanning network...")
	}
	logger.Info("Starting network scan (force=%v)", forceRefresh)

	devices := network.ScanNetworkDevices()
	cachedDevices = devices
	hasScanned = true

	logger.Info("Found %d devices", len(devices))
	populateDeviceMenu(parentMenu, devices)
}

func populateDeviceMenu(parentMenu *systray.MenuItem, devices []network.NetworkDevice) {
	// Clear previous device entries (again, to be safe if called from cache path)
	for _, item := range deviceSubmenus {
		item.Hide()
	}
	deviceSubmenus = nil

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
		// Format: "📱 IP / MAC / Vendor" (as requested)
		// Include Hostname if known
		var label string
		if device.Hostname != "Unknown" && device.Hostname != "" {
			label = fmt.Sprintf("📱 %s (%s) / %s / %s", device.Hostname, device.IP, device.MAC, device.Vendor)
		} else {
			label = fmt.Sprintf("📱 %s / %s / %s", device.IP, device.MAC, device.Vendor)
		}

		// Tooltip shows detailed info
		tooltip := fmt.Sprintf("Click to monitor • IP: %s\nMAC: %s\nVendor: %s\nHostname: %s",
			device.IP, device.MAC, device.Vendor, device.Hostname)

		deviceItem := parentMenu.AddSubMenuItem(label, tooltip)
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
