package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Settings struct {
	HomeSSID string `json:"home_ssid"`
	PhoneIP  string `json:"phone_ip"`
	IsPaused bool   `json:"is_paused"`
}

const settingsFile = "settings.json"

func Load() (Settings, error) {
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return Settings{}, err
	}
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return Settings{}, err
	}
	return settings, nil
}

func Save(settings Settings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsFile, data, 0644)
}

func Update(ssid, ip string) error {
	settings, _ := Load()
	if ssid != "" {
		settings.HomeSSID = ssid
	}
	if ip != "" {
		settings.PhoneIP = ip
	}

	fmt.Printf("Updating settings: %+v\n", settings)
	return Save(settings)
}

func SetPaused(paused bool) error {
	settings, _ := Load()
	settings.IsPaused = paused
	fmt.Printf("Updating paused status: %v\n", paused)
	return Save(settings)
}
