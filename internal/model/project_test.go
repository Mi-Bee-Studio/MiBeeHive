package model

import (
	"encoding/json"
	"testing"
)

func TestProjectSettingsJSONRoundTrip(t *testing.T) {
	original := ProjectSettings{
		CrawlInterval:  3600,
		GitHubOwner:    "prometheus",
		GitHubRepo:     "prometheus",
		FilterPatterns: []string{"linux-arm64", "linux-amd64"},
		StorageSubpath: "prometheus",
		DownloadAll:    true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded ProjectSettings
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.CrawlInterval != original.CrawlInterval {
		t.Errorf("CrawlInterval: expected %d, got %d", original.CrawlInterval, decoded.CrawlInterval)
	}
	if decoded.GitHubOwner != original.GitHubOwner {
		t.Errorf("GitHubOwner: expected %q, got %q", original.GitHubOwner, decoded.GitHubOwner)
	}
	if decoded.GitHubRepo != original.GitHubRepo {
		t.Errorf("GitHubRepo: expected %q, got %q", original.GitHubRepo, decoded.GitHubRepo)
	}
	if len(decoded.FilterPatterns) != len(original.FilterPatterns) {
		t.Errorf("FilterPatterns: expected %d items, got %d", len(original.FilterPatterns), len(decoded.FilterPatterns))
	}
	if decoded.StorageSubpath != original.StorageSubpath {
		t.Errorf("StorageSubpath: expected %q, got %q", original.StorageSubpath, decoded.StorageSubpath)
	}
	if decoded.DownloadAll != original.DownloadAll {
		t.Errorf("DownloadAll: expected %v, got %v", original.DownloadAll, decoded.DownloadAll)
	}
}

func TestProjectSettingsEmptyRoundTrip(t *testing.T) {
	original := ProjectSettings{}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	// Empty struct should produce "{}"
	if string(data) != "{}" {
		t.Errorf("empty ProjectSettings: expected {}, got %s", string(data))
	}

	var decoded ProjectSettings
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.CrawlInterval != 0 || decoded.DownloadAll != false {
		t.Error("empty struct should have zero values")
	}
}
