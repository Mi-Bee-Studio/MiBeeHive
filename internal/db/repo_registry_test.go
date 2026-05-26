package db

import (
	"context"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

var testEncKey = []byte("test-encryption-key-1234567890ab") // exactly 32 bytes for AES-256
const testEncKeyStr = "test-encryption-key-1234567890ab"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"normal password", "s3cretP@ss!"},
		{"empty string", ""},
		{"long password", "this-is-a-very-long-password-with-special-chars-!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"unicode", "密码パスワードмотdeпасе"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := encryptPassword(tc.input, testEncKey)
			if err != nil {
				t.Fatalf("encryptPassword(%q): %v", tc.input, err)
			}

			if tc.input == "" {
				if encrypted != "" {
					t.Errorf("expected empty encrypted for empty input, got %q", encrypted)
				}
				return
			}

			decrypted, err := decryptPassword(encrypted, testEncKey)
			if err != nil {
				t.Fatalf("decryptPassword: %v", err)
			}

			if decrypted != tc.input {
				t.Errorf("round-trip failed: input=%q got=%q", tc.input, decrypted)
			}
		})
	}
}

func TestRegistryRepoCRUD(t *testing.T) {
	db := testDB(t)
	repo := NewRegistryRepo(db, testEncKeyStr)
	ctx := context.Background()

	reg := &model.Registry{
		Name:             "DockerHub",
		URL:              "https://registry-1.docker.io",
		Type:             model.DockerHub,
		Username:         "admin",
		EncryptedPassword: "s3cretP@ss",
		Enabled:          true,
	}

	id, err := repo.Create(ctx, reg)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected registry, got nil")
	}
	if got.Name != "DockerHub" {
		t.Errorf("expected name=DockerHub, got %q", got.Name)
	}
	if got.Type != model.DockerHub {
		t.Errorf("expected type=dockerhub, got %q", got.Type)
	}
	if got.Username != "admin" {
		t.Errorf("expected username=admin, got %q", got.Username)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}

	password, err := repo.DecryptPassword(ctx, id)
	if err != nil {
		t.Fatalf("DecryptPassword: %v", err)
	}
	if password != "s3cretP@ss" {
		t.Errorf("password round-trip failed: got %q", password)
	}

	got, err = repo.GetByURL(ctx, "https://registry-1.docker.io")
	if err != nil {
		t.Fatalf("GetByURL: %v", err)
	}
	if got == nil || got.ID != id {
		t.Errorf("GetByURL: expected id=%d, got %v", id, got)
	}

	got, err = repo.GetByURL(ctx, "https://nonexistent.io")
	if err != nil {
		t.Fatalf("GetByURL(nonexistent): %v", err)
	}
	if got != nil {
		t.Error("GetByURL(nonexistent): expected nil")
	}
	// Re-fetch for update test.
	got, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID for update: %v", err)
	}
	got.Name = "DockerHub-Updated"
	got.Type = model.GHCR
	got.EncryptedPassword = "newPassword123"
	err = repo.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.Name != "DockerHub-Updated" {
		t.Errorf("expected name=DockerHub-Updated, got %q", updated.Name)
	}
	if updated.Type != model.GHCR {
		t.Errorf("expected type=ghcr, got %q", updated.Type)
	}

	password, err = repo.DecryptPassword(ctx, id)
	if err != nil {
		t.Fatalf("DecryptPassword after update: %v", err)
	}
	if password != "newPassword123" {
		t.Errorf("password after update: got %q", password)
	}

	got.EncryptedPassword = ""
	got.Name = "NoPasswordChange"
	err = repo.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update (no password change): %v", err)
	}
	password, err = repo.DecryptPassword(ctx, id)
	if err != nil {
		t.Fatalf("DecryptPassword after no-change update: %v", err)
	}
	if password != "newPassword123" {
		t.Errorf("password should not change: got %q", password)
	}

	reg2 := &model.Registry{
		Name:             "Quay",
		URL:              "https://quay.io",
		Type:             model.Quay,
		Username:         "user",
		EncryptedPassword: "quayPass",
		Enabled:          true,
	}
	_, err = repo.Create(ctx, reg2)
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	registries, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(registries) != 2 {
		t.Fatalf("List: expected 2, got %d", len(registries))
	}

	err = repo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err = repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got != nil {
		t.Error("GetByID after delete: expected nil")
	}
}

