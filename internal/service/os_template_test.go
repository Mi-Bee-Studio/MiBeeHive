package service

import (
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// fullParams returns a model.OsInstallParams with all fields populated.
func fullParams() model.OsInstallParams {
	return model.OsInstallParams{
		Hostname:        "test-server",
		IPAddress:       "192.168.1.100",
		Netmask:         "255.255.255.0",
		Gateway:         "192.168.1.1",
		DNSServers:      "8.8.8.8, 8.8.4.4",
		RootPassword:    "rootpass123",
		UserName:        "admin",
		UserPassword:    "userpass123",
		UserSSHKey:      "ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com",
		Timezone:        "Asia/Shanghai",
		Language:        "en_US.UTF-8",
		KeyboardLayout:  "us",
		Disk:            "/dev/vda",
		PartitionScheme: "whole_disk",
		Packages:        []string{"vim", "curl", "htop"},
		AdditionalConfig: "echo 'custom setup'",
	}
}

// minimalParams returns params with only hostname set (everything else defaults).
func minimalParams() model.OsInstallParams {
	return model.OsInstallParams{
		Hostname: "minimal-host",
	}
}

// --- GeneratePreseed tests ---

func TestGeneratePreseed_FullParams(t *testing.T) {
	out, err := GeneratePreseed(fullParams())
	if err != nil {
		t.Fatalf("GeneratePreseed failed: %v", err)
	}

	// Check required directives.
	checks := []string{
		"d-i debian-installer/locale string en_US.UTF-8",
		"d-i keyboard-configuration/xkb-keymap select us",
		"d-i netcfg/disable_autoconfig boolean true",
		"d-i netcfg/get_ipaddress string 192.168.1.100",
		"d-i netcfg/get_netmask string 255.255.255.0",
		"d-i netcfg/get_gateway string 192.168.1.1",
		"d-i netcfg/get_nameservers string 8.8.8.8 8.8.4.4",
		"d-i netcfg/get_hostname string test-server",
		"d-i time/zone string Asia/Shanghai",
		"d-i partman-auto/disk string /dev/vda",
		"d-i passwd/root-password password rootpass123",
		"d-i passwd/username string admin",
		"d-i passwd/user-password password userpass123",
		"d-i pkgsel/include string vim curl htop",
		"in-target mkdir -p /home/admin/.ssh",
		"ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com",
		"echo 'custom setup'",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("preseed output missing: %q", c)
		}
	}
}

func TestGeneratePreseed_MinimalParams(t *testing.T) {
	out, err := GeneratePreseed(minimalParams())
	if err != nil {
		t.Fatalf("GeneratePreseed failed: %v", err)
	}

	// Defaults applied.
	if !strings.Contains(out, "d-i time/zone string UTC") {
		t.Error("expected default timezone UTC")
	}
	if !strings.Contains(out, "d-i debian-installer/locale string en_US") {
		t.Error("expected default language en_US")
	}
	if !strings.Contains(out, "d-i keyboard-configuration/xkb-keymap select us") {
		t.Error("expected default keyboard us")
	}
	if !strings.Contains(out, "d-i partman-auto/disk string /dev/sda") {
		t.Error("expected default disk /dev/sda")
	}
	if !strings.Contains(out, "d-i netcfg/get_hostname string minimal-host") {
		t.Error("expected hostname minimal-host")
	}

	// DHCP mode (no static IP).
	if strings.Contains(out, "d-i netcfg/disable_autoconfig boolean true") {
		t.Error("minimal params should use DHCP, not static")
	}
	if !strings.Contains(out, "d-i netcfg/disable_autoconfig boolean false") {
		t.Error("expected DHCP autoconfig")
	}

	// No root login when no root password.
	if !strings.Contains(out, "d-i passwd/root-login boolean false") {
		t.Error("expected root login disabled with empty root password")
	}

	// No SSH key section when empty.
	if strings.Contains(out, "preseed/late_command") {
		t.Error("minimal params should not have SSH key late_command")
	}
}

