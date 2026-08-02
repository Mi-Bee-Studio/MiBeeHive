package service

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// mockTokenChecker is a DBChecker stub for testing token generation.
type mockTokenChecker struct {
	exists bool
	calls  int
}

func (m *mockTokenChecker) TokenExists(_ context.Context, _ string) (bool, error) {
	m.calls++
	return m.exists, nil
}

func isBase58(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune(base58Alphabet, r) {
			return false
		}
	}
	return true
}

func TestPublicTokenLength(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"first"},
		{"second"},
		{"third"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &mockTokenChecker{exists: false}
			token, err := GeneratePublicToken(checker)
			if err != nil {
				t.Fatalf("GeneratePublicToken: %v", err)
			}
			if len(token) != publicTokenLength {
				t.Errorf("token length = %d, want %d", len(token), publicTokenLength)
			}
			if !isBase58(token) {
				t.Errorf("token %q contains non-base58 characters", token)
			}
		})
	}
}

func TestPublicTokenUniqueness(t *testing.T) {
	checker := &mockTokenChecker{exists: false}
	seen := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		token, err := GeneratePublicToken(checker)
		if err != nil {
			t.Fatalf("GeneratePublicToken iteration %d: %v", i, err)
		}
		if _, dup := seen[token]; dup {
			t.Fatalf("duplicate token generated: %q", token)
		}
		seen[token] = struct{}{}
	}
}

func TestPublicTokenCollisionRetry(t *testing.T) {
	checker := &mockTokenChecker{exists: true}
	_, err := GeneratePublicToken(checker)
	if err == nil {
		t.Fatal("expected error when token always collides")
	}
	if checker.calls != maxTokenRetries {
		t.Errorf("TokenExists called %d times, want %d", checker.calls, maxTokenRetries)
	}
}

func TestPublicTokenBackfillEmpty(t *testing.T) {
	testDB, err := sql.Open("sqlite", ":memory:?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { testDB.Close() })
	testDB.SetMaxOpenConns(1)

	if _, err := testDB.Exec(`CREATE TABLE files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		public_token TEXT DEFAULT NULL
	)`); err != nil {
		t.Fatalf("creating files table: %v", err)
	}

	if err := BackfillPublicTokens(testDB); err != nil {
		t.Fatalf("BackfillPublicTokens on empty table: %v", err)
	}
}