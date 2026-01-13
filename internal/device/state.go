package device

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/drummonds/openwrt-configurator.git/internal/condition"
	"github.com/drummonds/openwrt-configurator.git/internal/config"
	"github.com/drummonds/openwrt-configurator.git/internal/ssh"
	"github.com/drummonds/openwrt-configurator.git/internal/uci"
)

// OpenWrtState represents the state to be applied to a device
type OpenWrtState struct {
	Config                map[string]any
	PackagesToInstall     []uci.Package
	PackagesToUninstall   []string
	ConfigSectionsToReset map[string][]string
}

// GetOpenWrtState generates the OpenWrt state for a device
func GetOpenWrtState(oncConfig *config.ONCConfig, deviceConfig *config.DeviceConfig, deviceSchema *DeviceSchema) (*OpenWrtState, error) {
	ctx := &condition.ConditionContext{
		DeviceConfig: deviceConfig,
		DeviceSchema: &condition.DeviceSchema{
			SwConfig: deviceSchema.SwConfig,
			Version:  deviceSchema.Version,
		},
	}

	// Resolve config
	openWrtConfig, err := resolveConfig(oncConfig, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve config: %w", err)
	}

	// Get packages
	packagesToInstall, packagesToUninstall := resolvePackages(oncConfig, ctx)

	// Get config sections to reset
	configsToNotReset := resolveConfigsToNotReset(oncConfig, ctx)
	configSectionsToReset := getConfigSectionsToReset(deviceSchema, configsToNotReset)

	state := &OpenWrtState{
		Config:                openWrtConfig,
		PackagesToInstall:     packagesToInstall,
		PackagesToUninstall:   packagesToUninstall,
		ConfigSectionsToReset: configSectionsToReset,
	}

	return state, nil
}

func resolveConfig(oncConfig *config.ONCConfig, ctx *condition.ConditionContext) (map[string]any, error) {
	resolved := make(map[string]any)

	// Convert config to map for easier processing
	configData, err := json.Marshal(oncConfig.Config)
	if err != nil {
		return nil, err
	}

	var configMap map[string]any
	if err := json.Unmarshal(configData, &configMap); err != nil {
		return nil, err
	}

	// Process each config section
	for configKey, configValue := range configMap {
		if configKey == "extra" {
			continue
		}

		configObj, ok := configValue.(map[string]any)
		if !ok {
			continue
		}

		// Apply conditions to the config object
		appliedConfig := applyObject(configObj, ctx)
		if len(appliedConfig) == 0 {
			continue
		}

		// Process sections within the config
		resolvedSections := make(map[string]any)
		for sectionKey, sectionValue := range appliedConfig {
			if strings.HasPrefix(sectionKey, ".") {
				continue
			}

			sections, ok := sectionValue.([]any)
			if !ok {
				continue
			}

			var resolvedSectionList []any
			for _, section := range sections {
				sectionMap, ok := section.(map[string]any)
				if !ok {
					continue
				}

				resolvedSection := applyObject(sectionMap, ctx)
				if len(resolvedSection) > 0 {
					resolvedSectionList = append(resolvedSectionList, resolvedSection)
				}
			}

			if len(resolvedSectionList) > 0 {
				resolvedSections[sectionKey] = resolvedSectionList
			}
		}

		if len(resolvedSections) > 0 {
			resolved[configKey] = resolvedSections
		}
	}

	return resolved, nil
}

func applyObject(obj map[string]any, ctx *condition.ConditionContext) map[string]any {
	// Check if condition
	var conditionStr *string
	if ifVal, ok := obj[".if"]; ok {
		if s, ok := ifVal.(string); ok {
			conditionStr = &s
		}
	}

	matches := condition.Evaluate(conditionStr, ctx)
	if !matches {
		return make(map[string]any)
	}

	// Apply overrides
	result := make(map[string]any)
	for k, v := range obj {
		if k != ".if" && k != ".overrides" {
			result[k] = v
		}
	}

	// Process overrides
	if overridesVal, ok := obj[".overrides"]; ok {
		overrides, ok := overridesVal.([]any)
		if ok {
			for _, override := range overrides {
				overrideMap, ok := override.(map[string]any)
				if !ok {
					continue
				}

				var overrideCondition *string
				if ifVal, ok := overrideMap[".if"]; ok {
					if s, ok := ifVal.(string); ok {
						overrideCondition = &s
					}
				}

				if condition.Evaluate(overrideCondition, ctx) {
					if overrideData, ok := overrideMap["override"].(map[string]any); ok {
						for k, v := range overrideData {
							result[k] = v
						}
					}
				}
			}
		}
	}

	return result
}

