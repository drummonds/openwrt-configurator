package ssh

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MockClient simulates an OpenWRT device SSH connection with factory reset state
type MockClient struct {
	// Configuration
	ModelID       string
	InstalledPkgs []string

	// State tracking
	ExecutedCmds  []string
	UCIState      map[string]map[string]map[string]string // config -> section -> key -> value
	FailOnCommand string                                  // If set, fail when this command is executed

	// Callbacks
	OnExecute func(command string) (string, error)
}

// NewMockClient creates a new mock SSH client with factory reset state
func NewMockClient(modelID string) *MockClient {
	return &MockClient{
		ModelID:       modelID,
		InstalledPkgs: getFactoryPackages(),
		ExecutedCmds:  []string{},
		UCIState:      make(map[string]map[string]map[string]string),
	}
}

// Execute simulates executing a command on a factory reset OpenWRT device
func (m *MockClient) Execute(command string) (string, error) {
	m.ExecutedCmds = append(m.ExecutedCmds, command)

	// Check if we should fail on this command
	if m.FailOnCommand != "" && strings.Contains(command, m.FailOnCommand) {
		return "", fmt.Errorf("mock error: command failed")
	}

	// Custom callback
	if m.OnExecute != nil {
		return m.OnExecute(command)
	}

	// Handle specific commands
	if command == "cat /etc/board.json" {
		return m.getBoardJSON(), nil
	}

	if command == "opkg list-installed" {
		return m.getInstalledPackages(), nil
	}

	// Handle UCI commands
	if strings.HasPrefix(command, "uci set ") {
		m.handleUCISet(command)
		return "", nil
	}

	if strings.HasPrefix(command, "uci add_list ") {
		m.handleUCIAddList(command)
		return "", nil
	}

	if strings.HasPrefix(command, "uci commit") {
		return "", nil
	}

	if command == "reload_config" {
		return "", nil
	}

	// Handle opkg commands
	if strings.HasPrefix(command, "opkg remove ") {
		m.handleOpkgRemove(command)
		return "", nil
	}

	if strings.HasPrefix(command, "opkg install ") {
		m.handleOpkgInstall(command)
		return "", nil
	}

	if strings.HasPrefix(command, "opkg update") {
		return "", nil
	}

	// Handle delete commands
	if strings.Contains(command, "uci -q delete") {
		return "", nil
	}

	return "", nil
}

// ExecuteWithError runs a command and returns both stdout and error separately
func (m *MockClient) ExecuteWithError(command string) (string, error) {
	return m.Execute(command)
}

// Close simulates closing the SSH connection
func (m *MockClient) Close() error {
	return nil
}

// GetExecutedCommands returns all executed commands
func (m *MockClient) GetExecutedCommands() []string {
	return m.ExecutedCmds
}

// GetUCIValue retrieves a UCI value from the mock state
func (m *MockClient) GetUCIValue(config, section, key string) string {
	if configMap, ok := m.UCIState[config]; ok {
		if sectionMap, ok := configMap[section]; ok {
			return sectionMap[key]
		}
	}
	return ""
}