func TestGeneratePreseed_DHCP(t *testing.T) {
	p := fullParams()
	p.IPAddress = ""
	p.Netmask = ""
	p.Gateway = ""
	p.DNSServers = ""

	out, err := GeneratePreseed(p)
	if err != nil {
		t.Fatalf("GeneratePreseed DHCP failed: %v", err)
	}

	if strings.Contains(out, "d-i netcfg/disable_autoconfig boolean true") {
		t.Error("DHCP mode should not set static config")
	}
	if !strings.Contains(out, "d-i netcfg/disable_autoconfig boolean false") {
		t.Error("DHCP mode should enable autoconfig")
	}
}

func TestGeneratePreseed_ManualPartition(t *testing.T) {
	p := fullParams()
	p.PartitionScheme = "manual"

	out, err := GeneratePreseed(p)
	if err != nil {
		t.Fatalf("GeneratePreseed manual partition failed: %v", err)
	}

	if strings.Contains(out, "d-i partman-auto/choose_recipe select atomic") {
		t.Error("manual partition scheme should not use atomic recipe")
	}
	if !strings.Contains(out, "d-i partman-auto/disk string /dev/vda") {
		t.Error("should still reference the disk")
	}
}

func TestGeneratePreseed_NoHostname(t *testing.T) {
	p := fullParams()
	p.Hostname = ""

	_, err := GeneratePreseed(p)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
	if !strings.Contains(err.Error(), "hostname is required") {
		t.Errorf("error should mention hostname: %v", err)
	}
}

func TestGeneratePreseed_WhitespaceHostname(t *testing.T) {
	p := fullParams()
	p.Hostname = "   "

	_, err := GeneratePreseed(p)
	if err == nil {
		t.Fatal("expected error for whitespace-only hostname")
	}
}

// --- GenerateKickstart tests ---

func TestGenerateKickstart_FullParams(t *testing.T) {
	out, err := GenerateKickstart(fullParams(), "")
	if err != nil {
		t.Fatalf("GenerateKickstart failed: %v", err)
	}

	checks := []string{
		"lang en_US.UTF-8",
		"keyboard us",
		"network --bootproto=static",
		"--ip=192.168.1.100",
		"--netmask=255.255.255.0",
		"--gateway=192.168.1.1",
		"--nameserver=8.8.8.8 8.8.4.4",
		"--hostname=test-server",
		"rootpw --plaintext rootpass123",
		"timezone Asia/Shanghai --utc",
		"bootloader --location=mbr --driveorder=/dev/vda",
		"clearpart --all --initlabel --disklabel=gpt",
		"autopart --type=plain",
		"user --name=admin",
		"--password=userpass123",
		"vim",
		"curl",
		"htop",
		"echo \"ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com\" > /home/admin/.ssh/authorized_keys",
		"echo 'custom setup'",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("kickstart output missing: %q", c)
		}
	}
}

func TestGenerateKickstart_MinimalParams(t *testing.T) {
	out, err := GenerateKickstart(minimalParams(), "")
	if err != nil {
		t.Fatalf("GenerateKickstart failed: %v", err)
	}

	// Defaults.
	if !strings.Contains(out, "lang en_US") {
		t.Error("expected default language en_US")
	}
	if !strings.Contains(out, "keyboard us") {
		t.Error("expected default keyboard us")
	}
	if !strings.Contains(out, "timezone UTC --utc") {
		t.Error("expected default timezone UTC")
	}
	if !strings.Contains(out, "bootloader --location=mbr --driveorder=/dev/sda") {
		t.Error("expected default disk /dev/sda")
	}

	// DHCP mode.
	if !strings.Contains(out, "network --bootproto=dhcp") {
		t.Error("minimal params should use DHCP")
	}

	// Root password locked when empty.
	if !strings.Contains(out, "rootpw lock") {
		t.Error("expected root password locked with empty root password")
	}

	// Default packages (no user packages specified).
	if !strings.Contains(out, "@core") {
		t.Error("expected @core in default packages")
	}

	// No user section when username empty.
	if strings.Contains(out, "user --name=") {
		t.Error("minimal params should not create user when username empty")
	}
}

