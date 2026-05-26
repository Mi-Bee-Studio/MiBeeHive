package model

import (
	"testing"
)

func TestCreateRegistryRequest_ValidateValid(t *testing.T) {
	req := CreateRegistryRequest{
		Name:     "my-registry",
		URL:      "https://registry.example.com",
		Type:     DockerHub,
		Username: "admin",
		Password: "secret",
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCreateRegistryRequest_ValidateMissingName(t *testing.T) {
	req := CreateRegistryRequest{
		URL:      "https://registry.example.com",
		Type:     DockerHub,
		Username: "admin",
		Password: "secret",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing name, got nil")
	}
}

func TestCreateRegistryRequest_ValidateMissingURL(t *testing.T) {
	req := CreateRegistryRequest{
		Name:     "my-registry",
		Type:     DockerHub,
		Username: "admin",
		Password: "secret",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing url, got nil")
	}
}

func TestCreateRegistryRequest_ValidateMissingType(t *testing.T) {
	req := CreateRegistryRequest{
		Name:     "my-registry",
		URL:      "https://registry.example.com",
		Username: "admin",
		Password: "secret",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing type, got nil")
	}
}

func TestCreateRegistryRequest_ValidateMissingUsername(t *testing.T) {
	req := CreateRegistryRequest{
		Name:     "my-registry",
		URL:      "https://registry.example.com",
		Type:     DockerHub,
		Password: "secret",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing username, got nil")
	}
}

func TestCreateRegistryRequest_ValidateMissingPassword(t *testing.T) {
	req := CreateRegistryRequest{
		Name:     "my-registry",
		URL:      "https://registry.example.com",
		Type:     DockerHub,
		Username: "admin",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing password, got nil")
	}
}

func TestCreateRetentionPolicyRequest_ValidateValid(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID:  1,
		RepoPattern: "library/*",
		KeepDays:    30,
		KeepCount:   5,
		Enabled:     true,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCreateRetentionPolicyRequest_ValidateKeepDaysTooLow(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID:  1,
		RepoPattern: "library/*",
		KeepDays:    0,
		KeepCount:   5,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for keep_days < 1, got nil")
	}
}

func TestCreateRetentionPolicyRequest_ValidateKeepCountTooLow(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID:  1,
		RepoPattern: "library/*",
		KeepDays:    30,
		KeepCount:   0,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for keep_count < 1, got nil")
	}
}

func TestCreateRetentionPolicyRequest_ValidateInvalidRegex(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID:  1,
		RepoPattern: "library/*",
		KeepDays:    30,
		KeepCount:   5,
		KeepPattern: "[invalid",
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for invalid regex, got nil")
	}
}

func TestCreateRetentionPolicyRequest_ValidateValidRegex(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID:  1,
		RepoPattern: "library/*",
		KeepDays:    30,
		KeepCount:   5,
		KeepPattern: "^v[0-9]+\\.[0-9]+\\.[0-9]+$",
		Enabled:     true,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected no error for valid regex, got: %v", err)
	}
}

func TestCreateRetentionPolicyRequest_ValidateMissingRepoPattern(t *testing.T) {
	req := CreateRetentionPolicyRequest{
		RegistryID: 1,
		KeepDays:   30,
		KeepCount:  5,
	}
	if err := req.Validate(); err == nil {
		t.Error("expected error for missing repo_pattern, got nil")
	}
}