// mockBoardJSON represents a simplified board.json structure
type mockBoardJSON struct {
	Model struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"model"`
	Network struct {
		Lan struct {
			Ports []string `json:"ports"`
		} `json:"lan"`
		Wan struct {
			Device string `json:"device"`
		} `json:"wan"`
	} `json:"network"`
}

// getBoardJSON returns factory reset board.json content
func (m *MockClient) getBoardJSON() string {
	boardJSON := mockBoardJSON{}
	boardJSON.Model.ID = m.ModelID
	boardJSON.Model.Name = "Mock Device"
	boardJSON.Network.Lan.Ports = []string{"lan1", "lan2", "lan3", "lan4"}
	boardJSON.Network.Wan.Device = "eth0"

	data, _ := json.Marshal(boardJSON)
	return string(data)
}

// getInstalledPackages returns factory reset installed packages
func (m *MockClient) getInstalledPackages() string {
	var output strings.Builder
	for _, pkg := range m.InstalledPkgs {
		output.WriteString(fmt.Sprintf("%s - 1.0.0\n", pkg))
	}
	return output.String()
}

// getFactoryPackages returns the default packages on a factory reset device
func getFactoryPackages() []string {
	return []string{
		"base-files",
		"busybox",
		"dnsmasq",
		"dropbear",
		"firewall4",
		"kmod-gpio-button-hotplug",
		"libc",
		"logd",
		"mtd",
		"netifd",
		"opkg",
		"ppp",
		"ppp-mod-pppoe",
		"uci",
		"uclient-fetch",
		"urandom-seed",
		"urngd",
	}
}

// handleUCISet processes a "uci set" command
func (m *MockClient) handleUCISet(command string) {
	// Parse: uci set config.section.key='value' or uci set config.section=type
	parts := strings.SplitN(command, "uci set ", 2)
	if len(parts) < 2 {
		return
	}

	setPart := strings.TrimSpace(parts[1])
	eqIdx := strings.Index(setPart, "=")
	if eqIdx == -1 {
		return
	}

	left := setPart[:eqIdx]
	right := strings.Trim(setPart[eqIdx+1:], "'\"")

	// Parse left side: config.section or config.section.key
	dotParts := strings.Split(left, ".")
	if len(dotParts) < 2 {
		return
	}

	config := dotParts[0]
	section := dotParts[1]

	if m.UCIState[config] == nil {
		m.UCIState[config] = make(map[string]map[string]string)
	}
	if m.UCIState[config][section] == nil {
		m.UCIState[config][section] = make(map[string]string)
	}

	if len(dotParts) == 2 {
		// Setting section type: config.section=type
		m.UCIState[config][section]["_type"] = right
	} else {
		// Setting key value: config.section.key=value
		key := dotParts[2]
		m.UCIState[config][section][key] = right
	}
}

// handleUCIAddList processes a "uci add_list" command
func (m *MockClient) handleUCIAddList(command string) {
	// Parse: uci add_list config.section.key='value'
	parts := strings.SplitN(command, "uci add_list ", 2)
	if len(parts) < 2 {
		return
	}

	addPart := strings.TrimSpace(parts[1])
	eqIdx := strings.Index(addPart, "=")
	if eqIdx == -1 {
		return
	}

	left := addPart[:eqIdx]
	right := strings.Trim(addPart[eqIdx+1:], "'\"")

	// Parse left side: config.section.key
	dotParts := strings.Split(left, ".")
	if len(dotParts) != 3 {
		return
	}

	config := dotParts[0]
	section := dotParts[1]
	key := dotParts[2]

	if m.UCIState[config] == nil {
		m.UCIState[config] = make(map[string]map[string]string)
	}
	if m.UCIState[config][section] == nil {
		m.UCIState[config][section] = make(map[string]string)
	}

	// Append to existing value with space separator
	existing := m.UCIState[config][section][key]
	if existing == "" {
		m.UCIState[config][section][key] = right
	} else {
		m.UCIState[config][section][key] = existing + " " + right
	}
}

// handleOpkgRemove removes packages from installed list
func (m *MockClient) handleOpkgRemove(command string) {
	// Parse: opkg remove --force-removal-of-dependent-packages pkg1 pkg2 ...
	parts := strings.Fields(command)
	if len(parts) < 3 {
		return
	}

	// Find where package names start (after flags)
	startIdx := 2
	for i := 2; i < len(parts); i++ {
		if !strings.HasPrefix(parts[i], "-") {
			startIdx = i
			break
		}
	}

	packagesToRemove := parts[startIdx:]
	newInstalled := []string{}

	for _, installed := range m.InstalledPkgs {
		shouldRemove := false
		for _, toRemove := range packagesToRemove {
			if installed == toRemove {
				shouldRemove = true
				break
			}
		}
		if !shouldRemove {
			newInstalled = append(newInstalled, installed)
		}
	}

	m.InstalledPkgs = newInstalled
}

// handleOpkgInstall adds packages to installed list
func (m *MockClient) handleOpkgInstall(command string) {
	// Parse: opkg install pkg1 pkg2 ...
	parts := strings.Fields(command)
	if len(parts) < 3 {
		return
	}

	packagesToInstall := parts[2:]

	for _, pkg := range packagesToInstall {
		// Check if already installed
		alreadyInstalled := false
		for _, installed := range m.InstalledPkgs {
			if installed == pkg {
				alreadyInstalled = true
				break
			}
		}
		if !alreadyInstalled {
			m.InstalledPkgs = append(m.InstalledPkgs, pkg)
		}
	}
}
