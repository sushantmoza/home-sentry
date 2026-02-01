package sentry

import (
	"encoding/json"
	"fmt"
	"home-sentry/pkg/config"
	"home-sentry/pkg/network"
	"home-sentry/pkg/ntfy"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"home-sentry/pkg/logger"
)

type SentryStatus string

const (
	StatusRoaming          SentryStatus = "Roaming"
	StatusMonitoring       SentryStatus = "Monitoring"
	StatusGracePeriod      SentryStatus = "GracePeriod"
	StatusShutdownImminent SentryStatus = "ShutdownImminent"
	StatusPaused           SentryStatus = "Paused"
	StatusWaitingForPhone  SentryStatus = "WaitingForPhone"
)

type SentryManager struct {
	status          SentryStatus
	graceCount      int
	phoneEverSeen   bool
	StatusCallback  func(SentryStatus)
	cancelShutdown  chan struct{}
	shutdownPending bool
	mu              sync.Mutex
	stateFile       string
}

type SentryState struct {
	PhoneEverSeen bool `json:"phone_ever_seen"`
}

func NewSentryManager() *SentryManager {
	statePath := getStateFilePath()
	sm := &SentryManager{
		status:          StatusRoaming,
		graceCount:      0,
		phoneEverSeen:   false,
		cancelShutdown:  make(chan struct{}),
		shutdownPending: false,
		stateFile:       statePath,
	}
	// Load persisted state
	sm.loadState()
	return sm
}

func getStateFilePath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return "sentry-state.json"
	}
	dir := filepath.Join(appData, "HomeSentry")
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "sentry-state.json")
}

func (s *SentryManager) loadState() {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Info("Failed to load state file: %v", err)
		}
		return
	}
	var state SentryState
	if err := json.Unmarshal(data, &state); err != nil {
		logger.Info("Failed to parse state file: %v", err)
		return
	}
	s.phoneEverSeen = state.PhoneEverSeen
	logger.Info("Loaded state: phoneEverSeen=%v", s.phoneEverSeen)
}

func (s *SentryManager) saveState() {
	state := SentryState{PhoneEverSeen: s.phoneEverSeen}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		logger.Info("Failed to marshal state: %v", err)
		return
	}
	if err := os.WriteFile(s.stateFile, data, 0644); err != nil {
		logger.Info("Failed to save state file: %v", err)
	}
}

func (s *SentryManager) SetStatusCallback(cb func(SentryStatus)) {
	s.StatusCallback = cb
}

func (s *SentryManager) setStatus(status SentryStatus) {
	s.status = status
	if s.StatusCallback != nil {
		s.StatusCallback(status)
	}
}

// CancelShutdown cancels a pending shutdown if one is in progress
func (s *SentryManager) CancelShutdown() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.shutdownPending {
		close(s.cancelShutdown)
		s.cancelShutdown = make(chan struct{}) // Reset for future use
		s.shutdownPending = false
		s.graceCount = 0
		logger.Info("Shutdown cancelled by user")
		return true
	}
	return false
}

// IsShutdownPending returns true if a shutdown countdown is in progress
func (s *SentryManager) IsShutdownPending() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shutdownPending
}

func (s *SentryManager) StartMonitor() {
	logger.Info("Starting Sentry Monitor...")
	for {
		settings, err := config.Load()
		if err != nil {
			logger.Info("Error loading settings: %v. Retrying in %ds...", err, settings.PollInterval)
			time.Sleep(time.Duration(settings.PollInterval) * time.Second)
			continue
		}

		ssid := network.GetCurrentSSID()

		if settings.IsPaused {
			logger.Info("Status: PAUSED. Protection disabled.")
			s.setStatus(StatusPaused)
			time.Sleep(time.Duration(settings.PollInterval) * time.Second)
			continue
		}

		logger.Info("Monitor Check: Current SSID=%s, Home SSID=%s, MAC=%s", ssid, settings.HomeSSID, settings.PhoneMAC)

		if ssid == settings.HomeSSID {
			// At home, check for phone
			if settings.HasDeviceConfigured() {
				alive := network.IsDeviceOnNetwork(settings.PhoneMAC)
				if alive {
					logger.Info("Phone (MAC: %s) detected. Safe.", settings.PhoneMAC)
					s.setStatus(StatusMonitoring)
					s.graceCount = 0
					if !s.phoneEverSeen {
						s.phoneEverSeen = true
						s.saveState()
						logger.Info("Phone first seen - state persisted")
					}
				} else {
					logger.Info("WARNING: Phone (MAC: %s) NOT detected on home wifi!", settings.PhoneMAC)

					// Only enter grace period if we've seen the phone before
					if s.phoneEverSeen {
						s.graceCount++
						s.setStatus(StatusGracePeriod)
						logger.Info("Status: GRACE PERIOD (%d/%d)", s.graceCount, settings.GraceChecks)

						if s.graceCount >= settings.GraceChecks {
							s.setStatus(StatusShutdownImminent)
							logger.Info("CRITICAL: Grace period expired. SHUTDOWN IMMINENT!")
							s.triggerShutdownWithCountdown(settings)
						}
					} else {
						// Phone never seen yet, waiting for initial connection
						logger.Info("Waiting for phone to be detected for the first time...")
						s.setStatus(StatusWaitingForPhone)
					}
				}
			} else {
				logger.Info("No device configured. Monitoring disabled.")
				s.setStatus(StatusRoaming)
			}
		} else {
			s.setStatus(StatusRoaming)
			s.graceCount = 0
			logger.Info("Status: Roaming (Not on Home WiFi).")
		}

		time.Sleep(time.Duration(settings.PollInterval) * time.Second)
	}
}

