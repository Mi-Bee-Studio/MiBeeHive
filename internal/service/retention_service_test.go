package service

import (
	"context"
	"database/sql"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	_ "modernc.org/sqlite"
)

// ---- Validation Tests ----

func TestValidatePolicyRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *model.CreateRetentionPolicyRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid time-based policy",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				KeepDays:    30,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "valid count-based policy",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				KeepCount:   5,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "valid regex pattern policy",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				KeepPattern: `^v\d+\.\d+\.\d+$`,
				Enabled:     true,
			},
			wantErr: false,
		},
		{
			name: "no rules set",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				Enabled:     true,
			},
			wantErr: true,
			errMsg:  "at least one retention rule must be set",
		},
		{
			name: "empty repo pattern",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID: 1,
				KeepDays:   30,
				Enabled:    true,
			},
			wantErr: true,
			errMsg:  "repo_pattern is required",
		},
		{
			name: "invalid regex pattern",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				KeepPattern: `[invalid`,
				Enabled:     true,
			},
			wantErr: true,
			errMsg:  "invalid regex",
		},
		{
			name: "combined rules valid",
			req: &model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "myapp/*",
				KeepDays:    30,
				KeepCount:   10,
				KeepPattern: `^v`,
				Enabled:     true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePolicyRequest(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

// ---- Tag Selection Tests ----

func TestSelectTagsForDeletion_TimeBased(t *testing.T) {
	svc := &RetentionService{}
	now := time.Now()

	policy := &model.RetentionPolicy{KeepDays: 30}

	tags := []tagInfo{
		{name: "v1.0", digest: "sha256:a1", createdAt: now.AddDate(0, 0, -60)},
		{name: "v1.1", digest: "sha256:a2", createdAt: now.AddDate(0, 0, -10)},
		{name: "v1.2", digest: "sha256:a3", createdAt: now.AddDate(0, 0, -45)},
	}

	toDelete := svc.selectTagsForDeletion(policy, tags)
	if len(toDelete) != 2 {
		t.Fatalf("expected 2 tags to delete, got %d", len(toDelete))
	}

	names := tagNames(toDelete)
	if !containsString(names, "v1.0") || !containsString(names, "v1.2") {
		t.Errorf("expected v1.0 and v1.2 to be deleted, got %v", names)
	}
	if containsString(names, "v1.1") {
		t.Error("v1.1 should not be deleted")
	}
}

func TestSelectTagsForDeletion_CountBased(t *testing.T) {
	svc := &RetentionService{}
	now := time.Now()

	policy := &model.RetentionPolicy{KeepCount: 2}

	tags := []tagInfo{
		{name: "v1.0", digest: "sha256:a1", createdAt: now.AddDate(0, 0, -30)},
		{name: "v1.1", digest: "sha256:a2", createdAt: now.AddDate(0, 0, -20)},
		{name: "v1.2", digest: "sha256:a3", createdAt: now.AddDate(0, 0, -10)},
		{name: "v1.3", digest: "sha256:a4", createdAt: now.AddDate(0, 0, -1)},
	}

	toDelete := svc.selectTagsForDeletion(policy, tags)
	if len(toDelete) != 2 {
		t.Fatalf("expected 2 tags to delete, got %d", len(toDelete))
	}

	names := tagNames(toDelete)
	if !containsString(names, "v1.0") || !containsString(names, "v1.1") {
		t.Errorf("expected v1.0 and v1.1 to be deleted, got %v", names)
	}
	if containsString(names, "v1.2") || containsString(names, "v1.3") {
		t.Errorf("v1.2 and v1.3 should be kept, got deletions: %v", names)
	}
}

func TestSelectTagsForDeletion_Regex(t *testing.T) {
	svc := &RetentionService{}

	policy := &model.RetentionPolicy{KeepPattern: `^v\d+\.\d+\.\d+$`}

	tags := []tagInfo{
		{name: "v1.0.0", digest: "sha256:a1", createdAt: time.Now()},
		{name: "v1.1.0", digest: "sha256:a2", createdAt: time.Now()},
		{name: "test-build", digest: "sha256:a3", createdAt: time.Now()},
		{name: "pr-123", digest: "sha256:a4", createdAt: time.Now()},
	}

	toDelete := svc.selectTagsForDeletion(policy, tags)
	if len(toDelete) != 2 {
		t.Fatalf("expected 2 tags to delete, got %d", len(toDelete))
	}

	names := tagNames(toDelete)
	if !containsString(names, "test-build") || !containsString(names, "pr-123") {
		t.Errorf("expected test-build and pr-123 to be deleted, got %v", names)
	}
}

func TestSelectTagsForDeletion_ProtectLatest(t *testing.T) {
	svc := &RetentionService{}
	now := time.Now()

	tags := []tagInfo{
		{name: "latest", digest: "sha256:latest", createdAt: now.AddDate(0, 0, -365)},
		{name: "v1.0", digest: "sha256:v1", createdAt: now.AddDate(0, 0, -30)},
	}

	t.Run("time_based", func(t *testing.T) {
		policy := &model.RetentionPolicy{KeepDays: 1}
		toDelete := svc.selectTagsForDeletion(policy, tags)
		for _, td := range toDelete {
			if td.name == "latest" {
				t.Error("'latest' should not be deleted by time-based rule")
			}
		}
	})

	t.Run("count_based", func(t *testing.T) {
		policy := &model.RetentionPolicy{KeepCount: 1}
		toDelete := svc.selectTagsForDeletion(policy, tags)
		for _, td := range toDelete {
			if td.name == "latest" {
				t.Error("'latest' should not be deleted by count-based rule")
			}
		}
	})

	t.Run("regex_based", func(t *testing.T) {
		policy := &model.RetentionPolicy{KeepPattern: `^v\d`}
		toDelete := svc.selectTagsForDeletion(policy, tags)
		for _, td := range toDelete {
			if td.name == "latest" {
				t.Error("'latest' should not be deleted by regex rule")
			}
		}
	})
}

func TestSelectTagsForDeletion_NoDuplicates(t *testing.T) {
	svc := &RetentionService{}
	now := time.Now()

	policy := &model.RetentionPolicy{
		KeepDays:  10,
		KeepCount: 1,
	}

	tags := []tagInfo{
		{name: "v1.0", digest: "sha256:a1", createdAt: now.AddDate(0, 0, -30)},
		{name: "v1.1", digest: "sha256:a2", createdAt: now.AddDate(0, 0, -20)},
		{name: "v1.2", digest: "sha256:a3", createdAt: now.AddDate(0, 0, -1)},
	}

	toDelete := svc.selectTagsForDeletion(policy, tags)
	names := tagNames(toDelete)
	counts := make(map[string]int)
	for _, n := range names {
		counts[n]++
	}
	for name, count := range counts {
		if count > 1 {
			t.Errorf("tag %q appears %d times in deletion list, expected at most 1", name, count)
		}
	}
}

// ---- Glob Pattern Tests ----

func TestMatchingRepos_GlobPattern(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		match   bool
	}{
		{"myapp/*", "myapp/backend", true},
		{"myapp/*", "myapp/frontend", true},
		{"myapp/*", "other/backend", false},
		{"*", "myapp/backend", false},
		{"*", "other", true},
		{"myapp/**", "myapp/backend/api", false},
		{"*-api", "myapp-api", true},
		{"*-api", "other-api", true},
		{"*-api", "myapp-web", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"/"+tt.name, func(t *testing.T) {
			ok, err := path.Match(tt.pattern, tt.name)
			if err != nil {
				t.Fatalf("path.Match error: %v", err)
			}
			if ok != tt.match {
				t.Errorf("path.Match(%q, %q) = %v, want %v", tt.pattern, tt.name, ok, tt.match)
			}
		})
	}
}

// ---- CRUD Tests with Real DB ----

func setupRetentionTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := db.Migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func TestCRUDWithDB(t *testing.T) {
	database := setupRetentionTestDB(t)

	policyRepo := db.NewRetentionPolicyRepo(database)
	regRepo := db.NewRegistryRepo(database, testSvcEncKey)
	svc := NewRetentionService(policyRepo, regRepo)
	ctx := context.Background()

	// Seed a registry first (foreign key constraint).
	reg := &model.Registry{
		Name: "test", URL: "https://registry.example.com",
		Type: model.GHCR, Username: "user", Enabled: true,
	}
	regID, err := regRepo.Create(ctx, reg)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}

	// Create.
	req := model.CreateRetentionPolicyRequest{
		RegistryID:  regID,
		RepoPattern: "myapp/*",
		KeepDays:    30,
		Enabled:     true,
	}
	created, err := svc.CreatePolicy(ctx, req)
	if err != nil {
		t.Fatalf("CreatePolicy: %v", err)
	}
	if created.ID <= 0 {
		t.Fatalf("expected positive ID, got %d", created.ID)
	}
	id := created.ID

	// Get.
	p, err := svc.GetPolicy(ctx, id)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if p.RepoPattern != "myapp/*" || p.KeepDays != 30 {
		t.Errorf("unexpected policy: %+v", p)
	}

	// List.
	policies, err := svc.ListPolicies(ctx)
	if err != nil {
		t.Fatalf("ListPolicies: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	// Update.
	updateReq := model.CreateRetentionPolicyRequest{
		RegistryID:  regID,
		RepoPattern: "myapp/*",
		KeepDays:    60,
		Enabled:     true,
	}
	_, err = svc.UpdatePolicy(ctx, id, updateReq)
	if err != nil {
		t.Fatalf("UpdatePolicy: %v", err)
	}
	updated, err := svc.GetPolicy(ctx, id)
	if err != nil {
		t.Fatalf("GetPolicy after update: %v", err)
	}
	if updated.KeepDays != 60 {
		t.Errorf("expected keep_days=60, got %d", updated.KeepDays)
	}

	// Delete.
	if err := svc.DeletePolicy(ctx, id); err != nil {
		t.Fatalf("DeletePolicy: %v", err)
	}
	_, err = svc.GetPolicy(ctx, id)
	if err == nil {
		t.Error("expected error for deleted policy, got nil")
	}
}

func TestGetPolicy_NotFound(t *testing.T) {
	database := setupRetentionTestDB(t)

	policyRepo := db.NewRetentionPolicyRepo(database)
	regRepo := db.NewRegistryRepo(database, testSvcEncKey)
	svc := NewRetentionService(policyRepo, regRepo)

	_, err := svc.GetPolicy(context.Background(), 999)
	if err == nil {
		t.Error("expected error for non-existent policy, got nil")
	}
}

func TestCreatePolicy_InvalidInput(t *testing.T) {
	database := setupRetentionTestDB(t)

	policyRepo := db.NewRetentionPolicyRepo(database)
	regRepo := db.NewRegistryRepo(database, testSvcEncKey)
	svc := NewRetentionService(policyRepo, regRepo)
	ctx := context.Background()

	tests := []struct {
		name string
		req  model.CreateRetentionPolicyRequest
	}{
		{
			name: "no rules",
			req: model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "app/*",
				Enabled:     true,
			},
		},
		{
			name: "empty pattern",
			req: model.CreateRetentionPolicyRequest{
				RegistryID: 1,
				KeepDays:   30,
				Enabled:    true,
			},
		},
		{
			name: "bad regex",
			req: model.CreateRetentionPolicyRequest{
				RegistryID:  1,
				RepoPattern: "app/*",
				KeepPattern: `[`,
				Enabled:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreatePolicy(ctx, tt.req)
			if err == nil {
				t.Error("expected validation error, got nil")
			}
		})
	}
}

// ---- Scheduler Tests ----

func TestScheduler_StartStop(t *testing.T) {
	scheduler := NewRetentionScheduler(nil, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.Start(ctx)
	time.Sleep(100 * time.Millisecond)
	scheduler.Stop()
	time.Sleep(50 * time.Millisecond)
}

func TestScheduler_ContextCancellation(t *testing.T) {
	scheduler := NewRetentionScheduler(nil, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	scheduler.Start(ctx)
	time.Sleep(80 * time.Millisecond)
	cancel()
	time.Sleep(80 * time.Millisecond)
}

// ---- Helpers ----

func tagNames(tags []tagInfo) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.name
	}
	return names
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