func TestSyncTaskRepoCRUD(t *testing.T) {
	db := testDB(t)
	regRepo := NewRegistryRepo(db, testEncKeyStr)
	taskRepo := NewSyncTaskRepo(db)
	ctx := context.Background()

	reg1 := &model.Registry{
		Name: "Source", URL: "https://source.io", Type: model.DockerHub,
		Username: "admin", EncryptedPassword: "pass", Enabled: true,
	}
	reg2 := &model.Registry{
		Name: "Target", URL: "https://target.io", Type: model.DockerHub,
		Username: "admin", EncryptedPassword: "pass", Enabled: true,
	}
	srcID, _ := regRepo.Create(ctx, reg1)
	tgtID, _ := regRepo.Create(ctx, reg2)

	task := &model.SyncTask{
		SourceRegistryID: srcID,
		TargetRegistryID: tgtID,
		SourceRepo:       "library/nginx",
		SourceTag:        "latest",
		TargetRepo:       "local/nginx",
		TargetTag:        "latest",
	}

	id, err := taskRepo.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := taskRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected task, got nil")
	}
	if got.Status != model.SyncTaskPending {
		t.Errorf("expected status=pending, got %q", got.Status)
	}
	if got.SourceRepo != "library/nginx" {
		t.Errorf("expected source_repo=library/nginx, got %q", got.SourceRepo)
	}

	active, err := taskRepo.GetActiveByTarget(ctx, tgtID, "local/nginx", "latest")
	if err != nil {
		t.Fatalf("GetActiveByTarget: %v", err)
	}
	if active == nil || active.ID != id {
		t.Error("GetActiveByTarget: expected active task")
	}

	err = taskRepo.Start(ctx, id)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	got, _ = taskRepo.GetByID(ctx, id)
	if got.Status != model.SyncTaskRunning {
		t.Errorf("expected status=running after Start, got %q", got.Status)
	}

	err = taskRepo.UpdateProgress(ctx, id, 5120, 10240)
	if err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}
	got, _ = taskRepo.GetByID(ctx, id)
	if got.ProgressBytes != 5120 || got.TotalBytes != 10240 {
		t.Errorf("progress: got %d/%d, expected 5120/10240", got.ProgressBytes, got.TotalBytes)
	}

	err = taskRepo.Complete(ctx, id)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, _ = taskRepo.GetByID(ctx, id)
	if got.Status != model.SyncTaskCompleted {
		t.Errorf("expected status=completed, got %q", got.Status)
	}

	err = taskRepo.Start(ctx, id)
	if err == nil {
		t.Error("expected error starting completed task")
	}

	err = taskRepo.Complete(ctx, id)
	if err == nil {
		t.Error("expected error completing completed task")
	}

	task2 := &model.SyncTask{
		SourceRegistryID: srcID, TargetRegistryID: tgtID,
		SourceRepo: "library/redis", SourceTag: "7",
		TargetRepo: "local/redis", TargetTag: "7",
	}
	id2, _ := taskRepo.Create(ctx, task2)

	err = taskRepo.Fail(ctx, id2, "connection refused")
	if err != nil {
		t.Fatalf("Fail from pending: %v", err)
	}
	got, _ = taskRepo.GetByID(ctx, id2)
	if got.Status != model.SyncTaskFailed {
		t.Errorf("expected status=failed, got %q", got.Status)
	}
	if got.ErrorMessage != "connection refused" {
		t.Errorf("expected error_message=connection refused, got %q", got.ErrorMessage)
	}

	task3 := &model.SyncTask{
		SourceRegistryID: srcID, TargetRegistryID: tgtID,
		SourceRepo: "library/alpine", SourceTag: "3",
		TargetRepo: "local/alpine", TargetTag: "3",
	}
	id3, _ := taskRepo.Create(ctx, task3)

	err = taskRepo.Cancel(ctx, id3)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	got, _ = taskRepo.GetByID(ctx, id3)
	if got.Status != model.SyncTaskFailed {
		t.Errorf("expected status=failed after cancel, got %q", got.Status)
	}
	if got.ErrorMessage != "cancelled" {
		t.Errorf("expected error_message=cancelled, got %q", got.ErrorMessage)
	}

	task4 := &model.SyncTask{
		SourceRegistryID: srcID, TargetRegistryID: tgtID,
		SourceRepo: "library/busybox", SourceTag: "1",
		TargetRepo: "local/busybox", TargetTag: "1",
	}
	id4, _ := taskRepo.Create(ctx, task4)
	taskRepo.Start(ctx, id4)
	taskRepo.Fail(ctx, id4, "disk full")

	task5 := &model.SyncTask{
		SourceRegistryID: srcID, TargetRegistryID: tgtID,
		SourceRepo: "library/node", SourceTag: "18",
		TargetRepo: "local/node", TargetTag: "18",
	}
	_, _ = taskRepo.Create(ctx, task5)

	pending, err := taskRepo.ListByStatus(ctx, model.SyncTaskPending)
	if err != nil {
		t.Fatalf("ListByStatus(pending): %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending task, got %d", len(pending))
	}

	failed, err := taskRepo.ListByStatus(ctx, model.SyncTaskFailed)
	if err != nil {
		t.Fatalf("ListByStatus(failed): %v", err)
	}
	if len(failed) != 3 {
		t.Errorf("expected 3 failed tasks, got %d", len(failed))
	}

	noTask, err := taskRepo.GetActiveByTarget(ctx, tgtID, "local/nginx", "latest")
	if err != nil {
		t.Fatalf("GetActiveByTarget completed task: %v", err)
	}
	if noTask != nil {
		t.Error("completed task should not be active")
	}

	got, err = taskRepo.GetByID(ctx, 9999)
	if err != nil {
		t.Fatalf("GetByID nonexistent: %v", err)
	}
	if got != nil {
		t.Error("GetByID nonexistent: expected nil")
	}
}