func (s *SentryManager) triggerShutdownWithCountdown(settings config.Settings) {
	s.mu.Lock()
	s.shutdownPending = true
	s.mu.Unlock()

	// Show local notification
	s.showNotification("Home Sentry Alert", fmt.Sprintf("Phone not detected! Shutting down in %d seconds...", settings.ShutdownDelay))

	// Send ntfy notification if enabled
	var ntfyCmdCh <-chan ntfy.Command
	var ntfyClient *ntfy.Client
	if settings.NtfyEnabled && settings.NtfyTopic != "" {
		ntfyClient = ntfy.NewClient(settings.NtfyServer, settings.NtfyTopic)
		if err := ntfyClient.SendShutdownNotification(settings.ShutdownDelay); err != nil {
			logger.Info("Failed to send ntfy notification: %v", err)
		} else {
			// Start listening for cancel commands
			var err error
			ntfyCmdCh, err = ntfyClient.StartShutdownCancelListener()
			if err != nil {
				logger.Info("Failed to start ntfy cancel listener: %v", err)
			}
		}
	}

	// Play initial warning sound
	s.playWarningSound()

	// Shutdown countdown with cancel option and periodic beeps
	logger.Info("Starting %d second shutdown countdown...", settings.ShutdownDelay)

	// Timer for the total countdown
	shutdownTimer := time.NewTimer(time.Duration(settings.ShutdownDelay) * time.Second)
	defer shutdownTimer.Stop()

	// Ticker for periodic beeps
	beepTicker := time.NewTicker(2 * time.Second)
	defer beepTicker.Stop()

	// Cleanup ntfy listener when done
	defer func() {
		if ntfyClient != nil {
			ntfyClient.StopListener()
		}
	}()

	countdown := settings.ShutdownDelay - 2 // Already played first beep, next beep shows (delay-2) seconds
	for {
		select {
		case <-beepTicker.C:
			if countdown > 0 {
				s.playWarningSound()
				logger.Info("Shutdown in %d seconds...", countdown)
				countdown -= 2
			}
		case <-shutdownTimer.C:
			// Countdown completed, proceed with shutdown
			s.mu.Lock()
			s.shutdownPending = false
			s.mu.Unlock()

			// Check if PIN confirmation is required
			if settings.RequirePIN && settings.ShutdownPIN != "" {
				// Show PIN dialog - for now, we just delay and allow manual cancel
				// A real implementation would show a dialog
				logger.Info("PIN confirmation required - showing dialog...")
				s.showNotification("Home Sentry", "Enter PIN to proceed with shutdown")
				// In a real app, this would wait for PIN input
				// For now, we add an extra 10 second delay for manual intervention
				time.Sleep(10 * time.Second)
			}

			s.executeShutdown(settings)
			return
		case <-s.cancelShutdown:
			// Shutdown was cancelled locally
			logger.Info("Shutdown countdown cancelled (local)")
			s.setStatus(StatusMonitoring)
			return
		case cmd := <-ntfyCmdCh:
			// Shutdown was cancelled via ntfy
			s.mu.Lock()
			s.shutdownPending = false
			s.graceCount = 0
			s.mu.Unlock()

			if cmd == ntfy.CmdCancelAndPause {
				// Cancel and pause protection until resumed
				logger.Info("Shutdown cancelled via ntfy - PAUSING protection")
				config.SetPaused(true)
				s.setStatus(StatusPaused)
				// Send confirmation notification
				if ntfyClient != nil {
					go ntfyClient.SendPausedNotification()
				}
			} else {
				// Cancel only - keep monitoring
				logger.Info("Shutdown cancelled via ntfy - resuming monitoring")
				s.setStatus(StatusMonitoring)
			}
			return
		}
	}
}

// playWarningSound plays a system warning beep
func (s *SentryManager) playWarningSound() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command",
			"[console]::beep(1000, 300)")
		network.HideConsole(cmd)
		go cmd.Run()
	}
}

func (s *SentryManager) showNotification(title, message string) {
	if runtime.GOOS == "windows" {
		// Use PowerShell for toast notification
		script := fmt.Sprintf(`
			Add-Type -AssemblyName System.Windows.Forms
			$balloon = New-Object System.Windows.Forms.NotifyIcon
			$balloon.Icon = [System.Drawing.SystemIcons]::Warning
			$balloon.BalloonTipIcon = [System.Windows.Forms.ToolTipIcon]::Warning
			$balloon.BalloonTipTitle = '%s'
			$balloon.BalloonTipText = '%s'
			$balloon.Visible = $true
			$balloon.ShowBalloonTip(10000)
			Start-Sleep -Seconds 10
			$balloon.Dispose()
		`, title, message)
		cmd := exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", script)
		network.HideConsole(cmd)
		go cmd.Run() // Run async
	}
}

func (s *SentryManager) executeShutdown(settings config.Settings) {
	if runtime.GOOS != "windows" {
		logger.Info("Shutdown simulation (Non-Windows OS) - action: %s", settings.ShutdownAction)
		return
	}

	logger.Info("Executing %s command...", settings.ShutdownAction)

	var cmd *exec.Cmd
	switch settings.ShutdownAction {
	case config.ShutdownActionShutdown:
		cmd = exec.Command("shutdown", "/s", "/t", "0")
	case config.ShutdownActionHibernate:
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
	case config.ShutdownActionSleep:
		cmd = exec.Command("rundll32.exe", "powrprof.dll,SetSuspendState", "0,1,0")
	case config.ShutdownActionLock:
		cmd = exec.Command("rundll32.exe", "user32.dll,LockWorkStation")
	default:
		cmd = exec.Command("shutdown", "/s", "/t", "0")
	}

	network.HideConsole(cmd)
	err := cmd.Run()
	if err != nil {
		logger.Info("Failed to execute %s: %v", settings.ShutdownAction, err)
	}
}
