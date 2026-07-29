package model

type OsInstallConfig struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ConfigName string `json:"config_name"`
	OsType     string `json:"os_type"`
	Config     string `json:"config"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// OsInstallParams defines all parameters for generating OS installation config files.
type OsInstallParams struct {
	Hostname         string   `json:"hostname"`
	IPAddress        string   `json:"ip_address"` // empty = DHCP
	Netmask          string   `json:"netmask"`    // e.g. "255.255.255.0"
	Gateway          string   `json:"gateway"`
	DNSServers       string   `json:"dns_servers"` // comma-separated
	RootPassword     string   `json:"root_password"`
	UserName         string   `json:"username"`
	UserPassword     string   `json:"user_password"`
	UserSSHKey       string   `json:"user_ssh_key"`     // public key content
	Timezone         string   `json:"timezone"`         // e.g. "Asia/Shanghai"
	Language         string   `json:"language"`         // e.g. "en_US"
	KeyboardLayout   string   `json:"keyboard_layout"`  // e.g. "us"
	Disk             string   `json:"disk"`             // e.g. "/dev/sda"
	PartitionScheme  string   `json:"partition_scheme"` // "whole_disk" or "manual"
	Packages         []string `json:"packages"`
	AdditionalConfig string   `json:"additional_config"` // raw text appended
}
