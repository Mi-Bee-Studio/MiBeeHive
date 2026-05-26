package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// Accept header for manifest requests — V2 Schema 2 and OCI types only.
const manifestAcceptHeader = "application/vnd.docker.distribution.manifest.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.index.v1+json"

// maxManifestRecursion prevents infinite recursion on nested manifest lists.
const maxManifestRecursion = 2

// targetOS and targetArch for multi-arch resolution.
const (
	targetOS   = "linux"
	targetArch = "arm64"
)

// ---- JSON types for parsing manifest responses ----

// manifestListResponse is a Docker manifest list or OCI image index.
type manifestListResponse struct {
	SchemaVersion int                       `json:"schemaVersion"`
	MediaType     string                    `json:"mediaType"`
	Manifests     []manifestListDescriptor  `json:"manifests"`
}

type manifestListDescriptor struct {
	MediaType string             `json:"mediaType"`
	Digest    string             `json:"digest"`
	Size      int64              `json:"size"`
	Platform  *platformDescriptor `json:"platform"`
}

type platformDescriptor struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant,omitempty"`
}

// manifestResponse is a Docker v2 or OCI single-arch manifest.
type manifestResponse struct {
	SchemaVersion int          `json:"schemaVersion"`
	MediaType     string       `json:"mediaType"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}

type descriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
}

// imageConfig is the JSON structure of an image config blob.
type imageConfig struct {
	Created string         `json:"created"`
	History []historyEntry `json:"history"`
}

type historyEntry struct {
	CreatedBy  string `json:"created_by"`
	EmptyLayer bool   `json:"empty_layer"`
}

// manifestResult holds a fetched manifest with its digest and raw body.
type manifestResult struct {
	detail      *model.ManifestDetail
	digest      string
	rawBody     []byte
	contentType string
}

// Manifest fetches and parses a manifest for the given repo and ref (tag or digest).
// For multi-arch images, it automatically resolves to the linux/arm64 variant.
func (c *RegistryClient) Manifest(ctx context.Context, repo, ref string) (*model.ManifestDetail, error) {
	result, err := c.fetchManifest(ctx, repo, ref, 0)
	if err != nil {
		return nil, err
	}
	return result.detail, nil
}

// TagDetail returns both tag metadata and manifest detail for a tag.
// The tag's size is the sum of all layer sizes; created time comes from the image config.
func (c *RegistryClient) TagDetail(ctx context.Context, repo, tag string) (*model.RegistryTag, *model.ManifestDetail, error) {
	result, err := c.fetchManifest(ctx, repo, tag, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch manifest for tag %q: %w", tag, err)
	}

	detail := result.detail

	// Sum layer sizes for total tag size.
	var totalSize int64
	for _, layer := range detail.Layers {
		totalSize += layer.Size
	}

	// Get created time from config blob.
	var createdAt time.Time
	if detail.Config.Digest != "" {
		cfg, err := c.fetchImageConfig(ctx, repo, detail.Config.Digest)
		if err == nil && cfg.Created != "" {
			createdAt, _ = time.Parse(time.RFC3339Nano, cfg.Created)
		}
	}

	platform := detail.Platform.OS + "/" + detail.Platform.Architecture

	registryTag := &model.RegistryTag{
		Name:          tag,
		Digest:        result.digest,
		Size:          totalSize,
		CreatedAt:     createdAt,
		MediaType:     detail.MediaType,
		SchemaVersion: detail.SchemaVersion,
		Platform:      platform,
	}

	return registryTag, detail, nil
}

// DeleteManifest deletes a manifest by digest. The digest must be a full content-addressable
// digest (e.g. "sha256:abc123"), NOT a tag name.
func (c *RegistryClient) DeleteManifest(ctx context.Context, repo, digest string) error {
	if !strings.Contains(digest, ":") {
		return fmt.Errorf("delete manifest: must use a digest (e.g. sha256:abc...), not a tag name")
	}

	path := fmt.Sprintf("/v2/%s/manifests/%s", repo, digest)
	req, err := c.newRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return fmt.Errorf("create delete manifest request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("delete manifest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return statusCodeError(resp)
	}

	return nil
}

// fetchManifest fetches a manifest and resolves multi-arch images.
func (c *RegistryClient) fetchManifest(ctx context.Context, repo, ref string, depth int) (*manifestResult, error) {
	if depth >= maxManifestRecursion {
		return nil, fmt.Errorf("manifest resolution: max recursion depth (%d) exceeded", maxManifestRecursion)
	}

	path := fmt.Sprintf("/v2/%s/manifests/%s", repo, ref)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create manifest request: %w", err)
	}
	req.Header.Set("Accept", manifestAcceptHeader)

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("manifest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusCodeError(resp)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read manifest body: %w", err)
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	contentType := resp.Header.Get("Content-Type")

	// Check if this is a manifest list / image index.
	if isManifestListContentType(contentType) {
		return c.resolveManifestList(ctx, repo, body, digest, depth)
	}

	// Parse as single-arch manifest.
	return c.parseSingleManifest(body, digest, contentType)
}

// resolveManifestList finds the target platform manifest and fetches it.
func (c *RegistryClient) resolveManifestList(ctx context.Context, repo string, body []byte, topDigest string, depth int) (*manifestResult, error) {
	var list manifestListResponse
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("parse manifest list: %w", err)
	}

	// Find the descriptor matching our target platform.
	for _, desc := range list.Manifests {
		if desc.Platform != nil &&
			desc.Platform.OS == targetOS &&
			desc.Platform.Architecture == targetArch {
			result, err := c.fetchManifest(ctx, repo, desc.Digest, depth+1)
			if err != nil {
				return nil, fmt.Errorf("resolve %s/%s manifest: %w", targetOS, targetArch, err)
			}
			// Keep the top-level digest as the tag's canonical digest.
			result.digest = topDigest
			return result, nil
		}
	}

	return nil, fmt.Errorf("manifest list: no manifest found for %s/%s", targetOS, targetArch)
}

// parseSingleManifest parses a single-arch manifest response into a ManifestDetail.
func (c *RegistryClient) parseSingleManifest(body []byte, digest, contentType string) (*manifestResult, error) {
	var m manifestResponse
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	detail := &model.ManifestDetail{
		SchemaVersion: m.SchemaVersion,
		MediaType:     m.MediaType,
		Config: struct {
			Digest string `json:"digest"`
		}{
			Digest: m.Config.Digest,
		},
	}

	// Convert descriptors to LayerInfo.
	for _, d := range m.Layers {
		detail.Layers = append(detail.Layers, model.LayerInfo{
			Digest:    d.Digest,
			MediaType: d.MediaType,
			Size:      d.Size,
		})
	}

	return &manifestResult{detail: detail, digest: digest, rawBody: body, contentType: contentType}, nil
}

// fetchImageConfig fetches and parses the image config blob for layer commands and created time.
func (c *RegistryClient) fetchImageConfig(ctx context.Context, repo, digest string) (*imageConfig, error) {
	path := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create config request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("config request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusCodeError(resp)
	}

	var cfg imageConfig
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	return &cfg, nil
}

// populateLayerCommands fills the Command field on each LayerInfo from the image config history.
// Non-empty history entries are matched 1:1 with layers in order.
func populateLayerCommands(detail *model.ManifestDetail, cfg *imageConfig) {
	if cfg == nil || len(cfg.History) == 0 {
		return
	}

	// Collect non-empty-layer history entries — they correspond to actual layers.
	var commands []string
	for _, h := range cfg.History {
		if !h.EmptyLayer {
			commands = append(commands, h.CreatedBy)
		}
	}

	// Assign commands to layers 1:1.
	for i := range detail.Layers {
		if i < len(commands) {
			detail.Layers[i].Command = commands[i]
		}
	}
}

// isManifestListContentType checks if a content type indicates a manifest list / image index.
func isManifestListContentType(ct string) bool {
	return strings.Contains(ct, "manifest.list") ||
		strings.Contains(ct, "image.index")
}
