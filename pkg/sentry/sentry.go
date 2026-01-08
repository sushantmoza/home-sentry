package sentry

import (
	"fmt"
	"home-sentry/pkg/config"
	"home-sentry/pkg/network"
	"log"
	"os/exec"
	"runtime"
	"sync"
	"time"
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
}

func NewSentryManager() *SentryManager {
	return &SentryManager{
		status:          StatusRoaming,
		graceCount:      0,
		phoneEverSeen:   false,
		cancelShutdown:  make(chan struct{}),
		shutdownPending: false,
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
		log.Println("Shutdown cancelled by user")
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
	log.Println("Starting Sentry Monitor...")
	for {
		settings, err := config.Load()
		if err != nil {
			log.Printf("Error loading settings: %v. Retrying in %ds...", err, settings.PollInterval)
			time.Sleep(time.Duration(settings.PollInterval) * time.Second)
			continue
		}

		ssid := network.GetCurrentSSID()

		if settings.IsPaused {
			log.Println("Status: PAUSED. Protection disabled.")
			s.setStatus(StatusPaused)
			time.Sleep(time.Duration(settings.PollInterval) * time.Second)
			continue
		}

		log.Printf("Monitor Check: Current SSID=%s, Home SSID=%s, IP=%s", ssid, settings.HomeSSID, settings.PhoneIP)

		if ssid == settings.HomeSSID {
			// At home, check for phone
			if settings.PhoneIP != "" && settings.PhoneIP != "0.0.0.0" {
				alive := network.PingHostWithTimeout(settings.PhoneIP, settings.PingTimeoutMs)
				if alive {
					log.Printf("Phone (%s) detected. Safe.", settings.PhoneIP)
					s.setStatus(StatusMonitoring)
					s.graceCount = 0
					s.phoneEverSeen = true
				} else {
					log.Printf("WARNING: Phone (%s) NOT detected on home wifi!", settings.PhoneIP)

					// Only enter grace period if we've seen the phone before
					if s.phoneEverSeen {
						s.graceCount++
						s.setStatus(StatusGracePeriod)
						log.Printf("Status: GRACE PERIOD (%d/%d)", s.graceCount, settings.GraceChecks)

						if s.graceCount >= settings.GraceChecks {
							s.setStatus(StatusShutdownImminent)
							log.Println("CRITICAL: Grace period expired. SHUTDOWN IMMINENT!")
							s.triggerShutdownWithCountdown(settings)
						}
					} else {
						// Phone never seen yet, waiting for initial connection
						log.Println("Waiting for phone to be detected for the first time...")
						s.setStatus(StatusWaitingForPhone)
					}
				}
			} else {
				log.Println("No phone IP configured. Monitoring disabled.")
				s.setStatus(StatusRoaming)
			}
		} else {
			s.setStatus(StatusRoaming)
			s.graceCount = 0
			log.Println("Status: Roaming (Not on Home WiFi).")
		}

		time.Sleep(time.Duration(settings.PollInterval) * time.Second)
	}
}

func (s *SentryManager) triggerShutdownWithCountdown(settings config.Settings) {
	s.mu.Lock()
	s.shutdownPending = true
	s.mu.Unlock()

	// Show notification
	s.showNotification("Home Sentry Alert", "Phone not detected! Shutting down in 10 seconds...")

	// 10 second countdown with cancel option
	log.Println("Starting 10 second shutdown countdown...")
	select {
	case <-time.After(10 * time.Second):
		// Countdown completed, proceed with shutdown
		s.mu.Lock()
		s.shutdownPending = false
		s.mu.Unlock()
		s.executeShutdown()
	case <-s.cancelShutdown:
		// Shutdown was cancelled
		log.Println("Shutdown countdown cancelled")
		s.setStatus(StatusMonitoring)
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

func (s *SentryManager) executeShutdown() {
	if runtime.GOOS == "windows" {
		log.Println("Executing shutdown command...")
		cmd := exec.Command("shutdown", "/s", "/t", "0")
		network.HideConsole(cmd)
		err := cmd.Run()
		if err != nil {
			log.Printf("Failed to execute shutdown: %v", err)
		}
	} else {
		log.Println("Shutdown simulation (Non-Windows OS)")
	}
}
