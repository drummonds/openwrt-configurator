package device

import (
	"encoding/json"
	"fmt"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
)

// DeviceSchema represents the schema for a device
type DeviceSchema struct {
	Name           string              `json:"name"`
	Version        string              `json:"version"`
	SwConfig       bool                `json:"sw_config"`
	ConfigSections map[string][]string `json:"config_sections,omitempty"`
	Ports          []Port              `json:"ports,omitempty"`
	Radios         []Radio             `json:"radios,omitempty"`
}

// Port represents a network port on the device
type Port struct {
	Name            string  `json:"name"`
	DefaultRole     *string `json:"default_role,omitempty"`
	SwConfigCPUName *string `json:"sw_config_cpu_name,omitempty"`
}

// Radio represents a WiFi radio on the device
type Radio struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Band string `json:"band"`
}

// BoardJSON represents the board.json structure
type BoardJSON struct {
	Model struct {
		ID string `json:"id"`
	} `json:"model"`
	Switch  map[string]SwitchInfo `json:"switch,omitempty"`
	Network NetworkInfo           `json:"network"`
}

// SwitchInfo represents switch configuration
type SwitchInfo struct {
	Enable bool       `json:"enable"`
	Reset  bool       `json:"reset"`
	Ports  []PortInfo `json:"ports"`
}

// PortInfo represents port information
type PortInfo struct {
	Num    int     `json:"num"`
	Role   *string `json:"role,omitempty"`
	Device *string `json:"device,omitempty"`
}

// NetworkInfo represents network information from board.json
type NetworkInfo struct {
	Lan NetworkInterface  `json:"lan"`
	Wan *NetworkInterface `json:"wan,omitempty"`
}

// NetworkInterface represents a network interface
type NetworkInterface struct {
	Ports    []string `json:"ports,omitempty"`
	Device   *string  `json:"device,omitempty"`
	Protocol string   `json:"protocol"`
}

// WirelessConfigResponse represents the ubus response for wireless config
type WirelessConfigResponse struct {
	Values map[string]WifiDeviceInfo `json:"values"`
}

// WifiDeviceInfo represents information about a WiFi device
type WifiDeviceInfo struct {
	Type    string  `json:"type"`
	Name    string  `json:".name"`
	Path    string  `json:"path"`
	Channel string  `json:"channel"`
	Band    string  `json:"band"`
	Htmode  *string `json:"htmode,omitempty"`
}

