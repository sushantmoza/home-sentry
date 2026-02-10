package config

// Default configuration constants
const (
	DefaultGraceChecks    = 5
	DefaultPollInterval   = 10
	DefaultPingTimeoutMs  = 500
	DefaultShutdownDelay  = 10
	DefaultShutdownAction = ShutdownActionShutdown
	DefaultDetectionType  = DetectionTypeMAC
	DefaultRetryAttempts  = 3
	ShutdownMaxDelay      = 300 // 5 minutes
	ShutdownMinDelay      = 5   // 5 seconds
	MinPollInterval       = 1
	MaxPollInterval       = 300
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
	MinPINLength   = 4
	MaxPINLength   = 8
)
