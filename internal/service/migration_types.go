package service

// MigrationTaskInfo holds the state of a storage migration task.
// T5 will extend this with full migration logic.
type MigrationTaskInfo struct {
	ID            int64   `json:"id"`
	Module        string  `json:"module"`
	OldPath       string  `json:"old_path"`
	NewPath       string  `json:"new_path"`
	Status        string  `json:"status"`
	Progress      int     `json:"progress"`
	TotalFiles    int     `json:"total_files"`
	MigratedFiles int     `json:"migrated_files"`
	TotalBytes    int64   `json:"total_bytes"`
	MigratedBytes int64   `json:"migrated_bytes"`
	StartedAt     *string `json:"started_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
	ErrorMessage  string  `json:"error_message,omitempty"`
	CreatedAt     string  `json:"created_at"`
}
