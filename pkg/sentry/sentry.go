package sentry

import (
	"home-sentry/pkg/config"
	"home-sentry/pkg/network"
	"log"
	"os/exec"
	"runtime"
	"time"
)

type SentryStatus string

const (
	StatusRoaming          SentryStatus = "Roaming"
	StatusMonitoring       SentryStatus = "Monitoring"
	StatusGracePeriod      SentryStatus = "GracePeriod"
	StatusShutdownImminent SentryStatus = "ShutdownImminent"
	StatusPaused           SentryStatus = "Paused"
)

type SentryManager struct {
	status         SentryStatus
	graceCount     int
	StatusCallback func(SentryStatus)
}

func NewSentryManager() *SentryManager {
	return &SentryManager{
		status:     StatusRoaming,
		graceCount: 0,
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

func (s *SentryManager) StartMonitor() {
	log.Println("Starting Sentry Monitor...")
	for {
		// 1. Get current state
		ssid := network.GetCurrentSSID()
		settings, err := config.Load()
		if err != nil {
			log.Printf("Error loading settings: %v. Retrying in 10s...", err)
			time.Sleep(10 * time.Second)
			continue
		}

		if settings.IsPaused {
			log.Println("Status: PAUSED. Protection disabled.")
			s.setStatus(StatusPaused)
			time.Sleep(10 * time.Second)
			continue
		}

		// 2. Logic
		log.Printf("Monitor Check: Current SSID=%s, Home SSID=%s, IP=%s", ssid, settings.HomeSSID, settings.PhoneIP)

		if ssid == settings.HomeSSID {
			// At home, check for phone
			if settings.PhoneIP != "" && settings.PhoneIP != "0.0.0.0" {
				alive := network.PingHost(settings.PhoneIP)
				if alive {
					log.Printf("Phone (%s) detected. Safe.", settings.PhoneIP)
					s.setStatus(StatusMonitoring)
					s.graceCount = 0
				} else {
					log.Printf("WARNING: Phone (%s) NOT detected on home wifi!", settings.PhoneIP)

					// Only enter grace period if we were previously monitoring (phone was detected before)
					if s.status == StatusMonitoring {
						s.setStatus(StatusGracePeriod)
						s.graceCount++
						log.Printf("Status: GRACE PERIOD (%d/5)", s.graceCount)

						if s.graceCount >= 5 {
							s.setStatus(StatusShutdownImminent)
							log.Println("CRITICAL: Grace period expired. SHUTDOWN IMMINENT!")

							// Execute Shutdown
							s.triggerShutdown()
						}
					} else {
						// Not yet monitoring, just stay in roaming until phone is detected
						log.Println("Waiting for phone to be detected...")
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

		time.Sleep(10 * time.Second)
	}
}

func (s *SentryManager) triggerShutdown() {
	if runtime.GOOS == "windows" {
		log.Println("Executing shutdown command...")
		cmd := exec.Command("shutdown", "/s", "/t", "0")
		err := cmd.Run()
		if err != nil {
			log.Printf("Failed to execute shutdown: %v", err)
		}
	} else {
		log.Println("Shutdown simulation (Non-Windows OS)")
	}
}
