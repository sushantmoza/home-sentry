package main

import (
	"fmt"
	"home-sentry/pkg/config"
	"home-sentry/pkg/custommenu"
	"home-sentry/pkg/logger"
	"home-sentry/pkg/network"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

var (
	fyneApp   fyne.App
	popupMenu *custommenu.PopupMenu

	// Menu item references for dynamic updates
	menuStatus        *custommenu.MenuItem
	menuLocation      *custommenu.MenuItem
	menuWiFi          *custommenu.MenuItem
	menuPhoneMAC      *custommenu.MenuItem
	menuVersion       *custommenu.MenuItem
	menuPause         *custommenu.MenuItem
	menuShutdownTimer *custommenu.MenuItem
)

// initFyneApp initializes the Fyne application and custom menu
func initFyneApp() {
	fyneApp = app.NewWithID("com.homesentry.app")
	fyneApp.Settings().SetTheme(&custommenu.CustomTheme{})

	popupMenu = custommenu.NewPopupMenu(fyneApp, "Home Sentry")
	buildCustomMenu()
}

// buildCustomMenu creates all menu items
func buildCustomMenu() {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()
	safeSSID := config.SanitizeDisplayString(currentSSID)

	// Status info (disabled/grayed)
	menuStatus = popupMenu.AddDisabledItem("Status: Starting...")

	locationText := "📍 Roaming"
	if currentSSID == settings.HomeSSID && settings.HomeSSID != "" {
		locationText = "🏠 At Home"
	}
	menuLocation = popupMenu.AddDisabledItem(locationText)

	menuWiFi = popupMenu.AddDisabledItem(fmt.Sprintf("📶 WiFi: %s", safeSSID))

	phoneDisplay := "Not Set"
	if settings.PhoneMAC != "" {
		phoneDisplay = config.SanitizeDisplayString(settings.PhoneMAC)
	}
	menuPhoneMAC = popupMenu.AddDisabledItem(fmt.Sprintf("📱 Phone: %s", phoneDisplay))

	menuVersion = popupMenu.AddDisabledItem(fmt.Sprintf("ℹ️ Version: %s", Version))

	popupMenu.AddSeparator()

	// Actions
	popupMenu.AddItem("🏠 Set Current WiFi as Home", func() {
		ssid := network.GetCurrentSSID()
		if err := config.Update(ssid, ""); err != nil {
			logger.Error("Failed to set home SSID: %v", err)
		} else {
			safeSSID := config.SanitizeDisplayString(ssid)
			logger.Info("Home SSID set to: %s", safeSSID)
		}
		updateCustomMenuDisplay()
	})

	popupMenu.AddItem("📱 Select Monitored Device", func() {
		logger.Info("Select Device clicked - scanning...")
		devices := network.ScanNetworkDevices()
		if len(devices) > 0 {
			// Devices are already sanitized by ScanNetworkDevices
			config.Update("", devices[0].MAC)
			safeMAC := config.SanitizeDisplayString(devices[0].MAC)
			logger.Info("Auto-selected first device: %s", safeMAC)
		}
		updateCustomMenuDisplay()
	})

	popupMenu.AddSeparator()

	pauseText := "⏸️ Pause Protection"
	if settings.IsPaused {
		pauseText = "▶️ Resume Protection"
	}
	menuPause = popupMenu.AddItem(pauseText, func() {
		settings, _ := config.Load()
		if settings.IsPaused {
			config.SetPaused(false)
			menuPause.SetText("⏸️ Pause Protection")
			logger.Info("Protection resumed")
		} else {
			config.SetPaused(true)
			menuPause.SetText("▶️ Resume Protection")
			logger.Info("Protection paused")
		}
	})

	menuShutdownTimer = popupMenu.AddItem(fmt.Sprintf("⏱ Shutdown Timer (%ds)", settings.ShutdownDelay), func() {
		// Cycle through options: 10 -> 30 -> 60 -> 300 -> 10
		settings, _ := config.Load()
		var newDelay int
		switch settings.ShutdownDelay {
		case 10:
			newDelay = 30
		case 30:
			newDelay = 60
		case 60:
			newDelay = 300
		default:
			newDelay = 10
		}
		config.SetShutdownDelay(newDelay)
		menuShutdownTimer.SetText(fmt.Sprintf("⏱ Shutdown Timer (%ds)", newDelay))
		logger.Info("Shutdown timer set to %ds", newDelay)
	})

	popupMenu.AddSeparator()

	popupMenu.AddItem("❌ Quit", func() {
		logger.Info("User requested quit from custom menu")
		popupMenu.Hide()
		fyneApp.Quit()
	})

	popupMenu.Build()
}

// updateCustomMenuDisplay updates the dynamic menu items
func updateCustomMenuDisplay() {
	settings, _ := config.Load()
	currentSSID := network.GetCurrentSSID()
	safeSSID := config.SanitizeDisplayString(currentSSID)

	if menuLocation != nil {
		if currentSSID == settings.HomeSSID && settings.HomeSSID != "" {
			menuLocation.SetText("🏠 At Home")
		} else {
			menuLocation.SetText("📍 Roaming")
		}
	}

	if menuWiFi != nil {
		menuWiFi.SetText(fmt.Sprintf("📶 WiFi: %s", safeSSID))
	}

	if menuPhoneMAC != nil {
		if settings.PhoneMAC != "" {
			safeMAC := config.SanitizeDisplayString(settings.PhoneMAC)
			menuPhoneMAC.SetText(fmt.Sprintf("📱 Phone: %s", safeMAC))
		} else {
			menuPhoneMAC.SetText("📱 Phone: Not Set")
		}
	}

	if menuShutdownTimer != nil {
		menuShutdownTimer.SetText(fmt.Sprintf("⏱ Shutdown Timer (%ds)", settings.ShutdownDelay))
	}
}

// showCustomMenu toggles the custom popup menu
func showCustomMenu() {
	if popupMenu != nil {
		popupMenu.Toggle()
	}
}

// runFyneApp starts the Fyne event loop (call in goroutine)
func runFyneApp() {
	fyneApp.Run()
}