func TestGenerateKickstart_DHCP(t *testing.T) {
	p := fullParams()
	p.IPAddress = ""
	p.Netmask = ""
	p.Gateway = ""

	out, err := GenerateKickstart(p, "")
	if err != nil {
		t.Fatalf("GenerateKickstart DHCP failed: %v", err)
	}

	if strings.Contains(out, "--bootproto=static") {
		t.Error("DHCP mode should not use static")
	}
	if !strings.Contains(out, "--bootproto=dhcp") {
		t.Error("DHCP mode should use dhcp")
	}
}

func TestGenerateKickstart_ManualPartition(t *testing.T) {
	p := fullParams()
	p.PartitionScheme = "manual"

	out, err := GenerateKickstart(p, "")
	if err != nil {
		t.Fatalf("GenerateKickstart manual partition failed: %v", err)
	}

	if strings.Contains(out, "autopart --type=plain") {
		t.Error("manual partition should not use autopart")
	}
	if !strings.Contains(out, "part /boot") {
		t.Error("manual partition should define /boot")
	}
	if !strings.Contains(out, "part swap") {
		t.Error("manual partition should define swap")
	}
	if !strings.Contains(out, "part / --fstype=xfs --grow") {
		t.Error("manual partition should define root grow partition")
	}
}

func TestGenerateKickstart_NoHostname(t *testing.T) {
	p := fullParams()
	p.Hostname = ""

	_, err := GenerateKickstart(p, "")
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
}

// --- GenerateAutoinstall tests ---

func TestGenerateAutoinstall_FullParams(t *testing.T) {
	out, err := GenerateAutoinstall(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoinstall failed: %v", err)
	}

	checks := []string{
		"version: 1",
		"hostname: test-server",
		"username: admin",
		"password: \"userpass123\"",
		"version: 2",
		"addresses:",
		"- 192.168.1.100/24",
		"gateway4: 192.168.1.1",
		"- 8.8.8.8",
		"- 8.8.4.4",
		"locale: en_US.UTF-8",
		"layout: us",
		"timezone: Asia/Shanghai",
		"install-server: true",
		"authorized-keys:",
		"ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com",
		"name: direct",
		"- vim",
		"- curl",
		"- htop",
		"echo 'custom setup'",
		"late-commands:",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("autoinstall output missing: %q", c)
		}
	}
}

func TestGenerateAutoinstall_MinimalParams(t *testing.T) {
	out, err := GenerateAutoinstall(minimalParams())
	if err != nil {
		t.Fatalf("GenerateAutoinstall failed: %v", err)
	}

	// Defaults.
	if !strings.Contains(out, "version: 1") {
		t.Error("expected version: 1")
	}
	if !strings.Contains(out, "locale: en_US") {
		t.Error("expected default language en_US")
	}
	if !strings.Contains(out, "layout: us") {
		t.Error("expected default keyboard us")
	}
	if !strings.Contains(out, "timezone: UTC") {
		t.Error("expected default timezone UTC")
	}

	// DHCP mode.
	if !strings.Contains(out, "dhcp4: true") {
		t.Error("minimal params should use DHCP")
	}
	if strings.Contains(out, "addresses:") {
		t.Error("DHCP mode should not have static addresses")
	}

	// No SSH authorized-keys section when key is empty.
	if strings.Contains(out, "authorized-keys:") {
		t.Error("minimal params should not have authorized-keys")
	}

	// Storage default is direct (whole_disk).
	if !strings.Contains(out, "name: direct") {
		t.Error("expected default storage layout 'direct'")
	}
}

func TestGenerateAutoinstall_DHCP(t *testing.T) {
	p := fullParams()
	p.IPAddress = ""
	p.Netmask = ""
	p.Gateway = ""
	p.DNSServers = ""

	out, err := GenerateAutoinstall(p)
	if err != nil {
		t.Fatalf("GenerateAutoinstall DHCP failed: %v", err)
	}

	if !strings.Contains(out, "dhcp4: true") {
		t.Error("DHCP mode should set dhcp4: true")
	}
	if strings.Contains(out, "gateway4:") {
		t.Error("DHCP mode should not have gateway4")
	}
	if strings.Contains(out, "addresses:") {
		t.Error("DHCP mode should not have static addresses")
	}
}

