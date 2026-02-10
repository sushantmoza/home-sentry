package network

import (
	"fmt"
	"home-sentry/pkg/config"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type NetworkDevice struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	MAC      string `json:"mac"`
	Vendor   string `json:"vendor"`
}

func GetCurrentSSID() string {
	if runtime.GOOS == "windows" {
		ssid, err := RetryWithResult(DefaultRetryConfig(), func() (string, error) {
			ssid := getWindowsSSID()
			if ssid == "Disconnected" || ssid == "Unknown" {
				return ssid, fmt.Errorf("wifi not connected")
			}
			return ssid, nil
		})
		if err != nil {
			return "Unknown"
		}
		return ssid
	}
	return "Simulated WiFi"
}

func ScanWifiNetworks() []string {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("netsh", "wlan", "show", "networks")
		HideConsole(cmd)
		output, err := cmd.Output()
		if err != nil {
			return []string{}
		}

		var ssids []string
		re := regexp.MustCompile(`SSID \d+ : (.+)`)
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				ssid := strings.TrimSpace(matches[1])
				if ssid != "" {
					ssids = append(ssids, ssid)
				}
			}
		}
		return ssids
	}
	return []string{"Simulated Network 1", "Simulated Network 2"}
}

func getWindowsSSID() string {
	cmd := exec.Command("netsh", "wlan", "show", "interfaces")
	HideConsole(cmd)
	output, err := cmd.Output()
	if err != nil {
		return "Unknown"
	}

	re := regexp.MustCompile(`\s+SSID\s+:\s+(.+)`)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return "Disconnected"
}

func ScanNetworkDevices() []NetworkDevice {
	if runtime.GOOS == "windows" {
		// 1. Determine local subnet
		ip, subnet, err := getLocalIP()
		if err == nil {
			// 2. Ping sweep to populate ARP table
			pingSweep(ip, subnet)
		}
		// 3. Read ARP table
		return scanARPWindows()
	}
	return []NetworkDevice{
		{IP: "192.168.1.100", Hostname: "Simulated-iPhone", MAC: "00:11:22:33:44:55"},
	}
}

func getLocalIP() (string, string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", "", err
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	// Get mask - simple assumption /24 for now as fetching valid mask is complex cross-platform without cgo
	// In a real robust app we'd iterate interfaces.
	return localAddr.IP.String(), "255.255.255.0", nil
}

func pingSweep(myIP string, mask string) {
	// Simple assumption: /24 network
	parts := strings.Split(myIP, ".")
	if len(parts) != 4 {
		return
	}
	baseIP := fmt.Sprintf("%s.%s.%s.", parts[0], parts[1], parts[2])

	var wg sync.WaitGroup
	// Ping 1-254
	for i := 1; i < 255; i++ {
		targetIP := baseIP + strconv.Itoa(i)
		// Don't ping self
		if targetIP == myIP {
			continue
		}

		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			// Fast timeout ping
			PingHost(ip)
		}(targetIP)
	}
	wg.Wait()
}

