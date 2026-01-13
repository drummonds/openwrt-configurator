package provision

import (
	"encoding/json"
	"testing"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/device"
	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
)

// TestFactoryResetProvisionBasic tests provisioning to a factory reset device
func TestFactoryResetProvisionBasic(t *testing.T) {
	// Create mock factory reset device
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

	// Create minimal config
	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  "ubnt,edgerouter-x",
				Hostname: "test-router",
				IPAddr:   "192.168.1.1",
			},
		},
		Config: config.ConfigConfig{
			System: &config.SystemConfig{
				System: []config.SystemSection{
					{
						Name:     stringPtr("system"),
						Hostname: stringPtr("test-router"),
						Timezone: stringPtr("UTC"),
					},
				},
			},
		},
	}

	// Get device schema
	deviceConfig := &oncConfig.Devices[0]
	deviceSchema := &device.DeviceSchema{
		Name: "ubnt,edgerouter-x",
		ConfigSections: map[string][]string{
			"system":   {"system"},
			"network":  {"interface"},
			"firewall": {"zone", "forwarding", "rule"},
		},
	}

	// Get state
	state, err := device.GetOpenWrtState(oncConfig, deviceConfig, deviceSchema)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	// Get commands
	commands, err := device.GetDeviceScript(state, mockClient)
	if err != nil {
		t.Fatalf("Failed to get device script: %v", err)
	}

	// Execute commands on mock
	for _, cmd := range commands {
		_, err := mockClient.Execute(cmd)
		if err != nil {
			t.Fatalf("Command failed: %s, error: %v", cmd, err)
		}
	}

	// Verify UCI state
	hostname := mockClient.GetUCIValue("system", "system", "hostname")
	if hostname != "test-router" {
		t.Errorf("Expected hostname 'test-router', got '%s'", hostname)
	}

	timezone := mockClient.GetUCIValue("system", "system", "timezone")
	if timezone != "UTC" {
		t.Errorf("Expected timezone 'UTC', got '%s'", timezone)
	}

	// Verify commit was called
	hasCommit := false
	for _, cmd := range mockClient.GetExecutedCommands() {
		if cmd == "uci commit" {
			hasCommit = true
			break
		}
	}
	if !hasCommit {
		t.Error("Expected 'uci commit' to be executed")
	}
}

// TestFactoryResetWithPackages tests package installation/removal
func TestFactoryResetWithPackages(t *testing.T) {
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

	enabled := true
	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  "ubnt,edgerouter-x",
				Hostname: "test-router",
				IPAddr:   "192.168.1.1",
				Enabled:  &enabled,
				Tags: map[string]any{
					"role": "router",
				},
			},
		},
		PackageProfiles: []config.PackageProfile{
			{
				Packages: []string{"sqm-scripts", "luci-app-sqm", "-firewall4"},
			},
		},
		Config: config.ConfigConfig{},
	}

	deviceConfig := &oncConfig.Devices[0]
	deviceSchema := &device.DeviceSchema{
		Name:           "ubnt,edgerouter-x",
		ConfigSections: map[string][]string{},
	}

	state, err := device.GetOpenWrtState(oncConfig, deviceConfig, deviceSchema)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	// Verify packages to install/uninstall
	if len(state.PackagesToInstall) != 2 {
		t.Errorf("Expected 2 packages to install, got %d", len(state.PackagesToInstall))
	}

	if len(state.PackagesToUninstall) != 1 {
		t.Errorf("Expected 1 package to uninstall, got %d", len(state.PackagesToUninstall))
	}

	// Execute commands
	commands, err := device.GetDeviceScript(state, mockClient)
	if err != nil {
		t.Fatalf("Failed to get device script: %v", err)
	}

	for _, cmd := range commands {
		_, _ = mockClient.Execute(cmd)
	}

	// Verify firewall4 was removed
	hasFirewall4 := false
	for _, pkg := range mockClient.InstalledPkgs {
		if pkg == "firewall4" {
			hasFirewall4 = true
			break
		}
	}
	if hasFirewall4 {
		t.Error("Expected firewall4 to be removed")
	}

	// Verify sqm-scripts was added
	hasSQM := false
	for _, pkg := range mockClient.InstalledPkgs {
		if pkg == "sqm-scripts" {
			hasSQM = true
			break
		}
	}
	if !hasSQM {
		t.Error("Expected sqm-scripts to be installed")
	}
}

// TestFactoryResetWithConditionals tests conditional config application
func TestFactoryResetWithConditionals(t *testing.T) {
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

	enabled := true
	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  "ubnt,edgerouter-x",
				Hostname: "test-router",
				IPAddr:   "192.168.1.1",
				Enabled:  &enabled,
				Tags: map[string]any{
					"role": "router",
				},
			},
		},
		Config: config.ConfigConfig{
			Network: &config.NetworkConfig{
				Interface: []config.InterfaceSection{
					{
						If:     stringPtr("device.tag.role == 'router'"),
						Name:   stringPtr("wan"),
						Device: stringPtr("eth0"),
						Proto:  stringPtr("static"),
						IPAddr: stringPtr("10.0.0.1"),
					},
					{
						If:     stringPtr("device.tag.role == 'ap'"),
						Name:   stringPtr("lan"),
						Device: stringPtr("br-lan"),
						Proto:  stringPtr("dhcp"),
					},
				},
			},
		},
	}

	deviceConfig := &oncConfig.Devices[0]
	deviceSchema := &device.DeviceSchema{
		Name:           "ubnt,edgerouter-x",
		ConfigSections: map[string][]string{},
	}

	state, err := device.GetOpenWrtState(oncConfig, deviceConfig, deviceSchema)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	// Execute commands
	commands, err := device.GetDeviceScript(state, mockClient)
	if err != nil {
		t.Fatalf("Failed to get device script: %v", err)
	}

	for _, cmd := range commands {
		_, _ = mockClient.Execute(cmd)
	}

	// Verify WAN interface was created (condition matched)
	wanProto := mockClient.GetUCIValue("network", "wan", "proto")
	if wanProto != "static" {
		t.Errorf("Expected WAN proto 'static', got '%s'", wanProto)
	}

	wanIP := mockClient.GetUCIValue("network", "wan", "ipaddr")
	if wanIP != "10.0.0.1" {
		t.Errorf("Expected WAN IP '10.0.0.1', got '%s'", wanIP)
	}

	// Verify LAN interface was NOT created (condition didn't match)
	lanProto := mockClient.GetUCIValue("network", "lan", "proto")
	if lanProto != "" {
		t.Errorf("Expected LAN interface not to be created, but found proto '%s'", lanProto)
	}
}

