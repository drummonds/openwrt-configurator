package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/device"
	"github.com/drummonds/openwrt-configurator.git/internal/export"
	"github.com/drummonds/openwrt-configurator.git/internal/provision"
)

const version = "0.0.4"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Check for global flags
	if os.Args[1] == "-h" || os.Args[1] == "--help" {
		printUsage()
		os.Exit(0)
	}

	if os.Args[1] == "-v" || os.Args[1] == "--version" {
		fmt.Printf("openwrt-configurator version %s\n", version)
		os.Exit(0)
	}

	// Parse subcommand
	subcommand := os.Args[1]

	switch subcommand {
	case "provision":
		if err := provisionCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "print-uci-commands":
		if err := printUciCommandsCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "export-config":
		if err := exportConfigCmd(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `openwrt-configurator - OpenWrt Configuration Tool

Usage:
  openwrt-configurator <command> [arguments]

Available Commands:
  provision              Provision configuration to devices
  print-uci-commands     Print UCI commands for configuration
  export-config          Export configuration from an OpenWRT device

Flags:
  -h, --help             Show help
  -v, --version          Show version

Use "openwrt-configurator <command> -h" for more information about a command.
`)
}

func provisionCmd(args []string) error {
	fs := flag.NewFlagSet("provision", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Provision configuration to devices

Usage:
  openwrt-configurator provision [flags] <config-file>

Flags:
  -h, --help    Show help

Arguments:
  config-file   Path to the configuration JSON file
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("requires exactly one argument: config-file")
	}

	configPath := fs.Arg(0)

	// Read config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config
	var oncConfig config.ONCConfig
	if err := json.Unmarshal(configData, &oncConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate and provision
	if err := provision.ProvisionConfig(&oncConfig); err != nil {
		return fmt.Errorf("provisioning failed: %w", err)
	}

	return nil
}

func printUciCommandsCmd(args []string) error {
	fs := flag.NewFlagSet("print-uci-commands", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Print UCI commands for configuration

Usage:
  openwrt-configurator print-uci-commands [flags] <config-file>

Flags:
  -h, --help    Show help

Arguments:
  config-file   Path to the configuration JSON file
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("requires exactly one argument: config-file")
	}

	configPath := fs.Arg(0)

	// Read config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse config
	var oncConfig config.ONCConfig
	if err := json.Unmarshal(configData, &oncConfig); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Get enabled devices
	devices := getEnabledDevices(&oncConfig)

	// Get device schemas for all devices
	deviceSchemas := make(map[string]*device.DeviceSchema)
	for _, dev := range devices {
		schema, err := device.GetDeviceSchema(&dev)
		if err != nil {
			return fmt.Errorf("failed to get device schema for %s: %w", dev.ModelID, err)
		}
		deviceSchemas[dev.ModelID] = schema
	}

	// Generate and print commands for each device
	for _, dev := range devices {
		schema := deviceSchemas[dev.ModelID]
		state, err := device.GetOpenWrtState(&oncConfig, &dev, schema)
		if err != nil {
			return fmt.Errorf("failed to get state for device %s: %w", dev.Hostname, err)
		}

		commands, err := device.GetDeviceScript(state, nil)
		if err != nil {
			return fmt.Errorf("failed to get commands for device %s: %w", dev.Hostname, err)
		}

		fmt.Printf("# device %s\n", dev.Hostname)
		for _, cmd := range commands {
			fmt.Println(cmd)
		}
	}

	return nil
}

func exportConfigCmd(args []string) error {
	fs := flag.NewFlagSet("export-config", flag.ExitOnError)

	modelID := fs.String("model", "", "Device model ID (e.g., ubnt,edgerouter-x)")
	ipAddr := fs.String("ip", "", "Device IP address")
	username := fs.String("user", "root", "SSH username")
	password := fs.String("pass", "", "SSH password")
	output := fs.String("output", "", "Output file (default: stdout)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Export configuration from an OpenWRT device

Usage:
  openwrt-configurator export-config [flags]

Flags:
  -model string     Device model ID (optional, auto-detected from device)
  -ip string        Device IP address (required)
  -user string      SSH username (default "root")
  -pass string      SSH password (required)
  -output string    Output file (default: stdout)
  -h, --help        Show help

Examples:
  # Export to stdout (model auto-detected)
  openwrt-configurator export-config -ip 192.168.1.1 -pass mypassword

  # Export to file
  openwrt-configurator export-config -ip 192.168.1.1 -pass mypassword -output config.json

  # Export with explicit model ID (for verification)
  openwrt-configurator export-config -model ubnt,edgerouter-x -ip 192.168.1.1 -pass mypassword -output config.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if *ipAddr == "" {
		fs.Usage()
		return fmt.Errorf("required flag: -ip")
	}
	if *password == "" {
		fs.Usage()
		return fmt.Errorf("required flag: -pass")
	}

	// Export configuration from device
	fmt.Fprintf(os.Stderr, "Connecting to %s@%s...\n", *username, *ipAddr)
	oncConfig, err := export.ExportConfig(*modelID, *ipAddr, *username, *password)
	if err != nil {
		return fmt.Errorf("failed to export config: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Configuration exported successfully.\n")

	// Marshal to JSON with indentation
	jsonData, err := json.MarshalIndent(oncConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file or stdout
	if *output != "" {
		if err := os.WriteFile(*output, jsonData, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Configuration written to %s\n", *output)
	} else {
		fmt.Println(string(jsonData))
	}

	return nil
}

func getEnabledDevices(cfg *config.ONCConfig) []config.DeviceConfig {
	var enabled []config.DeviceConfig
	for _, dev := range cfg.Devices {
		if dev.Enabled == nil || *dev.Enabled {
			enabled = append(enabled, dev)
		}
	}
	return enabled
}
