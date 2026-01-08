package main

import (
	"fmt"
	"home-sentry/assets"
	"home-sentry/pkg/config"
	"home-sentry/pkg/network"
	"home-sentry/pkg/sentry"
	"os"
	"time"

	"fyne.io/systray"
)

const Version = "1.1.0"

var (
	sentryManager   *sentry.SentryManager
	mStatus         *systray.MenuItem
	mWiFi           *systray.MenuItem
	mPhoneIP        *systray.MenuItem
	mPause          *systray.MenuItem
	mCancelShutdown *systray.MenuItem
	deviceSubmenus  []*systray.MenuItem
)

func main() {
	if len(os.Args) < 2 {
		// Default: run with tray
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
			fmt.Println("Usage: home-sentry set-device <ip>")
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
	default:
		printHelp()
	}
}

func runWithTray() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(assets.IconGreen)
	systray.SetTitle("Home Sentry")
	systray.SetTooltip("Home Sentry - Monitoring")

	// Load initial settings
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	// Status info
	mStatus = systray.AddMenuItem("Status: Starting...", "Current status")
	mStatus.Disable()

	mWiFi = systray.AddMenuItem(fmt.Sprintf("WiFi: %s", currentSSID), "Current WiFi network")
	mWiFi.Disable()

	mPhoneIP = systray.AddMenuItem(fmt.Sprintf("Phone: %s", settings.PhoneIP), "Monitored device IP")
	mPhoneIP.Disable()

	mVersion := systray.AddMenuItem(fmt.Sprintf("Version: %s", Version), "Application version")
	mVersion.Disable()

	systray.AddSeparator()

	// Actions
	mSetHome := systray.AddMenuItem("Set Current WiFi as Home", "Use current network as home")

	// Device selector with submenu
	mSelectDevice := systray.AddMenuItem("Select Monitored Device", "Choose device from network")
	mScanDevices := mSelectDevice.AddSubMenuItem("🔄 Scan Network", "Scan for devices")

	mPause = systray.AddMenuItem("Pause Protection", "Temporarily disable protection")

	mCancelShutdown = systray.AddMenuItem("⚠️ Cancel Shutdown", "Cancel pending shutdown")
	mCancelShutdown.Hide() // Hidden until shutdown is pending

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Exit Home Sentry")

	// Start sentry in background
	sentryManager = sentry.NewSentryManager()
	sentryManager.SetStatusCallback(onStatusChange)
	go sentryManager.StartMonitor()

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mSetHome.ClickedCh:
				ssid := network.GetCurrentSSID()
				config.Update(ssid, "")
				updateInfoDisplay()
			case <-mScanDevices.ClickedCh:
				scanAndPopulateDevices(mSelectDevice)
			case <-mPause.ClickedCh:
				settings, _ := config.Load()
				if settings.IsPaused {
					config.SetPaused(false)
					mPause.SetTitle("Pause Protection")
				} else {
					config.SetPaused(true)
					mPause.SetTitle("Resume Protection")
				}
			case <-mCancelShutdown.ClickedCh:
				if sentryManager.CancelShutdown() {
					mCancelShutdown.Hide()
					if mStatus != nil {
						mStatus.SetTitle("Status: Shutdown Cancelled")
					}
				}
			case <-mQuit.ClickedCh:
				systray.Quit()
			}
		}
	}()

	// Update display periodically
	go func() {
		for {
			updateInfoDisplay()
			time.Sleep(5 * time.Second)
		}
	}()
}

func updateInfoDisplay() {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	if mWiFi != nil {
		mWiFi.SetTitle(fmt.Sprintf("WiFi: %s", currentSSID))
	}
	if mPhoneIP != nil {
		if settings.PhoneIP != "" {
			mPhoneIP.SetTitle(fmt.Sprintf("Phone: %s", settings.PhoneIP))
		} else {
			mPhoneIP.SetTitle("Phone: Not Set")
		}
	}

	// Show/hide cancel shutdown based on state
	if sentryManager != nil && mCancelShutdown != nil {
		if sentryManager.IsShutdownPending() {
			mCancelShutdown.Show()
		} else {
			mCancelShutdown.Hide()
		}
	}
}