func scanARPWindows() []NetworkDevice {
	cmd := exec.Command("arp", "-a")
	HideConsole(cmd)
	output, err := cmd.Output()
	if err != nil {
		return []NetworkDevice{}
	}

	var devices []NetworkDevice
	lines := strings.Split(string(output), "\n")
	re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s+([0-9a-fA-F-]{17})`)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 2 {
			ip := matches[1]
			mac := matches[2]
			if strings.HasPrefix(ip, "224.") || strings.HasPrefix(ip, "239.") || mac == "ff-ff-ff-ff-ff-ff" {
				continue
			}

			wg.Add(1)
			go func(ip, mac string) {
				defer wg.Done()

				// Validate IP from ARP table
				sanitizedIP, err := config.SanitizeIP(ip)
				if err != nil || sanitizedIP == "" {
					return // Skip invalid IPs from ARP
				}

				// Validate MAC from ARP table
				sanitizedMAC, err := config.SanitizeMAC(mac)
				if err != nil || sanitizedMAC == "" {
					return // Skip invalid MACs from ARP
				}

				hostname := "Unknown"
				names, lookupErr := net.LookupAddr(sanitizedIP)
				if lookupErr == nil && len(names) > 0 {
					raw := strings.TrimSuffix(names[0], ".")
					// Sanitize hostname from DNS to prevent injection
					sanitizedHost, err := config.SanitizeHostname(raw)
					if err == nil && sanitizedHost != "" {
						hostname = sanitizedHost
					}
				}

				mu.Lock()
				devices = append(devices, NetworkDevice{
					IP:       sanitizedIP,
					Hostname: hostname,
					MAC:      sanitizedMAC,
					Vendor:   GetVendor(sanitizedMAC),
				})
				mu.Unlock()
			}(ip, mac)
		}
	}
	wg.Wait()
	return devices
}

func PingHost(ip string) bool {
	return PingHostWithTimeout(ip, 500)
}

func PingHostWithTimeout(ip string, timeoutMs int) bool {
	if runtime.GOOS == "windows" {
		// Validate IP address to prevent command injection
		if net.ParseIP(ip) == nil {
			return false
		}
		cmd := exec.Command("ping", "-n", "1", "-w", strconv.Itoa(timeoutMs), ip)
		HideConsole(cmd)
		err := cmd.Run()
		return err == nil
	}
	return true
}

// IsDeviceOnNetwork checks if a device with the given MAC address is on the network
// by actively verifying its presence (not trusting stale ARP cache).
func IsDeviceOnNetwork(mac string) bool {
	if runtime.GOOS != "windows" {
		return true // Simulated on non-Windows
	}

	// Normalize MAC to lowercase with dashes
	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "-")

	// First find the IP associated with this MAC (if any)
	lastKnownIP := FindIPByMAC(mac)

	// Delete stale ARP entry to force fresh lookup
	if lastKnownIP != "" {
		deleteARPEntry(lastKnownIP)
	}

	// If we had an IP, ping it directly to refresh ARP
	if lastKnownIP != "" {
		// Ping the specific IP with short timeout
		PingHostWithTimeout(lastKnownIP, 500)
	} else {
		// No cached IP - do a quick ping sweep to find the device
		ip, subnet, err := getLocalIP()
		if err == nil {
			pingSweep(ip, subnet)
		}
	}

	// Now check if MAC appeared in fresh ARP table
	return checkARPForMAC(mac)
}

// deleteARPEntry removes a specific IP from the ARP cache to force fresh lookup
func deleteARPEntry(ip string) {
	// Validate IP address to prevent command injection
	if net.ParseIP(ip) == nil {
		return
	}
	cmd := exec.Command("arp", "-d", ip)
	HideConsole(cmd)
	cmd.Run() // Ignore errors - may fail if not admin, that's OK
}

// checkARPForMAC checks if the MAC address exists in the current ARP table
func checkARPForMAC(mac string) bool {
	cmd := exec.Command("arp", "-a")
	HideConsole(cmd)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Normalize and search
	outputLower := strings.ToLower(string(output))
	return strings.Contains(outputLower, mac)
}

// FindIPByMAC returns the IP address for a given MAC address from the ARP table
func FindIPByMAC(mac string) string {
	if runtime.GOOS != "windows" {
		return ""
	}

	mac = strings.ToLower(mac)
	mac = strings.ReplaceAll(mac, ":", "-")

	cmd := exec.Command("arp", "-a")
	HideConsole(cmd)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	lines := strings.Split(string(output), "\n")
	re := regexp.MustCompile(`(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\s+([0-9a-fA-F-]{17})`)

	for _, line := range lines {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 2 {
			foundMAC := strings.ToLower(matches[2])
			if foundMAC == mac {
				// Validate the IP before returning
				foundIP := matches[1]
				if net.ParseIP(foundIP) != nil {
					return foundIP
				}
				return ""
			}
		}
	}

	return ""
}
