package export

import (
	"strings"
	"testing"

	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
)

func TestExportConfig(t *testing.T) {
	// Create mock client with UCI configuration
	mockClient := ssh.NewMockClient("ubnt,edgerouter-x")

	// Save the original board.json response
	boardJSONResponse, _ := mockClient.Execute("cat /etc/board.json")
	packagesResponse, _ := mockClient.Execute("opkg list-installed")

	// Configure mock to respond to UCI commands
	mockClient.OnExecute = func(command string) (string, error) {
		switch {
		case command == "cat /etc/board.json":
			return boardJSONResponse, nil

		case command == "uci show system":
			return `system.@system[0]=system
system.@system[0].hostname='test-router'
system.@system[0].timezone='UTC'
system.@system[0].zonename='UTC'
`, nil

		case command == "uci show network":
			return `network.loopback=interface
network.loopback.device='lo'
network.loopback.proto='static'
network.loopback.ipaddr='127.0.0.1'
network.loopback.netmask='255.0.0.0'
network.lan=interface
network.lan.device='br-lan'
network.lan.proto='static'
network.lan.ipaddr='192.168.1.1'
network.lan.netmask='255.255.255.0'
`, nil

		case command == "uci show wireless":
			return `wireless.radio0=wifi-device
wireless.radio0.type='mac80211'
wireless.radio0.band='2g'
wireless.radio0.channel='auto'
wireless.default_radio0=wifi-iface
wireless.default_radio0.device='radio0'
wireless.default_radio0.mode='ap'
wireless.default_radio0.ssid='OpenWrt'
wireless.default_radio0.encryption='psk2'
wireless.default_radio0.network='lan'
`, nil

		case command == "uci show dropbear":
			return `dropbear.@dropbear[0]=dropbear
dropbear.@dropbear[0].PasswordAuth='on'
dropbear.@dropbear[0].RootPasswordAuth='on'
dropbear.@dropbear[0].Port='22'
`, nil

		case command == "opkg list-installed":
			return packagesResponse, nil

		default:
			return "", nil
		}
	}

	// Export configuration using the mock client
	oncConfig, err := ExportConfigFromClient(mockClient, "ubnt,edgerouter-x", "192.168.1.1", "root", "password")
	if err != nil {
		t.Fatalf("Failed to export config: %v", err)
	}

	// Verify device configuration
	if len(oncConfig.Devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(oncConfig.Devices))
	}

	device := oncConfig.Devices[0]
	if device.ModelID != "ubnt,edgerouter-x" {
		t.Errorf("Expected model ID 'ubnt,edgerouter-x', got '%s'", device.ModelID)
	}

	if device.IPAddr != "192.168.1.1" {
		t.Errorf("Expected IP '192.168.1.1', got '%s'", device.IPAddr)
	}

	if device.Hostname != "test-router" {
		t.Errorf("Expected hostname 'test-router', got '%s'", device.Hostname)
	}

	// Verify system configuration
	if oncConfig.Config.System == nil {
		t.Fatal("System config is nil")
	}

	if len(oncConfig.Config.System.System) == 0 {
		t.Fatal("No system sections found")
	}

	systemSection := oncConfig.Config.System.System[0]
	if systemSection.Hostname == nil || *systemSection.Hostname != "test-router" {
		t.Error("System hostname not correctly exported")
	}

	if systemSection.Timezone == nil || *systemSection.Timezone != "UTC" {
		t.Error("System timezone not correctly exported")
	}

	// Verify network configuration
	if oncConfig.Config.Network == nil {
		t.Fatal("Network config is nil")
	}

	if len(oncConfig.Config.Network.Interface) < 2 {
		t.Errorf("Expected at least 2 interfaces, got %d", len(oncConfig.Config.Network.Interface))
	}

	// Find LAN interface
	var lanInterface *struct {
		Name    *string
		Device  *string
		Proto   *string
		IPAddr  *string
		Netmask *string
	}

	for _, iface := range oncConfig.Config.Network.Interface {
		if iface.Name != nil && *iface.Name == "lan" {
			lanInterface = &struct {
				Name    *string
				Device  *string
				Proto   *string
				IPAddr  *string
				Netmask *string
			}{
				Name:    iface.Name,
				Device:  iface.Device,
				Proto:   iface.Proto,
				IPAddr:  iface.IPAddr,
				Netmask: iface.Netmask,
			}
			break
		}
	}

	if lanInterface == nil {
		t.Fatal("LAN interface not found")
	}

	if lanInterface.IPAddr == nil || *lanInterface.IPAddr != "192.168.1.1" {
		t.Error("LAN IP address not correctly exported")
	}

	// Verify wireless configuration (optional - may not exist on all devices)
	// Wireless config is tested separately in other tests

	// Verify packages
	if len(oncConfig.PackageProfiles) == 0 {
		t.Fatal("No package profiles found")
	}

	if len(oncConfig.PackageProfiles[0].Packages) == 0 {
		t.Error("No packages found")
	}
}