// GetDeviceSchema retrieves the schema for a device
func GetDeviceSchema(deviceConfig *config.DeviceConfig) (*DeviceSchema, error) {
	if deviceConfig.ProvisioningConfig == nil {
		return nil, fmt.Errorf("provisioning config not set for device %s", deviceConfig.ModelID)
	}

	// Connect via SSH
	client, err := ssh.Connect(
		deviceConfig.IPAddr,
		deviceConfig.ProvisioningConfig.SSHAuth.Username,
		deviceConfig.ProvisioningConfig.SSHAuth.Password,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %w", err)
	}
	defer client.Close()

	// Get board.json
	boardJSON, err := getBoardJSON(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get board.json: %w", err)
	}

	// Get radios
	radios, err := getRadios(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get radios: %w", err)
	}

	// Get config sections
	configSections, err := getConfigSections(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get config sections: %w", err)
	}

	// Get version
	version, err := getDeviceVersion(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get device version: %w", err)
	}

	// Determine if this is a swconfig device
	isSwConfig := len(boardJSON.Switch) > 0

	// Build ports list
	var ports []Port
	if isSwConfig {
		// For swconfig devices, use switch port info
		for _, switchInfo := range boardJSON.Switch {
			for _, port := range switchInfo.Ports {
				p := Port{
					Name: fmt.Sprintf("eth%d", port.Num),
				}
				if port.Role != nil {
					p.DefaultRole = port.Role
				}
				if port.Device != nil {
					p.SwConfigCPUName = port.Device
				}
				ports = append(ports, p)
			}
		}
	} else {
		// For DSA devices, use network info
		// Add LAN ports
		if len(boardJSON.Network.Lan.Ports) > 0 {
			for _, portName := range boardJSON.Network.Lan.Ports {
				role := "lan"
				ports = append(ports, Port{
					Name:        portName,
					DefaultRole: &role,
				})
			}
		} else if boardJSON.Network.Lan.Device != nil &&
			(*boardJSON.Network.Lan.Device == "lan" || *boardJSON.Network.Lan.Device == "eth0") {
			role := "lan"
			ports = append(ports, Port{
				Name:        *boardJSON.Network.Lan.Device,
				DefaultRole: &role,
			})
		}

		// Add WAN ports
		if boardJSON.Network.Wan != nil {
			if boardJSON.Network.Wan.Device != nil {
				role := "wan"
				ports = append(ports, Port{
					Name:        *boardJSON.Network.Wan.Device,
					DefaultRole: &role,
				})
			}
			for _, portName := range boardJSON.Network.Wan.Ports {
				role := "wan"
				ports = append(ports, Port{
					Name:        portName,
					DefaultRole: &role,
				})
			}
		}
	}

	if len(ports) == 0 {
		return nil, fmt.Errorf("found no ports for %s at %s", deviceConfig.ModelID, deviceConfig.IPAddr)
	}

	if isSwConfig {
		hasCPUPort := false
		for _, port := range ports {
			if port.SwConfigCPUName != nil {
				hasCPUPort = true
				break
			}
		}
		if !hasCPUPort {
			return nil, fmt.Errorf("found no CPU port for swConfig device %s at %s", deviceConfig.ModelID, deviceConfig.IPAddr)
		}
	}

	schema := &DeviceSchema{
		Name:           deviceConfig.ModelID,
		Version:        version,
		SwConfig:       isSwConfig,
		ConfigSections: configSections,
		Ports:          ports,
		Radios:         radios,
	}

	return schema, nil
}

func getBoardJSON(client *ssh.Client) (*BoardJSON, error) {
	output, err := client.Execute("cat /etc/board.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/board.json: %w", err)
	}

	var boardJSON BoardJSON
	if err := json.Unmarshal([]byte(output), &boardJSON); err != nil {
		return nil, fmt.Errorf("failed to parse board.json: %w", err)
	}

	return &boardJSON, nil
}

func getRadios(client *ssh.Client) ([]Radio, error) {
	output, err := client.Execute(`ubus call uci get '{"config": "wireless", "type": "wifi-device"}'`)
	if err != nil {
		// No wireless devices is not an error
		if output == "Command failed: Not found" {
			return []Radio{}, nil
		}
		return nil, fmt.Errorf("failed to get wireless config: %w", err)
	}

	var response WirelessConfigResponse
	if err := json.Unmarshal([]byte(output), &response); err != nil {
		return nil, fmt.Errorf("failed to parse wireless config: %w", err)
	}

	var radios []Radio
	for _, info := range response.Values {
		radio := Radio{
			Name: info.Name,
			Type: info.Type,
			Path: info.Path,
			Band: info.Band,
		}
		radios = append(radios, radio)
	}

	return radios, nil
}

func getConfigSections(client *ssh.Client) (map[string][]string, error) {
	// Get list of all config files
	_, err := client.Execute("ls /etc/config")
	if err != nil {
		return nil, fmt.Errorf("failed to list config files: %w", err)
	}

	// This is a simplified version - in the full implementation,
	// you would parse each config file to get section types
	sections := make(map[string][]string)

	// For now, return empty - this would need more complex parsing
	return sections, nil
}

func getDeviceVersion(client *ssh.Client) (string, error) {
	output, err := client.Execute("cat /etc/openwrt_release")
	if err != nil {
		return "", fmt.Errorf("failed to read /etc/openwrt_release: %w", err)
	}

	// Parse the version from the output
	// DISTRIB_RELEASE='23.05.0'
	lines := splitLines(output)
	for _, line := range lines {
		if len(line) > 16 && line[:16] == "DISTRIB_RELEASE=" {
			version := line[17 : len(line)-1] // Remove quotes
			return version, nil
		}
	}

	return "", fmt.Errorf("failed to find DISTRIB_RELEASE in /etc/openwrt_release")
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