func TestGenerateAutoinstall_ManualPartition(t *testing.T) {
	p := fullParams()
	p.PartitionScheme = "manual"

	out, err := GenerateAutoinstall(p)
	if err != nil {
		t.Fatalf("GenerateAutoinstall manual partition failed: %v", err)
	}

	if !strings.Contains(out, "name: lvm") {
		t.Error("manual partition should use lvm layout")
	}
	if strings.Contains(out, "name: direct") {
		t.Error("manual partition should not use direct layout")
	}
}

func TestGenerateAutoinstall_NoHostname(t *testing.T) {
	p := fullParams()
	p.Hostname = ""

	_, err := GenerateAutoinstall(p)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
}

func TestGenerateAutoinstall_NetmaskCIDR(t *testing.T) {
	tests := []struct {
		mask string
		want string
	}{
		{"255.255.255.0", "24"},
		{"255.255.0.0", "16"},
		{"255.255.255.128", "25"},
		{"255.255.255.252", "30"},
		{"", "24"}, // empty mask defaults to /24 via netmaskToCIDR fallback
	}
	for _, tt := range tests {
		got := netmaskToCIDR(tt.mask)
		if got != tt.want {
			t.Errorf("netmaskToCIDR(%q) = %q, want %q", tt.mask, got, tt.want)
		}
	}
}

// --- Shared utility tests ---

func TestApplyDefaults(t *testing.T) {
	p := model.OsInstallParams{
		Hostname: "test",
	}
	applyDefaults(&p)

	if p.Timezone != defaultTimezone {
		t.Errorf("timezone = %q, want %q", p.Timezone, defaultTimezone)
	}
	if p.Language != defaultLanguage {
		t.Errorf("language = %q, want %q", p.Language, defaultLanguage)
	}
	if p.KeyboardLayout != defaultKeyboardLayout {
		t.Errorf("keyboard = %q, want %q", p.KeyboardLayout, defaultKeyboardLayout)
	}
	if p.Disk != defaultDisk {
		t.Errorf("disk = %q, want %q", p.Disk, defaultDisk)
	}
	if p.PartitionScheme != defaultPartitionScheme {
		t.Errorf("partition = %q, want %q", p.PartitionScheme, defaultPartitionScheme)
	}
	// Netmask default only applies when IP is set.
	if p.Netmask != "" {
		t.Errorf("netmask should be empty when IP is empty, got %q", p.Netmask)
	}

	// With IP set, netmask should get default.
	p2 := model.OsInstallParams{Hostname: "test", IPAddress: "10.0.0.1"}
	applyDefaults(&p2)
	if p2.Netmask != defaultNetmask {
		t.Errorf("netmask with IP = %q, want %q", p2.Netmask, defaultNetmask)
	}
}

func TestApplyDefaults_NoOverride(t *testing.T) {
	p := fullParams()
	applyDefaults(&p)

	// All explicitly set values should be preserved.
	if p.Timezone != "Asia/Shanghai" {
		t.Error("applyDefaults should not override existing timezone")
	}
	if p.Disk != "/dev/vda" {
		t.Error("applyDefaults should not override existing disk")
	}
}

func TestValidateParams(t *testing.T) {
	err := validateParams(model.OsInstallParams{Hostname: "valid"})
	if err != nil {
		t.Errorf("validateParams with valid hostname failed: %v", err)
	}

	err = validateParams(model.OsInstallParams{Hostname: ""})
	if err == nil {
		t.Error("validateParams should reject empty hostname")
	}

	err = validateParams(model.OsInstallParams{Hostname: "  "})
	if err == nil {
		t.Error("validateParams should reject whitespace hostname")
	}
}