func scanAndPopulateDevices(parentMenu *systray.MenuItem) {
	// Clear old device submenus
	for _, item := range deviceSubmenus {
		item.Hide()
	}
	deviceSubmenus = nil

	// Scan network
	if mStatus != nil {
		mStatus.SetTitle("Status: Scanning network...")
	}

	devices := network.ScanNetworkDevices()

	if len(devices) == 0 {
		noDevices := parentMenu.AddSubMenuItem("No devices found", "")
		noDevices.Disable()
		deviceSubmenus = append(deviceSubmenus, noDevices)
		if mStatus != nil {
			mStatus.SetTitle("Status: No devices found")
		}
		return
	}

	// Add each device as a submenu item
	for _, device := range devices {
		deviceName := fmt.Sprintf("%s (%s)", device.Hostname, device.IP)
		if device.Hostname == "Unknown" {
			deviceName = device.IP
		}

		deviceItem := parentMenu.AddSubMenuItem(deviceName, fmt.Sprintf("MAC: %s", device.MAC))
		deviceSubmenus = append(deviceSubmenus, deviceItem)

		// Capture the IP for the closure
		deviceIP := device.IP

		// Handle device selection
		go func(ip string, item *systray.MenuItem) {
			<-item.ClickedCh
			config.Update("", ip)
			updateInfoDisplay()
			if mStatus != nil {
				mStatus.SetTitle(fmt.Sprintf("Device set: %s", ip))
			}
		}(deviceIP, deviceItem)
	}

	if mStatus != nil {
		mStatus.SetTitle(fmt.Sprintf("Status: Found %d devices", len(devices)))
	}
}

func onStatusChange(status sentry.SentryStatus) {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()

	switch status {
	case sentry.StatusMonitoring:
		systray.SetIcon(assets.IconGreen)
		systray.SetTooltip(fmt.Sprintf("Home Sentry - Safe\nWiFi: %s\nPhone: %s", currentSSID, settings.PhoneIP))
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
	// Cleanup
}

func printHelp() {
	fmt.Printf("Home Sentry v%s - CLI\n", Version)
	fmt.Println("Usage:")
	fmt.Println("  (no args)         Start with system tray")
	fmt.Println("  scan              Scan local network for devices")
	fmt.Println("  wifi              Scan for available WiFi networks")
	fmt.Println("  status            Show current status and settings")
	fmt.Println("  set-home <ssid>   Set your home network SSID")
	fmt.Println("  set-device <ip>   Set your monitored device IP")
	fmt.Println("  pause             Pause protection")
	fmt.Println("  resume            Resume protection")
	fmt.Println("  version           Show version")
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
	fmt.Printf("Phone IP:       %s\n", settings.PhoneIP)
	fmt.Printf("Paused:         %v\n", settings.IsPaused)
	fmt.Printf("Grace Checks:   %d\n", settings.GraceChecks)
	fmt.Printf("Poll Interval:  %ds\n", settings.PollInterval)
	fmt.Printf("Ping Timeout:   %dms\n", settings.PingTimeoutMs)
	fmt.Printf("Settings File:  %s\n", config.GetSettingsPath())

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
}

func runSetDevice(ip string) {
	if !config.ValidateIP(ip) {
		fmt.Printf("Error: Invalid IP address: %s\n", ip)
		return
	}
	err := config.Update("", ip)
	if err != nil {
		fmt.Println("Error saving settings:", err)
		return
	}
	fmt.Printf("Monitored Device IP updated to: %s\n", ip)
}

func runSetPaused(paused bool) {
	err := config.SetPaused(paused)
	if err != nil {
		fmt.Println("Error saving settings:", err)
		return
	}
	if paused {
		fmt.Println("Protection PAUSED.")
	} else {
		fmt.Println("Protection RESUMED.")
	}
}
