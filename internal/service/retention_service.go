package service

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"regexp"
	"sort"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/registry"
)

// RetentionService manages tag retention policies and their execution.
type RetentionService struct {
	policyRepo    *db.RetentionPolicyRepo
	registryRepo  *db.RegistryRepo
	clientFactory func(url string, creds *registry.Credentials, opts ...registry.ClientOption) (*registry.RegistryClient, error)
}

// NewRetentionService creates a new RetentionService.
func NewRetentionService(policyRepo *db.RetentionPolicyRepo, registryRepo *db.RegistryRepo) *RetentionService {
	return &RetentionService{
		policyRepo:    policyRepo,
		registryRepo:  registryRepo,
		clientFactory: registry.NewClient,
	}
}

// CreatePolicy creates a new retention policy after validation and returns it.
func (s *RetentionService) CreatePolicy(ctx context.Context, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error) {
	if err := validatePolicyRequest(&req); err != nil {
		return nil, fmt.Errorf("validating retention policy: %w", err)
	}

	policy := &model.RetentionPolicy{
		RegistryID:  req.RegistryID,
		RepoPattern: req.RepoPattern,
		KeepDays:    req.KeepDays,
		KeepCount:   req.KeepCount,
		KeepPattern: req.KeepPattern,
		Enabled:     req.Enabled,
	}

	id, err := s.policyRepo.Create(ctx, policy)
	if err != nil {
		return nil, fmt.Errorf("creating retention policy: %w", err)
	}
	policy.ID = id
	return policy, nil
}

// UpdatePolicy updates an existing retention policy and returns the updated policy.
func (s *RetentionService) UpdatePolicy(ctx context.Context, id int64, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error) {
	if err := validatePolicyRequest(&req); err != nil {
		return nil, fmt.Errorf("validating retention policy: %w", err)
	}

	policy := &model.RetentionPolicy{
		ID:          id,
		RegistryID:  req.RegistryID,
		RepoPattern: req.RepoPattern,
		KeepDays:    req.KeepDays,
		KeepCount:   req.KeepCount,
		KeepPattern: req.KeepPattern,
		Enabled:     req.Enabled,
	}

	if err := s.policyRepo.Update(ctx, policy); err != nil {
		return nil, fmt.Errorf("updating retention policy %d: %w", id, err)
	}
	return policy, nil
}

// DeletePolicy removes a retention policy by ID.
func (s *RetentionService) DeletePolicy(ctx context.Context, id int64) error {
	if err := s.policyRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("deleting retention policy %d: %w", id, err)
	}
	return nil
}

// ListPolicies returns all retention policies.
func (s *RetentionService) ListPolicies(ctx context.Context) ([]model.RetentionPolicy, error) {
	policies, err := s.policyRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing retention policies: %w", err)
	}
	return policies, nil
}

// GetPolicy retrieves a single retention policy by ID.
func (s *RetentionService) GetPolicy(ctx context.Context, id int64) (*model.RetentionPolicy, error) {
	policy, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting retention policy %d: %w", id, err)
	}
	if policy == nil {
		return nil, fmt.Errorf("retention policy %d not found", id)
	}
	return policy, nil
}

// ExecutePolicy runs a retention policy and returns the number of tags deleted.
func (s *RetentionService) ExecutePolicy(ctx context.Context, policyID int64) (int, error) {
	policy, err := s.policyRepo.GetByID(ctx, policyID)
	if err != nil {
		return 0, fmt.Errorf("getting retention policy %d: %w", policyID, err)
	}
	if policy == nil {
		return 0, fmt.Errorf("retention policy %d not found", policyID)
	}

	reg, err := s.registryRepo.GetByID(ctx, policy.RegistryID)
	if err != nil {
		return 0, fmt.Errorf("getting registry %d: %w", policy.RegistryID, err)
	}
	if reg == nil {
		return 0, fmt.Errorf("registry %d not found", policy.RegistryID)
	}

	password, err := s.registryRepo.DecryptPassword(ctx, reg.ID)
	if err != nil {
		return 0, fmt.Errorf("decrypting password for registry %d: %w", reg.ID, err)
	}

	var creds *registry.Credentials
	if reg.Username != "" {
		creds = &registry.Credentials{Username: reg.Username, Password: password}
	}

	client, err := s.clientFactory(reg.URL, creds)
	if err != nil {
		return 0, fmt.Errorf("creating registry client for %q: %w", reg.URL, err)
	}

	// Fetch repos matching the pattern.
	repos, err := s.matchingRepos(ctx, client, policy.RepoPattern)
	if err != nil {
		return 0, fmt.Errorf("fetching matching repos for pattern %q: %w", policy.RepoPattern, err)
	}

	totalDeleted := 0
	for _, repo := range repos {
		deleted, err := s.executePolicyOnRepo(ctx, client, policy, repo)
		if err != nil {
			slog.Error("retention policy failed on repo",
				"policy_id", policyID, "repo", repo, "error", err)
			continue
		}
		totalDeleted += deleted
	}

	// Update last_executed_at.
	now := time.Now().UTC()
	if err := s.policyRepo.UpdateLastExecuted(ctx, policyID, now); err != nil {
		slog.Error("updating last_executed_at", "policy_id", policyID, "error", err)
	}

	return totalDeleted, nil
}

