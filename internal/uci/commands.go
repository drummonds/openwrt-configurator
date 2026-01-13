package uci

import (
	"fmt"
	"reflect"
	"strconv"
)

// GenerateCommands generates UCI commands from OpenWrt config
func GenerateCommands(openWrtConfig map[string]any) []string {
	var commands []string

	for configKey, configValue := range openWrtConfig {
		configMap, ok := configValue.(map[string]any)
		if !ok {
			continue
		}

		for sectionKey, sectionValue := range configMap {
			sections, ok := sectionValue.([]any)
			if !ok {
				continue
			}

			for _, section := range sections {
				sectionMap, ok := section.(map[string]any)
				if !ok {
					continue
				}

				// Get section name
				sectionName, ok := sectionMap[".name"].(string)
				if !ok {
					continue
				}

				identifier := fmt.Sprintf("%s.%s", configKey, sectionName)

				// Create section
				commands = append(commands, fmt.Sprintf("uci set %s=%s", identifier, sectionKey))

				// Set all properties
				for key, value := range sectionMap {
					if key == ".name" {
						continue
					}

					commands = append(commands, generatePropertyCommands(identifier, key, value)...)
				}
			}
		}
	}

	return commands
}

func generatePropertyCommands(identifier, key string, value any) []string {
	var commands []string

	switch v := value.(type) {
	case []any:
		// Handle array values with add_list
		for _, item := range v {
			coerced := coerceValue(item)
			commands = append(commands, fmt.Sprintf("uci add_list %s.%s='%s'", identifier, key, coerced))
		}
	default:
		// Handle single values
		coerced := coerceValue(v)
		commands = append(commands, fmt.Sprintf("uci set %s.%s='%s'", identifier, key, coerced))
	}

	return commands
}

func coerceValue(value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return "1"
		}
		return "0"
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// GetResetCommands generates commands to reset config sections
func GetResetCommands(configSectionsToReset map[string][]string) []string {
	var commands []string

	for configKey, sectionKeys := range configSectionsToReset {
		for _, sectionKey := range sectionKeys {
			cmd := fmt.Sprintf("while uci -q delete %s.@%s[0]; do :; done", configKey, sectionKey)
			commands = append(commands, cmd)
		}
	}

	return commands
}

// GetPackageCommands generates opkg commands for package management
func GetPackageCommands(packagesToInstall []Package, packagesToUninstall []string, installedPackages []InstalledPackage) []string {
	var commands []string

	// Filter packages that are already installed/uninstalled
	var filteredInstall []Package
	var filteredUninstall []string

	if installedPackages != nil {
		// Filter packages to uninstall (only if currently installed)
		for _, pkg := range packagesToUninstall {
			isInstalled := false
			for _, installed := range installedPackages {
				if installed.Name == pkg {
					isInstalled = true
					break
				}
			}
			if isInstalled {
				filteredUninstall = append(filteredUninstall, pkg)
			}
		}

		// Filter packages to install (only if not currently installed)
		for _, pkg := range packagesToInstall {
			isInstalled := false
			for _, installed := range installedPackages {
				if installed.Name == pkg.Name {
					isInstalled = true
					break
				}
			}
			if !isInstalled {
				filteredInstall = append(filteredInstall, pkg)
			}
		}
	} else {
		filteredInstall = packagesToInstall
		filteredUninstall = packagesToUninstall
	}

	// Generate uninstall commands
	if len(filteredUninstall) > 0 {
		pkgList := ""
		for i, pkg := range filteredUninstall {
			if i > 0 {
				pkgList += " "
			}
			pkgList += pkg
		}
		commands = append(commands, fmt.Sprintf("opkg remove --force-removal-of-dependent-packages %s", pkgList))
	}

	// Generate install commands
	if len(filteredInstall) > 0 {
		commands = append(commands, "opkg update;")
		pkgList := ""
		for i, pkg := range filteredInstall {
			if i > 0 {
				pkgList += " "
			}
			pkgList += pkg.Name
		}
		commands = append(commands, fmt.Sprintf("opkg install %s", pkgList))
	}

	return commands
}

// Package represents a package to install
type Package struct {
	Name    string
	Version string
}

// InstalledPackage represents an installed package
type InstalledPackage struct {
	Name    string
	Version string
}

// ConvertToMap converts a struct to a map for UCI command generation
func ConvertToMap(v interface{}) (map[string]any, error) {
	data, err := marshalToMap(v)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func marshalToMap(v interface{}) (map[string]any, error) {
	result := make(map[string]any)
	val := reflect.ValueOf(v)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %v", val.Kind())
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := typ.Field(i)

		jsonTag := typeField.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse json tag
		tagName := jsonTag
		for i, c := range jsonTag {
			if c == ',' {
				tagName = jsonTag[:i]
				break
			}
		}

		if !field.IsZero() {
			result[tagName] = field.Interface()
		}
	}

	return result, nil
}
