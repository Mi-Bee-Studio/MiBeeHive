package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTags(t *testing.T) {
	expectedTags := []string{"latest", "v1.0", "v1.1"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/myrepo/myimage/tags/list" {
			t.Errorf("path = %q, want /v2/myrepo/myimage/tags/list", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "myrepo/myimage",
			"tags": expectedTags,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	tags, err := client.Tags(context.Background(), "myrepo/myimage", 10, "")
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("len = %d, want 3", len(tags))
	}
	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("tags[%d] = %q, want %q", i, tag, expectedTags[i])
		}
	}
}

func TestTags_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		_ = r.URL.Query().Get("n") // page size is implementation detail

		var tags []string
		var link string
		switch callCount {
		case 1:
			tags = []string{"v1.0", "v1.1"}
			link = `</v2/myrepo/tags/list?n=100&last=v1.1>; rel="next"`
		case 2:
			tags = []string{"v2.0"}
			// No Link header — last page.
		}

		w.Header().Set("Content-Type", "application/json")
		if link != "" {
			w.Header().Set("Link", link)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "myrepo",
			"tags": tags,
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

		tags, err := client.TagsWithPagination(context.Background(), "myrepo")
	if err != nil {
		t.Fatalf("TagsWithPagination: %v", err)
	}

	if len(tags) != 3 {
		t.Fatalf("len = %d, want 3", len(tags))
	}
	expected := []string{"v1.0", "v1.1", "v2.0"}
	for i, tag := range tags {
		if tag != expected[i] {
			t.Errorf("tags[%d] = %q, want %q", i, tag, expected[i])
		}
	}
	if callCount != 2 {
		t.Errorf("callCount = %d, want 2", callCount)
	}
}

func TestTags_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name": "empty/repo",
			"tags": []string{},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	tags, err := client.Tags(context.Background(), "empty/repo", 10, "")
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("len = %d, want 0", len(tags))
	}
}

func TestTags_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.Tags(context.Background(), "nonexistent/repo", 10, "")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestParseLinkNext(t *testing.T) {
	tests := []struct {
		input  string
		want   string
		wantOK bool
	}{
		{`</v2/repo/tags/list?n=10&last=v1>; rel="next"`, "/v2/repo/tags/list?n=10&last=v1", true},
		{`</v2/repo/tags/list?n=10>; rel="prev"`, "", false},
		{"no link header", "", false},
		{``, "", false},
	}

	for _, tt := range tests {
		got, ok := parseLinkNext(tt.input)
		if ok != tt.wantOK {
			t.Errorf("parseLinkNext(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
		}
		if got != tt.want {
			t.Errorf("parseLinkNext(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