// matchingRepos returns repository names that match the given glob pattern.
func (s *RetentionService) matchingRepos(ctx context.Context, client *registry.RegistryClient, pattern string) ([]string, error) {
	allRepos, err := client.Catalog(ctx, 0, "")
	if err != nil {
		// Catalog may not be supported (e.g. Docker Hub).
		return nil, fmt.Errorf("catalog: %w", err)
	}

	var matched []string
	for _, r := range allRepos {
		ok, _ := path.Match(pattern, r)
		if ok {
			matched = append(matched, r)
		}
	}
	return matched, nil
}

// tagInfo holds a tag name with its digest and creation time for sorting/filtering.
type tagInfo struct {
	name      string
	digest    string
	createdAt time.Time
}

// executePolicyOnRepo applies retention rules to a single repository.
func (s *RetentionService) executePolicyOnRepo(ctx context.Context, client *registry.RegistryClient, policy *model.RetentionPolicy, repo string) (int, error) {
	tagNames, err := client.TagsWithPagination(ctx, repo)
	if err != nil {
		return 0, fmt.Errorf("listing tags for %q: %w", repo, err)
	}
	if len(tagNames) == 0 {
		return 0, nil
	}

	// Collect tag details (name, digest, created_at).
	var tags []tagInfo
	for _, tagName := range tagNames {
		registryTag, _, err := client.TagDetail(ctx, repo, tagName)
		if err != nil {
			slog.Warn("skipping tag during retention",
				"repo", repo, "tag", tagName, "error", err)
			continue
		}
		tags = append(tags, tagInfo{
			name:      tagName,
			digest:    registryTag.Digest,
			createdAt: registryTag.CreatedAt,
		})
	}

	// Determine which tags to delete.
	toDelete := s.selectTagsForDeletion(policy, tags)

	// Delete qualifying tags.
	deleted := 0
	for _, t := range toDelete {
		if err := client.DeleteManifest(ctx, repo, t.digest); err != nil {
			slog.Warn("failed to delete tag",
				"repo", repo, "tag", t.name, "digest", t.digest, "error", err)
			continue
		}
		slog.Info("deleted tag via retention policy",
			"policy_id", policy.ID, "repo", repo, "tag", t.name)
		deleted++
	}
	return deleted, nil
}

// selectTagsForDeletion determines which tags should be deleted based on policy rules.
// The "latest" tag is protected unless it's explicitly targeted by a deletion rule.
func (s *RetentionService) selectTagsForDeletion(policy *model.RetentionPolicy, tags []tagInfo) []tagInfo {
	var toDelete []tagInfo

	// Apply time-based rule.
	if policy.KeepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -policy.KeepDays)
		for _, t := range tags {
			if t.createdAt.Before(cutoff) && t.name != "latest" {
				toDelete = append(toDelete, t)
			}
		}
	}

	// Apply count-based rule.
	if policy.KeepCount > 0 {
		// Sort by created_at descending.
		sorted := make([]tagInfo, len(tags))
		copy(sorted, tags)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].createdAt.After(sorted[j].createdAt)
		})

		for i := policy.KeepCount; i < len(sorted); i++ {
			if sorted[i].name != "latest" && !containsTag(toDelete, sorted[i].name) {
				toDelete = append(toDelete, sorted[i])
			}
		}
	}

	// Apply regex pattern rule.
	if policy.KeepPattern != "" {
		re := regexp.MustCompile(policy.KeepPattern)
		for _, t := range tags {
			if !re.MatchString(t.name) && t.name != "latest" && !containsTag(toDelete, t.name) {
				toDelete = append(toDelete, t)
			}
		}
	}

	return toDelete
}

// containsTag checks if a tag name is already in the deletion list.
func containsTag(tags []tagInfo, name string) bool {
	for _, t := range tags {
		if t.name == name {
			return true
		}
	}
	return false
}

// validatePolicyRequest validates that at least one retention rule is set.
func validatePolicyRequest(req *model.CreateRetentionPolicyRequest) error {
	if req.RepoPattern == "" {
		return fmt.Errorf("repo_pattern is required")
	}
	if req.KeepPattern != "" {
		if _, err := regexp.Compile(req.KeepPattern); err != nil {
			return fmt.Errorf("keep_pattern: invalid regex: %w", err)
		}
	}
	// At least ONE rule must be set.
	if req.KeepDays == 0 && req.KeepCount == 0 && req.KeepPattern == "" {
		return fmt.Errorf("at least one retention rule must be set (keep_days, keep_count, or keep_pattern)")
	}
	return nil
}
