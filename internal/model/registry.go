package model

import (
	"fmt"
	"regexp"
	"time"
)

// RegistryType represents the type of container image registry.
type RegistryType string

const (
	DockerHub RegistryType = "dockerhub"
	GHCR      RegistryType = "ghcr"
	ACR       RegistryType = "acr"
	TCR       RegistryType = "tcr"
	Quay      RegistryType = "quay"
)

// Registry represents a container image registry connection.
type Registry struct {
	ID               int64        `json:"id"`
	Name             string       `json:"name"`
	URL              string       `json:"url"`
	Type             RegistryType `json:"type"`
	Username         string       `json:"username"`
	EncryptedPassword string      `json:"encrypted_password,omitempty"`
	Enabled          bool         `json:"enabled"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
}

// RegistryRepo represents a repository within a registry.
type RegistryRepo struct {
	ID          int64      `json:"id"`
	RegistryID  int64      `json:"registry_id"`
	Name        string     `json:"name"`
	TagCount    int        `json:"tag_count"`
	TotalSize   int64      `json:"total_size"`
	LastSynced  *time.Time `json:"last_synced,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// RegistryTag represents a single tag within a repository.
type RegistryTag struct {
	Name         string    `json:"name"`
	Digest       string    `json:"digest"`
	Size         int64     `json:"size"`
	CreatedAt    time.Time `json:"created_at"`
	MediaType    string    `json:"media_type"`
	Platform     string    `json:"platform,omitempty"`
	SchemaVersion int      `json:"schema_version"`
}

// ManifestDetail holds detailed manifest information for a tag.
type ManifestDetail struct {
	SchemaVersion int        `json:"schema_version"`
	MediaType     string     `json:"media_type"`
	Layers        []LayerInfo `json:"layers"`
	Config        struct {
		Digest string `json:"digest"`
	} `json:"config"`
	Platform struct {
		OS           string `json:"os"`
		Architecture string `json:"architecture"`
	} `json:"platform"`
}

// LayerInfo describes a single layer within a manifest.
type LayerInfo struct {
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	Command   string `json:"command,omitempty"`
}

// SyncTaskStatus represents the status of a sync task.
type SyncTaskStatus string

const (
	SyncTaskPending  SyncTaskStatus = "pending"
	SyncTaskRunning  SyncTaskStatus = "running"
	SyncTaskCompleted SyncTaskStatus = "completed"
	SyncTaskFailed   SyncTaskStatus = "failed"
)

// SyncTask represents a registry-to-registry image sync task.
type SyncTask struct {
	ID               int64          `json:"id"`
	SourceRegistryID  int64          `json:"source_registry_id"`
	TargetRegistryID  int64          `json:"target_registry_id"`
	SourceRepo       string         `json:"source_repo"`
	SourceTag        string         `json:"source_tag"`
	TargetRepo       string         `json:"target_repo"`
	TargetTag        string         `json:"target_tag"`
	Status           SyncTaskStatus `json:"status"`
	ProgressBytes    int64          `json:"progress_bytes"`
	TotalBytes       int64          `json:"total_bytes"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// RetentionPolicy defines rules for automatic cleanup of old tags.
type RetentionPolicy struct {
	ID              int64      `json:"id"`
	RegistryID      int64      `json:"registry_id"`
	RepoPattern     string     `json:"repo_pattern"`
	KeepDays        int        `json:"keep_days"`
	KeepCount       int        `json:"keep_count"`
	KeepPattern     string     `json:"keep_pattern,omitempty"`
	Enabled         bool       `json:"enabled"`
	LastExecutedAt  *time.Time `json:"last_executed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateRegistryRequest is the request body for creating a new registry connection.
type CreateRegistryRequest struct {
	Name     string       `json:"name"`
	URL      string       `json:"url"`
	Type     RegistryType `json:"type"`
	Username string       `json:"username"`
	Password string       `json:"password"`
}

// Validate checks that required fields are present.
func (r *CreateRegistryRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.URL == "" {
		return fmt.Errorf("url is required")
	}
	if r.Type == "" {
		return fmt.Errorf("type is required")
	}
	if r.Username == "" {
		return fmt.Errorf("username is required")
	}
	if r.Password == "" {
		return fmt.Errorf("password is required")
	}
	return nil
}

// UpdateRegistryRequest is the request body for updating a registry connection.
type UpdateRegistryRequest struct {
	ID       int64        `json:"id"`
	Name     string       `json:"name"`
	URL      string       `json:"url"`
	Type     RegistryType `json:"type"`
	Username string       `json:"username"`
	Password string       `json:"password,omitempty"`
}

// TestConnectionResponse holds the result of a registry connectivity test.
type TestConnectionResponse struct {
	Success      bool   `json:"success"`
	Version      string `json:"version,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	RegistryType string `json:"registry_type"`
}

// SyncRequest is the request body for initiating a registry sync task.
type SyncRequest struct {
	SourceRegistryID int64  `json:"source_registry_id"`
	TargetRegistryID int64  `json:"target_registry_id"`
	SourceRepo       string `json:"source_repo"`
	SourceTag        string `json:"source_tag"`
	TargetRepo       string `json:"target_repo"`
	TargetTag        string `json:"target_tag"`
	Platform         string `json:"platform,omitempty"`
}

// CreateRetentionPolicyRequest is the request body for creating a retention policy.
type CreateRetentionPolicyRequest struct {
	RegistryID  int64  `json:"registry_id"`
	RepoPattern string `json:"repo_pattern"`
	KeepDays    int    `json:"keep_days"`
	KeepCount   int    `json:"keep_count"`
	KeepPattern string `json:"keep_pattern,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// Validate checks that retention policy fields are valid.
func (r *CreateRetentionPolicyRequest) Validate() error {
	if r.KeepDays < 1 {
		return fmt.Errorf("keep_days must be >= 1")
	}
	if r.KeepCount < 1 {
		return fmt.Errorf("keep_count must be >= 1")
	}
	if r.RepoPattern == "" {
		return fmt.Errorf("repo_pattern is required")
	}
	if r.KeepPattern != "" {
		if _, err := regexp.Compile(r.KeepPattern); err != nil {
			return fmt.Errorf("keep_pattern: invalid regex: %w", err)
		}
	}
	return nil
}
