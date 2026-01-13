package config

import "encoding/json"

// ONCConfig represents the root configuration structure
type ONCConfig struct {
	Devices           []DeviceConfig      `json:"devices"`
	PackageProfiles   []PackageProfile    `json:"package_profiles,omitempty"`
	ConfigsToNotReset []ConfigsToNotReset `json:"configs_to_not_reset,omitempty"`
	Config            ConfigConfig        `json:"config"`
}

// DeviceConfig represents a single device configuration
type DeviceConfig struct {
	Enabled            *bool               `json:"enabled,omitempty"`
	ModelID            string              `json:"model_id"`
	IPAddr             string              `json:"ipaddr"`
	Hostname           string              `json:"hostname"`
	Tags               map[string]any      `json:"tags"`
	ProvisioningConfig *ProvisioningConfig `json:"provisioning_config,omitempty"`
}

// ProvisioningConfig contains SSH authentication details
type ProvisioningConfig struct {
	SSHAuth SSHAuth `json:"ssh_auth"`
}

// SSHAuth contains SSH credentials
type SSHAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PackageProfile defines packages to install/uninstall based on conditions
type PackageProfile struct {
	If       *string  `json:".if,omitempty"`
	Packages []string `json:"packages"`
}

// ConfigsToNotReset defines configs that should not be reset
type ConfigsToNotReset struct {
	If      *string  `json:".if,omitempty"`
	Configs []string `json:"configs"`
}

// ConfigConfig contains all UCI configuration sections
type ConfigConfig struct {
	System   *SystemConfig   `json:"system,omitempty"`
	Network  *NetworkConfig  `json:"network,omitempty"`
	Firewall *FirewallConfig `json:"firewall,omitempty"`
	DHCP     *DHCPConfig     `json:"dhcp,omitempty"`
	Wireless *WirelessConfig `json:"wireless,omitempty"`
	Dropbear *DropbearConfig `json:"dropbear,omitempty"`

	// Support for additional configs
	Extra map[string]any `json:"-"`
}

// UnmarshalJSON custom unmarshaler to handle extra fields
func (c *ConfigConfig) UnmarshalJSON(data []byte) error {
	type Alias ConfigConfig
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(c),
	}

	// First unmarshal into a map to capture all fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// Then unmarshal into the struct
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Store any extra fields
	c.Extra = make(map[string]any)
	knownFields := map[string]bool{
		"system": true, "network": true, "firewall": true,
		"dhcp": true, "wireless": true, "dropbear": true,
	}

	for key, val := range raw {
		if !knownFields[key] {
			var v any
			json.Unmarshal(val, &v)
			c.Extra[key] = v
		}
	}

	return nil
}

// SystemConfig contains system configuration
type SystemConfig struct {
	If        *string         `json:".if,omitempty"`
	Overrides []Override      `json:".overrides,omitempty"`
	System    []SystemSection `json:"system,omitempty"`
}

// SystemSection represents a system configuration section
type SystemSection struct {
	Name     *string `json:".name,omitempty"`
	Hostname *string `json:"hostname,omitempty"`
	Timezone *string `json:"timezone,omitempty"`
	Zonename *string `json:"zonename,omitempty"`
}

// NetworkConfig contains network configuration
type NetworkConfig struct {
	If         *string             `json:".if,omitempty"`
	Overrides  []Override          `json:".overrides,omitempty"`
	Interface  []InterfaceSection  `json:"interface,omitempty"`
	Device     []DeviceSection     `json:"device,omitempty"`
	Switch     []SwitchSection     `json:"switch,omitempty"`
	SwitchVlan []SwitchVlanSection `json:"switch_vlan,omitempty"`
	BridgeVlan []BridgeVlanSection `json:"bridge-vlan,omitempty"`
}

// InterfaceSection represents a network interface
type InterfaceSection struct {
	Name      *string    `json:".name,omitempty"`
	If        *string    `json:".if,omitempty"`
	Overrides []Override `json:".overrides,omitempty"`
	Device    *string    `json:"device,omitempty"`
	Proto     *string    `json:"proto,omitempty"`
	IPAddr    *string    `json:"ipaddr,omitempty"`
	Netmask   *string    `json:"netmask,omitempty"`
	Gateway   *string    `json:"gateway,omitempty"`
	DNS       []string   `json:"dns,omitempty"`
	Username  *string    `json:"username,omitempty"`
	Password  *string    `json:"password,omitempty"`

	// Support for additional fields
	Extra map[string]any `json:"-"`
}

// DeviceSection represents a network device
type DeviceSection struct {
	Name       *string    `json:".name,omitempty"`
	If         *string    `json:".if,omitempty"`
	Overrides  []Override `json:".overrides,omitempty"`
	DeviceName *string    `json:"name,omitempty"`
	Type       *string    `json:"type,omitempty"`
	Ports      []string   `json:"ports,omitempty"`

	// Support for additional fields
	Extra map[string]any `json:"-"`
}

// SwitchSection represents a switch configuration
type SwitchSection struct {
	Name       *string `json:".name,omitempty"`
	SwitchName *string `json:"name,omitempty"`
	Reset      *bool   `json:"reset,omitempty"`
	EnableVlan *bool   `json:"enable_vlan,omitempty"`
}

// SwitchVlanSection represents a switch VLAN
type SwitchVlanSection struct {
	Name   *string `json:".name,omitempty"`
	Device *string `json:"device,omitempty"`
	Vlan   *int    `json:"vlan,omitempty"`
	Ports  *string `json:"ports,omitempty"`
}

// BridgeVlanSection represents a bridge VLAN
type BridgeVlanSection struct {
	Name   *string  `json:".name,omitempty"`
	Device *string  `json:"device,omitempty"`
	Vlan   *int     `json:"vlan,omitempty"`
	Ports  []string `json:"ports,omitempty"`
}

