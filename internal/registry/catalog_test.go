package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCatalog(t *testing.T) {
	repos := []string{"repo1", "repo2", "repo3"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/_catalog" {
			t.Errorf("path = %q, want /v2/_catalog", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"repositories": repos,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.Catalog(context.Background(), 10, "")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("len = %d, want 3", len(result))
	}
	for i, repo := range result {
		if repo != repos[i] {
			t.Errorf("result[%d] = %q, want %q", i, repo, repos[i])
		}
	}
}

func TestCatalog_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		n := r.URL.Query().Get("n")
		last := r.URL.Query().Get("last")

		var repos []string
		if last == "" {
			repos = []string{"repo1", "repo2"}
		} else if last == "repo2" {
			repos = []string{"repo3"}
		}

		if n != "2" && callCount == 1 {
			t.Errorf("n = %q, want 2", n)
		}
		if callCount == 2 && last != "repo2" {
			t.Errorf("last = %q, want repo2", last)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"repositories": repos,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	result, err := client.Catalog(context.Background(), 2, "")
	if err != nil {
		t.Fatalf("Catalog: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
}

func TestCatalog_DockerHub(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Catalog(context.Background(), 10, "")
	if err == nil {
		t.Fatal("expected error for 404 catalog")
	}
	// Should contain helpful message about Docker Hub.
	if !containsSubstring(err.Error(), "does not support") {
		t.Errorf("error = %q, want message about unsupported endpoint", err.Error())
	}
}

func TestCatalog_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Catalog(context.Background(), 10, "")
	if err == nil {
		t.Fatal("expected error for 403 catalog")
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstring(s, sub)))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
