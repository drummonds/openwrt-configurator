package export

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/device"
	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
)

// ExportConfig reads configuration from an OpenWRT device and exports it as JSON
// If modelID is empty, it will be auto-detected from the device's board.json
func ExportConfig(modelID, ipAddr, username, password string) (*config.ONCConfig, error) {
	// Connect to device
	client, err := ssh.Connect(ipAddr, username, password)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %w", err)
	}
	defer client.Close()

	return ExportConfigFromClient(client, modelID, ipAddr, username, password)
}

// ExportConfigFromClient reads configuration from an OpenWRT device using an existing SSH client
// If modelID is empty, it will be auto-detected from the device's board.json
func ExportConfigFromClient(client ssh.SSHExecutor, modelID, ipAddr, username, password string) (*config.ONCConfig, error) {
	// Get board.json to detect/verify device model
	boardOutput, err := client.Execute("cat /etc/board.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read board.json: %w", err)
	}

	var boardJSON device.BoardJSON
	if err := json.Unmarshal([]byte(boardOutput), &boardJSON); err != nil {
		return nil, fmt.Errorf("failed to parse board.json: %w", err)
	}

	// Auto-detect model ID if not provided
	if modelID == "" {
		modelID = boardJSON.Model.ID
	}

	// Read system configuration
	systemConfig, err := readSystemConfig(client)
	if err != nil {
		return nil, fmt.Errorf("failed to read system config: %w", err)
	}

	// Read network configuration
	networkConfig, err := readNetworkConfig(client)
	if err != nil {
		return nil, fmt.Errorf("failed to read network config: %w", err)
	}

	// Read wireless configuration
	wirelessConfig, err := readWirelessConfig(client)
	if err != nil {
		// Wireless may not exist on all devices
		wirelessConfig = nil
	}

	// Read dropbear configuration
	dropbearConfig, err := readDropbearConfig(client)
	if err != nil {
		// Non-fatal, may not exist
		dropbearConfig = nil
	}

	// Read installed packages
	packages, err := readInstalledPackages(client)
	if err != nil {
		return nil, fmt.Errorf("failed to read installed packages: %w", err)
	}

	// Build ONCConfig
	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  boardJSON.Model.ID,
				IPAddr:   ipAddr,
				Hostname: systemConfig.Hostname,
				Tags:     make(map[string]any),
				ProvisioningConfig: &config.ProvisioningConfig{
					SSHAuth: config.SSHAuth{
						Username: username,
						Password: password,
					},
				},
			},
		},
		PackageProfiles: []config.PackageProfile{
			{
				Packages: packages,
			},
		},
		Config: config.ConfigConfig{
			System:   systemConfig.Config,
			Network:  networkConfig,
			Wireless: wirelessConfig,
			Dropbear: dropbearConfig,
		},
	}

	return oncConfig, nil
}

// SystemInfo holds basic system information
type SystemInfo struct {
	Hostname string
	Config   *config.SystemConfig
}

func readSystemConfig(client ssh.SSHExecutor) (*SystemInfo, error) {
	output, err := client.Execute("uci show system")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	sections := make(map[string]map[string]string)
	var hostname string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse: system.@system[0].hostname='my-router'
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "'\"")

		// Extract section and field
		keyParts := strings.Split(key, ".")
		if len(keyParts) < 3 {
			continue
		}

		section := keyParts[1]
		field := keyParts[2]

		if sections[section] == nil {
			sections[section] = make(map[string]string)
		}
		sections[section][field] = value

		if field == "hostname" {
			hostname = value
		}
	}

	// Build SystemConfig
	var systemSections []config.SystemSection
	for sectionName, fields := range sections {
		if !strings.Contains(sectionName, "system") {
			continue
		}

		section := config.SystemSection{
			Name: strPtr(sectionName),
		}

		if h, ok := fields["hostname"]; ok {
			section.Hostname = strPtr(h)
		}
		if tz, ok := fields["timezone"]; ok {
			section.Timezone = strPtr(tz)
		}
		if zn, ok := fields["zonename"]; ok {
			section.Zonename = strPtr(zn)
		}

		systemSections = append(systemSections, section)
	}

	systemConfig := &config.SystemConfig{
		System: systemSections,
	}

	return &SystemInfo{
		Hostname: hostname,
		Config:   systemConfig,
	}, nil
}

func readNetworkConfig(client ssh.SSHExecutor) (*config.NetworkConfig, error) {
	output, err := client.Execute("uci show network")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	interfaces := make(map[string]map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse network config lines
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "'\"")

		keyParts := strings.Split(key, ".")
		if len(keyParts) < 3 {
			continue
		}

		section := keyParts[1]
		field := keyParts[2]

		// Skip type definitions (e.g., network.lan=interface)
		if field == "interface" || field == "device" && value == "device" {
			continue
		}

		if interfaces[section] == nil {
			interfaces[section] = make(map[string]string)
		}
		interfaces[section][field] = value
	}

	// Build NetworkConfig
	var interfaceSections []config.InterfaceSection
	for sectionName, fields := range interfaces {
		// Only include sections that have actual interface properties
		if len(fields) == 0 {
			continue
		}

		section := config.InterfaceSection{
			Name: strPtr(sectionName),
		}

		if proto, ok := fields["proto"]; ok {
			section.Proto = strPtr(proto)
		}
		if device, ok := fields["device"]; ok {
			section.Device = strPtr(device)
		}
		if ipaddr, ok := fields["ipaddr"]; ok {
			section.IPAddr = strPtr(ipaddr)
		}
		if netmask, ok := fields["netmask"]; ok {
			section.Netmask = strPtr(netmask)
		}
		if gateway, ok := fields["gateway"]; ok {
			section.Gateway = strPtr(gateway)
		}

		interfaceSections = append(interfaceSections, section)
	}

	return &config.NetworkConfig{
		Interface: interfaceSections,
	}, nil
}