// FirewallConfig contains firewall configuration
type FirewallConfig struct {
	If         *string             `json:".if,omitempty"`
	Overrides  []Override          `json:".overrides,omitempty"`
	Defaults   []DefaultSection    `json:"defaults,omitempty"`
	Zone       []ZoneSection       `json:"zone,omitempty"`
	Forwarding []ForwardingSection `json:"forwarding,omitempty"`
	Rule       []RuleSection       `json:"rule,omitempty"`
}

// DefaultSection represents firewall defaults
type DefaultSection struct {
	Name        *string `json:".name,omitempty"`
	Input       *string `json:"input,omitempty"`
	Output      *string `json:"output,omitempty"`
	Forward     *string `json:"forward,omitempty"`
	SynFlood    *bool   `json:"syn_flood,omitempty"`
	DropInvalid *bool   `json:"drop_invalid,omitempty"`
}

// ZoneSection represents a firewall zone
type ZoneSection struct {
	Name     *string  `json:".name,omitempty"`
	ZoneName *string  `json:"name,omitempty"`
	Network  []string `json:"network,omitempty"`
	Input    *string  `json:"input,omitempty"`
	Output   *string  `json:"output,omitempty"`
	Forward  *string  `json:"forward,omitempty"`
	Masq     *bool    `json:"masq,omitempty"`
	MtuFix   *bool    `json:"mtu_fix,omitempty"`
}

// ForwardingSection represents a firewall forwarding rule
type ForwardingSection struct {
	Name *string `json:".name,omitempty"`
	Src  *string `json:"src,omitempty"`
	Dest *string `json:"dest,omitempty"`
}

// RuleSection represents a firewall rule
type RuleSection struct {
	Name     *string `json:".name,omitempty"`
	Src      *string `json:"src,omitempty"`
	Dest     *string `json:"dest,omitempty"`
	Proto    *string `json:"proto,omitempty"`
	DestPort *string `json:"dest_port,omitempty"`
	Target   *string `json:"target,omitempty"`
	Family   *string `json:"family,omitempty"`
}

// DHCPConfig contains DHCP configuration
type DHCPConfig struct {
	If        *string          `json:".if,omitempty"`
	Overrides []Override       `json:".overrides,omitempty"`
	Dnsmasq   []DnsmasqSection `json:"dnsmasq,omitempty"`
	DHCP      []DHCPSection    `json:"dhcp,omitempty"`
	Odhcpd    []OdhcpdSection  `json:"odhcpd,omitempty"`
}

// DnsmasqSection represents dnsmasq configuration
type DnsmasqSection struct {
	Name         *string `json:".name,omitempty"`
	DomainNeeded *bool   `json:"domainneeded,omitempty"`
	Boguspriv    *bool   `json:"boguspriv,omitempty"`
	LocalService *bool   `json:"localservice,omitempty"`
}

// DHCPSection represents a DHCP configuration
type DHCPSection struct {
	Name       *string  `json:".name,omitempty"`
	Interface  *string  `json:"interface,omitempty"`
	Start      *int     `json:"start,omitempty"`
	Limit      *int     `json:"limit,omitempty"`
	Leasetime  *string  `json:"leasetime,omitempty"`
	DHCPOption []string `json:"dhcp_option,omitempty"`
}

// OdhcpdSection represents odhcpd configuration
type OdhcpdSection struct {
	Name         *string `json:".name,omitempty"`
	Maindhcp     *bool   `json:"maindhcp,omitempty"`
	Leasefile    *string `json:"leasefile,omitempty"`
	Leasetrigger *string `json:"leasetrigger,omitempty"`
}

// WirelessConfig contains wireless configuration
type WirelessConfig struct {
	If         *string             `json:".if,omitempty"`
	Overrides  []Override          `json:".overrides,omitempty"`
	WifiDevice []WifiDeviceSection `json:"wifi-device,omitempty"`
	WifiIface  []WifiIfaceSection  `json:"wifi-iface,omitempty"`
}

// WifiDeviceSection represents a WiFi device (radio)
type WifiDeviceSection struct {
	Name     *string `json:".name,omitempty"`
	Type     *string `json:"type,omitempty"`
	Band     *string `json:"band,omitempty"`
	Channel  *string `json:"channel,omitempty"`
	Htmode   *string `json:"htmode,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
}

// WifiIfaceSection represents a WiFi interface
type WifiIfaceSection struct {
	Name       *string `json:".name,omitempty"`
	Device     any     `json:"device,omitempty"` // Can be string or []string
	Mode       *string `json:"mode,omitempty"`
	Network    *string `json:"network,omitempty"`
	SSID       *string `json:"ssid,omitempty"`
	Encryption *string `json:"encryption,omitempty"`
	Key        *string `json:"key,omitempty"`
	Disabled   *bool   `json:"disabled,omitempty"`
}

// DropbearConfig contains dropbear SSH configuration
type DropbearConfig struct {
	If        *string           `json:".if,omitempty"`
	Overrides []Override        `json:".overrides,omitempty"`
	Dropbear  []DropbearSection `json:"dropbear,omitempty"`
}

// DropbearSection represents dropbear configuration
type DropbearSection struct {
	Name             *string `json:".name,omitempty"`
	PasswordAuth     *string `json:"PasswordAuth,omitempty"`
	RootPasswordAuth *string `json:"RootPasswordAuth,omitempty"`
	Port             *int    `json:"Port,omitempty"`
	BannerFile       *string `json:"BannerFile,omitempty"`
}

// Override represents a conditional override
type Override struct {
	If       string         `json:".if"`
	Override map[string]any `json:"override"`
}