func resolvePackages(oncConfig *config.ONCConfig, ctx *condition.ConditionContext) ([]uci.Package, []string) {
	var allPackages []string

	for _, profile := range oncConfig.PackageProfiles {
		if condition.Evaluate(profile.If, ctx) {
			allPackages = append(allPackages, profile.Packages...)
		}
	}

	// Deduplicate
	packageSet := make(map[string]bool)
	for _, pkg := range allPackages {
		packageSet[pkg] = true
	}

	var install []uci.Package
	var uninstall []string

	for pkg := range packageSet {
		if len(pkg) > 0 && pkg[0] == '-' {
			uninstall = append(uninstall, pkg[1:])
		} else {
			// Check if package has version specifier
			parts := strings.Split(pkg, "@")
			p := uci.Package{Name: parts[0]}
			if len(parts) > 1 {
				p.Version = parts[1]
			}
			install = append(install, p)
		}
	}

	return install, uninstall
}

func resolveConfigsToNotReset(oncConfig *config.ONCConfig, ctx *condition.ConditionContext) []string {
	var configs []string

	for _, item := range oncConfig.ConfigsToNotReset {
		if condition.Evaluate(item.If, ctx) {
			configs = append(configs, item.Configs...)
		}
	}

	return configs
}

func getConfigSectionsToReset(deviceSchema *DeviceSchema, configsToNotReset []string) map[string][]string {
	result := make(map[string][]string)

	notResetSet := make(map[string]bool)
	for _, cfg := range configsToNotReset {
		notResetSet[cfg] = true
	}

	for configKey, sectionKeys := range deviceSchema.ConfigSections {
		// Check if entire config should not be reset
		if notResetSet[configKey+".*"] {
			continue
		}

		var sectionsToReset []string
		for _, sectionKey := range sectionKeys {
			fullKey := configKey + "." + sectionKey
			if !notResetSet[fullKey] {
				sectionsToReset = append(sectionsToReset, sectionKey)
			}
		}

		if len(sectionsToReset) > 0 {
			result[configKey] = sectionsToReset
		}
	}

	return result
}

// GetDeviceScript generates the script commands for a device
func GetDeviceScript(state *OpenWrtState, sshClient ssh.SSHExecutor) ([]string, error) {
	var commands []string

	// Get installed packages if SSH client is provided
	var installedPackages []uci.InstalledPackage
	if sshClient != nil {
		output, err := sshClient.Execute("opkg list-installed")
		if err == nil {
			installedPackages = parseInstalledPackages(output)
		}
	}

	// Generate package commands
	packageCommands := uci.GetPackageCommands(state.PackagesToInstall, state.PackagesToUninstall, installedPackages)
	commands = append(commands, packageCommands...)

	// Generate reset commands
	resetCommands := uci.GetResetCommands(state.ConfigSectionsToReset)
	commands = append(commands, resetCommands...)

	// Generate UCI commands
	uciCommands := uci.GenerateCommands(state.Config)
	commands = append(commands, uciCommands...)

	// Add commit and reload commands
	commands = append(commands, "uci commit")
	commands = append(commands, "reload_config")

	return commands, nil
}

func parseInstalledPackages(output string) []uci.InstalledPackage {
	var packages []uci.InstalledPackage

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		parts := strings.Split(line, " - ")
		if len(parts) == 2 {
			packages = append(packages, uci.InstalledPackage{
				Name:    parts[0],
				Version: parts[1],
			})
		}
	}

	return packages
}