func TestRetentionPolicyRepoCRUD(t *testing.T) {
	db := testDB(t)
	regRepo := NewRegistryRepo(db, testEncKeyStr)
	polRepo := NewRetentionPolicyRepo(db)
	ctx := context.Background()

	reg := &model.Registry{
		Name: "TestReg", URL: "https://test.io", Type: model.DockerHub,
		Username: "admin", EncryptedPassword: "pass", Enabled: true,
	}
	regID, _ := regRepo.Create(ctx, reg)

	policy := &model.RetentionPolicy{
		RegistryID:  regID,
		RepoPattern: "library/*",
		KeepDays:    30,
		KeepCount:   5,
		KeepPattern: "^v\\d+",
		Enabled:     true,
	}

	id, err := polRepo.Create(ctx, policy)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := polRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID: expected policy, got nil")
	}
	if got.RepoPattern != "library/*" {
		t.Errorf("expected repo_pattern=library/*, got %q", got.RepoPattern)
	}
	if got.KeepDays != 30 {
		t.Errorf("expected keep_days=30, got %d", got.KeepDays)
	}
	if got.KeepCount != 5 {
		t.Errorf("expected keep_count=5, got %d", got.KeepCount)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}

	got.KeepDays = 60
	got.KeepCount = 10
	got.Enabled = false
	err = polRepo.Update(ctx, got)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	updated, err := polRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if updated.KeepDays != 60 {
		t.Errorf("expected keep_days=60, got %d", updated.KeepDays)
	}
	if updated.Enabled {
		t.Error("expected enabled=false")
	}

	policy2 := &model.RetentionPolicy{
		RegistryID:  regID,
		RepoPattern: "prod/*",
		KeepDays:    7,
		KeepCount:   3,
		Enabled:     true,
	}
	polRepo.Create(ctx, policy2)

	policy3 := &model.RetentionPolicy{
		RegistryID:  regID,
		RepoPattern: "staging/*",
		KeepDays:    14,
		KeepCount:   2,
		Enabled:     false,
	}
	polRepo.Create(ctx, policy3)

	all, err := polRepo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("List: expected 3, got %d", len(all))
	}

	enabled, err := polRepo.ListEnabled(ctx)
	if err != nil {
		t.Fatalf("ListEnabled: %v", err)
	}
	if len(enabled) != 1 {
		t.Errorf("ListEnabled: expected 1, got %d", len(enabled))
	}
	if enabled[0].RepoPattern != "prod/*" {
		t.Errorf("ListEnabled: expected prod/*, got %q", enabled[0].RepoPattern)
	}

	execTime := time.Now().UTC().Truncate(time.Second)
	err = polRepo.UpdateLastExecuted(ctx, id, execTime)
	if err != nil {
		t.Fatalf("UpdateLastExecuted: %v", err)
	}

	updated, _ = polRepo.GetByID(ctx, id)
	if updated.LastExecutedAt == nil {
		t.Fatal("expected last_executed_at to be set")
	}
	if !updated.LastExecutedAt.Equal(execTime) {
		t.Errorf("last_executed_at: expected %v, got %v", execTime, updated.LastExecutedAt)
	}

	got, err = polRepo.GetByID(ctx, 9999)
	if err != nil {
		t.Fatalf("GetByID nonexistent: %v", err)
	}
	if got != nil {
		t.Error("GetByID nonexistent: expected nil")
	}

	err = polRepo.Delete(ctx, id)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, err = polRepo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got != nil {
		t.Error("GetByID after delete: expected nil")
	}

	all, _ = polRepo.List(ctx)
	if len(all) != 2 {
		t.Errorf("List after delete: expected 2, got %d", len(all))
	}
}