func TestFormatDNSServers(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"8.8.8.8, 8.8.4.4", "8.8.8.8 8.8.4.4"},
		{"8.8.8.8,8.8.4.4", "8.8.8.8 8.8.4.4"},
		{"8.8.8.8", "8.8.8.8"},
		{"", ""},
		{"  8.8.8.8  ,  8.8.4.4  ", "8.8.8.8 8.8.4.4"},
	}
	for _, tt := range tests {
		got := formatDNSServers(tt.input)
		if got != tt.want {
			t.Errorf("formatDNSServers(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatPackages(t *testing.T) {
	got := formatPackages([]string{"vim", "curl", "htop"})
	want := "vim curl htop"
	if got != want {
		t.Errorf("formatPackages = %q, want %q", got, want)
	}

	got = formatPackages(nil)
	if got != "" {
		t.Errorf("formatPackages(nil) = %q, want empty", got)
	}
}

func TestCountBits(t *testing.T) {
	tests := []struct {
		n    int
		want int
	}{
		{0b11111111, 8},
		{0b11110000, 4},
		{0b00000000, 0},
		{0b10101010, 4},
	}
	for _, tt := range tests {
		got := countBits(tt.n)
		if got != tt.want {
			t.Errorf("countBits(%d) = %d, want %d", tt.n, got, tt.want)
		}
	}
}

// --- Output non-empty tests ---

func TestGeneratePreseed_OutputNotEmpty(t *testing.T) {
	out, err := GeneratePreseed(fullParams())
	if err != nil {
		t.Fatalf("GeneratePreseed failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("preseed output is empty")
	}
}

func TestGenerateKickstart_OutputNotEmpty(t *testing.T) {
	out, err := GenerateKickstart(fullParams(), "")
	if err != nil {
		t.Fatalf("GenerateKickstart failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("kickstart output is empty")
	}
}

func TestGenerateAutoinstall_OutputNotEmpty(t *testing.T) {
	out, err := GenerateAutoinstall(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoinstall failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("autoinstall output is empty")
	}
}

// --- Generate dispatch tests (via service.Generate) ---

func TestGenerate_RockyLinux(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("rocky", fullParams())
	if err != nil {
		t.Fatalf("Generate rocky failed: %v", err)
	}
	if !strings.Contains(out, "lang en_US.UTF-8") {
		t.Error("rocky output should contain language setting")
	}
	if !strings.Contains(out, "network --bootproto=static") {
		t.Error("rocky output should contain network config")
	}
	if !strings.Contains(out, "rootpw --plaintext") {
		t.Error("rocky output should contain root password")
	}
	if !strings.Contains(out, "download.rockylinux.org") {
		t.Error("rocky kickstart should use rockylinux.org mirror URL")
	}
}

func TestGenerate_AlmaLinux(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("alma", fullParams())
	if err != nil {
		t.Fatalf("Generate alma failed: %v", err)
	}
	if !strings.Contains(out, "lang en_US.UTF-8") {
		t.Error("alma output should contain language setting")
	}
	if !strings.Contains(out, "repo.almalinux.org") {
		t.Error("alma kickstart should use almalinux.org mirror URL")
	}
}

func TestGenerate_Fedora(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("fedora", fullParams())
	if err != nil {
		t.Fatalf("Generate fedora failed: %v", err)
	}
	if !strings.Contains(out, "lang en_US.UTF-8") {
		t.Error("fedora output should contain language setting")
	}
	if !strings.Contains(out, "fedoraproject.org") {
		t.Error("fedora kickstart should use fedoraproject.org mirror URL")
	}
}

func TestGenerate_RockyLinux_MirrorURL(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("rocky", fullParams())
	if err != nil {
		t.Fatalf("Generate rocky failed: %v", err)
	}
	if !strings.Contains(out, "download.rockylinux.org") {
		t.Error("rocky kickstart should use rockylinux.org mirror URL")
	}
}

func TestGenerate_AlmaLinux_MirrorURL(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("alma", fullParams())
	if err != nil {
		t.Fatalf("Generate alma failed: %v", err)
	}
	if !strings.Contains(out, "repo.almalinux.org") {
		t.Error("alma kickstart should use almalinux.org mirror URL")
	}
}

func TestGenerate_Fedora_MirrorURL(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("fedora", fullParams())
	if err != nil {
		t.Fatalf("Generate fedora failed: %v", err)
	}
	if !strings.Contains(out, "fedoraproject.org") {
		t.Error("fedora kickstart should use fedoraproject.org mirror URL")
	}
}

func TestGenerate_Kickstart_CentOS_MirrorURL(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("centos", fullParams())
	if err != nil {
		t.Fatalf("Generate centos failed: %v", err)
	}
	if !strings.Contains(out, "mirror.centos.org") {
		t.Error("centos kickstart should use mirror.centos.org URL")
	}

	// RHEL should also use the same default URL.
	out, err = svc.Generate("rhel", fullParams())
	if err != nil {
		t.Fatalf("Generate rhel failed: %v", err)
	}
	if !strings.Contains(out, "mirror.centos.org") {
		t.Error("rhel kickstart should use mirror.centos.org URL")
	}
}

func TestGenerate_openSUSE(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("opensuse", fullParams())
	if err != nil {
		t.Fatalf("Generate opensuse failed: %v", err)
	}
	if !strings.Contains(out, "<profile") {
		t.Error("opensuse output should contain AutoYAST profile")
	}
}

func TestGenerate_openSUSELeap(t *testing.T) {
	svc := NewOsTemplateService()
	out, err := svc.Generate("opensuse-leap", fullParams())
	if err != nil {
		t.Fatalf("Generate opensuse-leap failed: %v", err)
	}
	if !strings.Contains(out, "<profile") {
		t.Error("opensuse-leap output should contain AutoYAST profile")
	}
}

// --- GenerateAutoYAST comprehensive tests ---

func TestGenerateAutoYAST_FullParams(t *testing.T) {
	out, err := GenerateAutoYAST(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST failed: %v", err)
	}

	checks := []string{
		"<?xml version=\"1.0\"?>",
		"<!-- AutoYAST Profile — generated by MiBeeHive -->",
		"<profile xmlns=\"http://www.suse.com/1.0/yast2ns\" xmlns:config=\"http://www.suse.com/1.0/configns\">",
		"<confirm config:type=\"boolean\">false</confirm>",
		"<language>en_US.UTF-8</language>",
		"<keymap>us</keymap>",
		"<timezone>Asia/Shanghai</timezone>",
		"<bootproto>static</bootproto>",
		"<ipaddr>192.168.1.100</ipaddr>",
		"<netmask>255.255.255.0</netmask>",
		"<nameserver>8.8.8.8</nameserver>",
		"<nameserver>8.8.4.4</nameserver>",
		"<gateway>192.168.1.1</gateway>",
		"<device>/dev/vda</device>",
		"<use>all</use>",
		"<username>root</username>",
		"<user_password>rootpass123</user_password>",
		"<username>admin</username>",
		"<user_password>userpass123</user_password>",
		"<home>/home/admin</home>",
		"<pattern>base</pattern>",
		"<package>vim</package>",
		"<package>curl</package>",
		"<package>htop</package>",
		"<filename>setup-ssh.sh</filename>",
		"ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com",
		"</profile>",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("autoyast output missing: %q", c)
		}
	}
}

func TestGenerateAutoYAST_MinimalParams(t *testing.T) {
	out, err := GenerateAutoYAST(minimalParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST failed: %v", err)
	}

	// Defaults applied.
	if !strings.Contains(out, "<language>en_US</language>") {
		t.Error("expected default language en_US")
	}
	if !strings.Contains(out, "<keymap>us</keymap>") {
		t.Error("expected default keyboard us")
	}
	if !strings.Contains(out, "<timezone>UTC</timezone>") {
		t.Error("expected default timezone UTC")
	}
	if !strings.Contains(out, "<device>/dev/sda</device>") {
		t.Error("expected default disk /dev/sda")
	}

	// DHCP mode.
	if !strings.Contains(out, "<bootproto>dhcp</bootproto>") {
		t.Error("minimal params should use DHCP")
	}
	if strings.Contains(out, "<ipaddr>") {
		t.Error("DHCP mode should not have ipaddr")
	}

	// No root user when root password empty.
	if strings.Contains(out, "<username>root</username>") {
		t.Error("minimal params should not have root user when no root password")
	}

	// No SSH key script when empty.
	if strings.Contains(out, "<scripts>") {
		t.Error("minimal params should not have scripts section")
	}
}

func TestGenerateAutoYAST_DHCP(t *testing.T) {
	p := fullParams()
	p.IPAddress = ""
	p.Netmask = ""
	p.Gateway = ""
	p.DNSServers = ""

	out, err := GenerateAutoYAST(p)
	if err != nil {
		t.Fatalf("GenerateAutoYAST DHCP failed: %v", err)
	}

	if !strings.Contains(out, "<bootproto>dhcp</bootproto>") {
		t.Error("DHCP mode should use dhcp bootproto")
	}
	if strings.Contains(out, "<bootproto>static</bootproto>") {
		t.Error("DHCP mode should not use static bootproto")
	}
	if strings.Contains(out, "<ipaddr>") {
		t.Error("DHCP mode should not have ipaddr")
	}
	if strings.Contains(out, "<dns>") {
		t.Error("DHCP mode should not have dns section")
	}
	if strings.Contains(out, "<routing>") {
		t.Error("DHCP mode should not have routing section")
	}
}

func TestGenerateAutoYAST_StaticIP(t *testing.T) {
	out, err := GenerateAutoYAST(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST static failed: %v", err)
	}

	if !strings.Contains(out, "<bootproto>static</bootproto>") {
		t.Error("static mode should use static bootproto")
	}
	if !strings.Contains(out, "<ipaddr>192.168.1.100</ipaddr>") {
		t.Error("static mode should have ipaddr")
	}
	if !strings.Contains(out, "<netmask>255.255.255.0</netmask>") {
		t.Error("static mode should have netmask")
	}
	if !strings.Contains(out, "<dns>") {
		t.Error("static mode should have dns section")
	}
	if !strings.Contains(out, "<routing>") {
		t.Error("static mode should have routing section")
	}
	if !strings.Contains(out, "<gateway>192.168.1.1</gateway>") {
		t.Error("static mode should have gateway")
	}
}

func TestGenerateAutoYAST_Users(t *testing.T) {
	out, err := GenerateAutoYAST(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST users failed: %v", err)
	}

	// Root user.
	if !strings.Contains(out, "<username>root</username>") {
		t.Error("expected root user section")
	}
	if !strings.Contains(out, "<user_password>rootpass123</user_password>") {
		t.Error("expected root password")
	}

	// Regular user.
	if !strings.Contains(out, "<username>admin</username>") {
		t.Error("expected admin user section")
	}
	if !strings.Contains(out, "<user_password>userpass123</user_password>") {
		t.Error("expected user password")
	}
	if !strings.Contains(out, "<home>/home/admin</home>") {
		t.Error("expected user home directory")
	}
}

func TestGenerateAutoYAST_SSHKey(t *testing.T) {
	out, err := GenerateAutoYAST(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST SSH key failed: %v", err)
	}

	if !strings.Contains(out, "<scripts>") {
		t.Error("expected scripts section for SSH key")
	}
	if !strings.Contains(out, "<filename>setup-ssh.sh</filename>") {
		t.Error("expected setup-ssh.sh script")
	}
	if !strings.Contains(out, "mkdir -p /home/admin/.ssh") {
		t.Error("expected SSH key setup command")
	}
	if !strings.Contains(out, "ssh-rsa AAAAB3NzaC1yc2EAAAA test@example.com") {
		t.Error("expected SSH key in output")
	}
	if !strings.Contains(out, "chown -R admin:users") {
		t.Error("expected chown with users group for openSUSE")
	}
}

func TestGenerateAutoYAST_Packages(t *testing.T) {
	out, err := GenerateAutoYAST(fullParams())
	if err != nil {
		t.Fatalf("GenerateAutoYAST packages failed: %v", err)
	}

	if !strings.Contains(out, "<pattern>base</pattern>") {
		t.Error("expected base pattern")
	}
	if !strings.Contains(out, "<package>vim</package>") {
		t.Error("expected vim package")
	}
	if !strings.Contains(out, "<package>curl</package>") {
		t.Error("expected curl package")
	}
	if !strings.Contains(out, "<package>htop</package>") {
		t.Error("expected htop package")
	}
}

func TestGenerateAutoYAST_NoHostname(t *testing.T) {
	p := fullParams()
	p.Hostname = ""

	_, err := GenerateAutoYAST(p)
	if err == nil {
		t.Fatal("expected error for empty hostname")
	}
	if !strings.Contains(err.Error(), "hostname is required") {
		t.Errorf("error should mention hostname: %v", err)
	}
}
