package service

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

func newSecretTestService(t *testing.T) (*SecretService, string) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), ".secret.key")
	svc, err := NewSecretService(database, keyPath)
	if err != nil {
		t.Fatalf("new secret service: %v", err)
	}
	return svc, keyPath
}

func TestValidateSecretName(t *testing.T) {
	for _, ok := range []string{"GITEE_TOKEN", "A", "GH_TOKEN_2", "MY_SECRET"} {
		if err := ValidateSecretName(ok); err != nil {
			t.Errorf("ValidateSecretName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "lower", "with space", "1START", "PATH", "HOME", "MIBEEHIVE_URL", "LD_PRELOAD", "带中文", "a-b"} {
		if err := ValidateSecretName(bad); err == nil {
			t.Errorf("ValidateSecretName(%q) = nil, want error", bad)
		}
	}
}

func TestSecretRoundTrip(t *testing.T) {
	svc, keyPath := newSecretTestService(t)
	ctx := context.Background()

	if _, err := svc.SetSecret(ctx, "GITEE_TOKEN", "pat-abc123"); err != nil {
		t.Fatalf("set: %v", err)
	}
	// Replace.
	if _, err := svc.SetSecret(ctx, "GITEE_TOKEN", "pat-def456"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := svc.SetSecret(ctx, "GH_TOKEN", "gh-xyz"); err != nil {
		t.Fatalf("set second: %v", err)
	}

	// List exposes names/timestamps only — no values anywhere.
	metas, err := svc.ListSecrets(ctx)
	if err != nil || len(metas) != 2 {
		t.Fatalf("list = %v, %v", metas, err)
	}
	if metas[0].Name != "GH_TOKEN" || metas[1].Name != "GITEE_TOKEN" {
		t.Errorf("names = %q, %q (want alphabetical)", metas[0].Name, metas[1].Name)
	}
	if metas[0].UpdatedAt.IsZero() {
		t.Error("updated_at not populated")
	}

	// EnvPairs returns plaintext NAME=value for execution.
	pairs, err := svc.EnvPairs(ctx)
	if err != nil {
		t.Fatalf("EnvPairs: %v", err)
	}
	joined := strings.Join(pairs, "\n")
	if !strings.Contains(joined, "GITEE_TOKEN=pat-def456") || !strings.Contains(joined, "GH_TOKEN=gh-xyz") {
		t.Errorf("pairs = %q", pairs)
	}
	if strings.Contains(joined, "pat-abc123") {
		t.Error("stale value still present after replace")
	}

	// Empty value rejected.
	if _, err := svc.SetSecret(ctx, "BAD", ""); err == nil {
		t.Error("empty value accepted")
	}

	// Delete.
	if err := svc.DeleteSecret(ctx, "GH_TOKEN"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	metas, _ = svc.ListSecrets(ctx)
	if len(metas) != 1 {
		t.Errorf("after delete len = %d, want 1", len(metas))
	}

	// Key persistence: a new service instance over the SAME key file decrypts.
	database := svc.db
	svc2, err := NewSecretService(database, keyPath)
	if err != nil {
		t.Fatalf("reload with same key: %v", err)
	}
	pairs2, err := svc2.EnvPairs(ctx)
	if err != nil || strings.Join(pairs2, "\n") != "GITEE_TOKEN=pat-def456" {
		t.Errorf("reload pairs = %q, %v — key file not reused", pairs2, err)
	}

	// Wrong key file: value undecryptable, EnvPairs skips it (returns partial error).
	otherKey := filepath.Join(t.TempDir(), "other.key")
	svc3, err := NewSecretService(database, otherKey)
	if err != nil {
		t.Fatalf("third service: %v", err)
	}
	pairs3, err := svc3.EnvPairs(ctx)
	if err == nil || len(pairs3) != 0 {
		t.Errorf("wrong-key pairs = %q, err = %v — want skip+error", pairs3, err)
	}
}

func TestSecretsInjectedIntoScriptRun(t *testing.T) {
	svc, _ := newSecretTestService(t)
	ctx := context.Background()
	database := svc.db

	if _, err := svc.SetSecret(ctx, "TEST_SECRET", "injected-value-42"); err != nil {
		t.Fatal(err)
	}

	scriptsDir := t.TempDir()
	scriptSvc, err := NewScriptService(database, scriptsDir, "http://x", slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	scriptSvc.AttachSecrets(svc)

	writeScript(t, scriptsDir, "env-probe.sh", "echo \"secret=$TEST_SECRET\"\n")
	sc, err := scriptSvc.CreateScript(ctx, ScriptInput{Name: "envprobe", Schedule: "0 3 * * *", ScriptPath: "env-probe.sh"})
	if err != nil {
		t.Fatal(err)
	}
	run, err := scriptSvc.StartRun(ctx, sc.ID, ScriptTriggerManual)
	if err != nil {
		t.Fatal(err)
	}
	got := waitForRun(t, scriptSvc, run.ID, 10*time.Second)
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Fatalf("run exit = %v err = %q stdout = %q", got.ExitCode, got.Error, got.Stdout)
	}
	if !strings.Contains(got.Stdout, "secret=injected-value-42") {
		t.Errorf("stdout = %q, want injected secret value", got.Stdout)
	}
}