func readWirelessConfig(client ssh.SSHExecutor) (*config.WirelessConfig, error) {
	output, err := client.Execute("uci show wireless")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(output) == "" {
		return nil, fmt.Errorf("no wireless configuration")
	}

	lines := strings.Split(output, "\n")
	devices := make(map[string]map[string]string)
	ifaces := make(map[string]map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "'\"")

		keyParts := strings.Split(key, ".")
		if len(keyParts) < 3 {
			continue
		}

		section := keyParts[1]
		field := keyParts[2]

		if strings.Contains(section, "radio") && field != "wifi-device" {
			if devices[section] == nil {
				devices[section] = make(map[string]string)
			}
			devices[section][field] = value
		} else if (strings.Contains(section, "default_") || strings.Contains(section, "iface")) && field != "wifi-iface" {
			if ifaces[section] == nil {
				ifaces[section] = make(map[string]string)
			}
			ifaces[section][field] = value
		}
	}

	// Build WirelessConfig
	var deviceSections []config.WifiDeviceSection
	for sectionName, fields := range devices {
		section := config.WifiDeviceSection{
			Name: strPtr(sectionName),
		}

		if t, ok := fields["type"]; ok {
			section.Type = strPtr(t)
		}
		if band, ok := fields["band"]; ok {
			section.Band = strPtr(band)
		}
		if channel, ok := fields["channel"]; ok {
			section.Channel = strPtr(channel)
		}

		deviceSections = append(deviceSections, section)
	}

	var ifaceSections []config.WifiIfaceSection
	for sectionName, fields := range ifaces {
		section := config.WifiIfaceSection{
			Name: strPtr(sectionName),
		}

		if device, ok := fields["device"]; ok {
			section.Device = device
		}
		if mode, ok := fields["mode"]; ok {
			section.Mode = strPtr(mode)
		}
		if ssid, ok := fields["ssid"]; ok {
			section.SSID = strPtr(ssid)
		}
		if encryption, ok := fields["encryption"]; ok {
			section.Encryption = strPtr(encryption)
		}
		if network, ok := fields["network"]; ok {
			section.Network = strPtr(network)
		}

		ifaceSections = append(ifaceSections, section)
	}

	if len(deviceSections) == 0 && len(ifaceSections) == 0 {
		return nil, fmt.Errorf("no wireless configuration found")
	}

	return &config.WirelessConfig{
		WifiDevice: deviceSections,
		WifiIface:  ifaceSections,
	}, nil
}

func readDropbearConfig(client ssh.SSHExecutor) (*config.DropbearConfig, error) {
	output, err := client.Execute("uci show dropbear")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	sections := make(map[string]map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := strings.Trim(parts[1], "'\"")

		keyParts := strings.Split(key, ".")
		if len(keyParts) < 3 {
			continue
		}

		section := keyParts[1]
		field := keyParts[2]

		// Skip type definitions
		if field == "dropbear" {
			continue
		}

		if sections[section] == nil {
			sections[section] = make(map[string]string)
		}
		sections[section][field] = value
	}

	var dropbearSections []config.DropbearSection
	for sectionName, fields := range sections {
		if len(fields) == 0 {
			continue
		}

		section := config.DropbearSection{
			Name: strPtr(sectionName),
		}

		if pa, ok := fields["PasswordAuth"]; ok {
			section.PasswordAuth = strPtr(pa)
		}
		if rpa, ok := fields["RootPasswordAuth"]; ok {
			section.RootPasswordAuth = strPtr(rpa)
		}
		if port, ok := fields["Port"]; ok {
			if p := parseInt(port); p != nil {
				section.Port = p
			}
		}

		dropbearSections = append(dropbearSections, section)
	}

	if len(dropbearSections) == 0 {
		return nil, fmt.Errorf("no dropbear configuration found")
	}

	return &config.DropbearConfig{
		Dropbear: dropbearSections,
	}, nil
}

func readInstalledPackages(client ssh.SSHExecutor) ([]string, error) {
	output, err := client.Execute("opkg list-installed")
	if err != nil {
		return nil, err
	}

	var packages []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "package-name - version"
		parts := strings.Split(line, " - ")
		if len(parts) > 0 {
			packages = append(packages, parts[0])
		}
	}

	return packages, nil
}

func strPtr(s string) *string {
	return &s
}

func parseInt(s string) *int {
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err == nil {
		return &i
	}
	return nil
}
