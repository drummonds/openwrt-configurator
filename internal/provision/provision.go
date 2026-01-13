package provision

import (
	"encoding/json"
	"fmt"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/device"
	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
)

// ProvisionConfig provisions configuration to all enabled devices
func ProvisionConfig(oncConfig *config.ONCConfig) error {
	// Get enabled devices
	var enabledDevices []config.DeviceConfig
	for _, dev := range oncConfig.Devices {
		if dev.Enabled == nil || *dev.Enabled {
			enabledDevices = append(enabledDevices, dev)
		}
	}

	// Get device schemas
	deviceSchemas := make(map[string]*device.DeviceSchema)
	for _, dev := range enabledDevices {
		schema, err := device.GetDeviceSchema(&dev)
		if err != nil {
			return fmt.Errorf("failed to get device schema for %s: %w", dev.ModelID, err)
		}
		deviceSchemas[dev.ModelID] = schema
	}

	// Provision each device
	for _, dev := range enabledDevices {
		if dev.IPAddr == "" || dev.ProvisioningConfig == nil {
			fmt.Printf("Skipping device %s: no IP address or provisioning config\n", dev.Hostname)
			continue
		}

		schema := deviceSchemas[dev.ModelID]
		if schema == nil {
			return fmt.Errorf("device schema not found for device: %s@%s", dev.ModelID, dev.IPAddr)
		}

		// Get state
		state, err := device.GetOpenWrtState(oncConfig, &dev, schema)
		if err != nil {
			return fmt.Errorf("failed to get state for device %s: %w", dev.Hostname, err)
		}

		// Provision
		if err := provisionDevice(&dev, schema, state); err != nil {
			return fmt.Errorf("failed to provision device %s: %w", dev.Hostname, err)
		}
	}

	return nil
}

func provisionDevice(deviceConfig *config.DeviceConfig, deviceSchema *device.DeviceSchema, state *device.OpenWrtState) error {
	fmt.Printf("Provisioning %s@%s...\n", deviceConfig.ProvisioningConfig.SSHAuth.Username, deviceConfig.IPAddr)

	// Connect via SSH
	fmt.Println("Connecting over SSH...")
	client, err := ssh.Connect(
		deviceConfig.IPAddr,
		deviceConfig.ProvisioningConfig.SSHAuth.Username,
		deviceConfig.ProvisioningConfig.SSHAuth.Password,
	)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()
	fmt.Println("Connected.")

	// Verify device
	fmt.Println("Verifying device...")
	boardJSON, err := verifyDevice(client, deviceConfig.ModelID)
	if err != nil {
		return fmt.Errorf("failed to verify device: %w", err)
	}
	if boardJSON.Model.ID != deviceConfig.ModelID {
		return fmt.Errorf("mismatching device model id: expected %s but found %s in /etc/board.json",
			deviceConfig.ModelID, boardJSON.Model.ID)
	}
	fmt.Println("Verified.")

	// Get commands
	commands, err := device.GetDeviceScript(state, client)
	if err != nil {
		return fmt.Errorf("failed to get device script: %w", err)
	}

	// Execute commands
	fmt.Println("Setting configuration...")
	revertCommands := getRevertCommands()

	for _, cmd := range commands {
		output, err := client.ExecuteWithError(cmd)
		if err != nil {
			fmt.Printf("Command failed: %s\n", cmd)
			fmt.Printf("Error: %s\n", output)
			fmt.Println("Reverting...")

			// Revert changes
			for _, revertCmd := range revertCommands {
				_, _ = client.Execute(revertCmd)
			}

			fmt.Println("Reverted.")
			return fmt.Errorf("failed to execute command: %s", cmd)
		}
	}

	fmt.Println("Configuration set.")
	fmt.Println("Provisioning completed.")

	return nil
}

func verifyDevice(client ssh.SSHExecutor, expectedModelID string) (*device.BoardJSON, error) {
	output, err := client.Execute("cat /etc/board.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read /etc/board.json: %w", err)
	}

	var boardJSON device.BoardJSON
	if err := json.Unmarshal([]byte(output), &boardJSON); err != nil {
		return nil, fmt.Errorf("failed to parse board.json: %w", err)
	}

	if boardJSON.Model.ID != expectedModelID {
		return nil, fmt.Errorf("device model mismatch: expected %s, got %s", expectedModelID, boardJSON.Model.ID)
	}

	return &boardJSON, nil
}

func getRevertCommands() []string {
	// These are the common configs that should be reverted
	configs := []string{"system", "network", "firewall", "dhcp", "wireless", "dropbear"}
	var commands []string

	for _, cfg := range configs {
		commands = append(commands, fmt.Sprintf("uci revert %s", cfg))
	}

	return commands
}