// TestFactoryResetVerifyDevice tests device verification
func TestFactoryResetVerifyDevice(t *testing.T) {
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

	// Test verifyDevice function
	boardJSON, err := verifyDevice(mockClient, "ubnt,edgerouter-x")
	if err != nil {
		t.Fatalf("Failed to verify device: %v", err)
	}

	if boardJSON.Model.ID != "ubnt,edgerouter-x" {
		t.Errorf("Expected model ID 'ubnt,edgerouter-x', got '%s'", boardJSON.Model.ID)
	}

	// Test mismatched model ID
	_, err = verifyDevice(mockClient, "wrong-model")
	if err == nil {
		t.Error("Expected error for mismatched model ID")
	}
}

// TestFactoryResetCommandFailure tests handling of command failures
func TestFactoryResetCommandFailure(t *testing.T) {
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")
	mockClient.FailOnCommand = "uci set system"

	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  "ubnt,edgerouter-x",
				Hostname: "test-router",
				IPAddr:   "192.168.1.1",
			},
		},
		Config: config.ConfigConfig{
			System: &config.SystemConfig{
				System: []config.SystemSection{
					{
						Name:     stringPtr("system"),
						Hostname: stringPtr("test-router"),
					},
				},
			},
		},
	}

	deviceConfig := &oncConfig.Devices[0]
	deviceSchema := &device.DeviceSchema{
		Name:           "ubnt,edgerouter-x",
		ConfigSections: map[string][]string{},
	}

	state, err := device.GetOpenWrtState(oncConfig, deviceConfig, deviceSchema)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	commands, err := device.GetDeviceScript(state, mockClient)
	if err != nil {
		t.Fatalf("Failed to get device script: %v", err)
	}

	// Execute commands - should fail
	hasError := false
	for _, cmd := range commands {
		_, err := mockClient.Execute(cmd)
		if err != nil {
			hasError = true
			break
		}
	}

	if !hasError {
		t.Error("Expected command execution to fail")
	}
}

// TestFactoryResetMultipleDevices tests configuration for multiple device types
func TestFactoryResetMultipleDevices(t *testing.T) {
	// Test that different device types get different configs

	// Router device
	enabled := true

	oncConfig := &config.ONCConfig{
		Devices: []config.DeviceConfig{
			{
				ModelID:  "ubnt,edgerouter-x",
				Hostname: "router",
				IPAddr:   "192.168.1.1",
				Enabled:  &enabled,
				Tags: map[string]any{
					"role": "router",
				},
			},
		},
		PackageProfiles: []config.PackageProfile{
			{
				If:       stringPtr("device.tag.role == 'router'"),
				Packages: []string{"ppp-mod-pppoe"},
			},
			{
				If:       stringPtr("device.tag.role == 'ap'"),
				Packages: []string{"-firewall4"},
			},
		},
		Config: config.ConfigConfig{},
	}

	deviceConfig := &oncConfig.Devices[0]
	deviceSchema := &device.DeviceSchema{
		Name:           "ubnt,edgerouter-x",
		ConfigSections: map[string][]string{},
	}

	state, err := device.GetOpenWrtState(oncConfig, deviceConfig, deviceSchema)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}

	// Verify router gets pppoe package
	hasPPPoE := false
	for _, pkg := range state.PackagesToInstall {
		if pkg.Name == "ppp-mod-pppoe" {
			hasPPPoE = true
			break
		}
	}
	if !hasPPPoE {
		t.Error("Expected router to have ppp-mod-pppoe in packages to install")
	}

	// Verify router doesn't get firewall removal
	hasFirewallRemoval := false
	for _, pkg := range state.PackagesToUninstall {
		if pkg == "firewall4" {
			hasFirewallRemoval = true
			break
		}
	}
	if hasFirewallRemoval {
		t.Error("Expected router not to have firewall4 in packages to uninstall")
	}
}

// TestFactoryResetBoardJSON tests various board.json configurations
func TestFactoryResetBoardJSON(t *testing.T) {
	testCases := []struct {
		name    string
		modelID string
	}{
		{"EdgeRouter X", "ubnt,edgerouter-x"},
		{"TP-Link EAP245", "tplink,eap245-v3"},
		{"Netgear R7800", "netgear,r7800"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := ssh.NewMockClient(tc.modelID)

			output, err := mockClient.Execute("cat /etc/board.json")
			if err != nil {
				t.Fatalf("Failed to get board.json: %v", err)
			}

			var boardJSON device.BoardJSON
			if err := json.Unmarshal([]byte(output), &boardJSON); err != nil {
				t.Fatalf("Failed to parse board.json: %v", err)
			}

			if boardJSON.Model.ID != tc.modelID {
				t.Errorf("Expected model ID '%s', got '%s'", tc.modelID, boardJSON.Model.ID)
			}
		})
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
