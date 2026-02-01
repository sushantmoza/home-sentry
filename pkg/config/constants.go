package config

import "time"

// Default configuration constants
const (
	DefaultGraceChecks       = 5
	DefaultPollInterval      = 10
	DefaultPingTimeoutMs     = 500
	DefaultShutdownDelay     = 10
	DefaultShutdownAction    = ShutdownActionShutdown
	DefaultNtfyServer        = "https://ntfy.sh"
	DefaultDetectionType     = DetectionTypeMAC
	DefaultRetryAttempts     = 3
	DefaultRetryDelay        = 500 * time.Millisecond
	DefaultRetryMultiplier   = 1.5
	DefaultBeepInterval      = 2 * time.Second
	DefaultSoundFrequency    = 1000
	DefaultSoundDuration     = 300 * time.Millisecond
	MaxLogAge                = 7 * 24 * time.Hour
	MaxLogFileSize           = 10 * 1024 * 1024 // 10MB
	NotificationTimeout      = 10 * time.Second
	ShutdownMaxDelay         = 300 // 5 minutes
	ShutdownMinDelay         = 5   // 5 seconds
	MinPollInterval          = 1
	MaxPollInterval          = 300
	PingSweepStart           = 1
	PingSweepEnd             = 255
	GraceCountResetOnCancel  = true
	AutoScanDelay            = 1 * time.Second
	DisplayUpdateInterval    = 5 * time.Second
	DefaultConfirmationDelay = 10
	MinPINLength             = 4
	MaxPINLength             = 8
)

// Shutdown actions
const (
	ShutdownActionShutdown  = "shutdown"
	ShutdownActionHibernate = "hibernate"
	ShutdownActionLock      = "lock"
	ShutdownActionSleep     = "sleep"
)

// Validation limits
const (
	MaxGraceChecks = 100
	MinGraceChecks = 1
	MaxSSIDLength  = 32
	MACLength      = 17
)