func TestReadSystemConfig(t *testing.T) {
	mockClient := ssh.NewMockClient("test-device")
	mockClient.OnExecute = func(command string) (string, error) {
		if command == "uci show system" {
			return `system.@system[0]=system
system.@system[0].hostname='my-router'
system.@system[0].timezone='America/New_York'
system.@system[0].zonename='EST5EDT'
`, nil
		}
		return "", nil
	}

	info, err := readSystemConfig(mockClient)
	if err != nil {
		t.Fatalf("Failed to read system config: %v", err)
	}

	if info.Hostname != "my-router" {
		t.Errorf("Expected hostname 'my-router', got '%s'", info.Hostname)
	}

	if info.Config == nil || len(info.Config.System) == 0 {
		t.Fatal("System config not properly parsed")
	}

	section := info.Config.System[0]
	if section.Hostname == nil || *section.Hostname != "my-router" {
		t.Error("Hostname not correctly parsed")
	}

	if section.Timezone == nil || *section.Timezone != "America/New_York" {
		t.Error("Timezone not correctly parsed")
	}
}

func TestReadNetworkConfig(t *testing.T) {
	mockClient := ssh.NewMockClient("test-device")
	mockClient.OnExecute = func(command string) (string, error) {
		if command == "uci show network" {
			return `network.lan=interface
network.lan.proto='static'
network.lan.device='br-lan'
network.lan.ipaddr='192.168.1.1'
network.lan.netmask='255.255.255.0'
network.wan=interface
network.wan.proto='dhcp'
network.wan.device='eth0'
`, nil
		}
		return "", nil
	}

	config, err := readNetworkConfig(mockClient)
	if err != nil {
		t.Fatalf("Failed to read network config: %v", err)
	}

	if len(config.Interface) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(config.Interface))
	}

	// Check LAN interface
	var lanFound bool
	for _, iface := range config.Interface {
		if iface.Name != nil && *iface.Name == "lan" {
			lanFound = true
			if iface.Proto == nil || *iface.Proto != "static" {
				t.Error("LAN proto not correctly parsed")
			}
			if iface.IPAddr == nil || *iface.IPAddr != "192.168.1.1" {
				t.Error("LAN IP not correctly parsed")
			}
		}
	}

	if !lanFound {
		t.Error("LAN interface not found")
	}
}

func TestReadInstalledPackages(t *testing.T) {
	mockClient := ssh.NewMockClient("test-device")

	packages, err := readInstalledPackages(mockClient)
	if err != nil {
		t.Fatalf("Failed to read packages: %v", err)
	}

	if len(packages) == 0 {
		t.Error("Expected packages to be returned")
	}

	// Check for some expected factory packages
	expectedPackages := []string{"base-files", "busybox", "firewall4"}
	for _, expected := range expectedPackages {
		found := false
		for _, pkg := range packages {
			if strings.Contains(pkg, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected package '%s' not found", expected)
		}
	}
}

func TestExportConfigAutoDetectModel(t *testing.T) {
	// Test that model ID is auto-detected when not provided
	mockClient := ssh.NewMockClient("tplink,eap245-v3")

	// Save the original responses
	boardJSONResponse, _ := mockClient.Execute("cat /etc/board.json")
	packagesResponse, _ := mockClient.Execute("opkg list-installed")

	// Configure mock to respond to UCI commands
	mockClient.OnExecute = func(command string) (string, error) {
		switch {
		case command == "cat /etc/board.json":
			return boardJSONResponse, nil
		case command == "uci show system":
			return `system.@system[0]=system
system.@system[0].hostname='auto-detect-test'
`, nil
		case command == "uci show network":
			return `network.lan=interface
network.lan.proto='static'
network.lan.device='br-lan'
network.lan.ipaddr='192.168.1.1'
`, nil
		case command == "uci show wireless":
			return "", nil
		case command == "uci show dropbear":
			return "", nil
		case command == "opkg list-installed":
			return packagesResponse, nil
		default:
			return "", nil
		}
	}

	// Export configuration WITHOUT providing model ID (empty string)
	oncConfig, err := ExportConfigFromClient(mockClient, "", "192.168.1.1", "root", "password")
	if err != nil {
		t.Fatalf("Failed to export config: %v", err)
	}

	// Verify that model ID was auto-detected
	if len(oncConfig.Devices) != 1 {
		t.Fatalf("Expected 1 device, got %d", len(oncConfig.Devices))
	}

	device := oncConfig.Devices[0]
	if device.ModelID != "tplink,eap245-v3" {
		t.Errorf("Expected auto-detected model ID 'tplink,eap245-v3', got '%s'", device.ModelID)
	}

	if device.Hostname != "auto-detect-test" {
		t.Errorf("Expected hostname 'auto-detect-test', got '%s'", device.Hostname)
	}
}
