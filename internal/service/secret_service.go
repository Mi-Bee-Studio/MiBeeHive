// Package service — named secret store for scheduled scripts (#70 follow-up).
//
// Secrets are entered once in the web UI (write-only), stored AES-256-GCM
// encrypted in SQLite, and injected as NAME=value environment variables into
// every scheduled script execution. Values are never returned by any API —
// only names and timestamps.
package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// SecretMeta is the non-secret projection returned by APIs.
type SecretMeta struct {
	Name      string    `json:"name"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ErrSecretValidation marks handler-mappable input errors.
var ErrSecretValidation = errors.New("secret validation error")

func secretValidationError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSecretValidation, fmt.Sprintf(format, args...))
}

// secretNameRE: uppercase ENV-style names only (GITEE_TOKEN, GH_TOKEN, …) —
// they double as environment variable names at injection time.
var secretNameRE = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// reservedSecretNames would clobber execution-critical environment when
// injected; reject them outright.
var reservedSecretNames = map[string]bool{
	"PATH": true, "HOME": true, "SHELL": true, "USER": true, "LOGNAME": true,
	"PWD": true, "TMPDIR": true, "IFS": true, "SHLVL": true, "LANG": true,
	"LD_PRELOAD": true, "LD_LIBRARY_PATH": true, "MIBEEHIVE_URL": true,
}

// ValidateSecretName checks an env-style uppercase name against the reserved
// set.
func ValidateSecretName(name string) error {
	if !secretNameRE.MatchString(name) {
		return secretValidationError("secret name must match %s (uppercase env-style, e.g. GITEE_TOKEN)", secretNameRE.String())
	}
	if reservedSecretNames[name] {
		return secretValidationError("secret name %s is reserved (would break script execution)", name)
	}
	if len(name) > 64 {
		return secretValidationError("secret name too long (max 64)")
	}
	return nil
}

// SecretService stores encrypted secrets and exports them as env pairs.
type SecretService struct {
	db  *sql.DB
	aes cipher.Block
}

// NewSecretService creates the service, loading or generating the 32-byte
// AES key at keyPath (0600).
func NewSecretService(db *sql.DB, keyPath string) (*SecretService, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("init aes: %w", err)
	}
	return &SecretService{db: db, aes: block}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	if key, err := os.ReadFile(path); err == nil {
		if len(key) == 32 {
			return key, nil
		}
		return nil, fmt.Errorf("secret key %s has wrong size %d (want 32)", path, len(key))
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading secret key: %w", err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating secret key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("creating key dir: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("writing secret key: %w", err)
	}
	return key, nil
}

func (s *SecretService) encrypt(plain string) ([]byte, error) {
	gcm, err := cipher.NewGCM(s.aes)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (s *SecretService) decrypt(blob []byte) (string, error) {
	gcm, err := cipher.NewGCM(s.aes)
	if err != nil {
		return "", err
	}
	if len(blob) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], nil)
	if err != nil {
		return "", fmt.Errorf("decrypting: %w", err)
	}
	return string(plain), nil
}

// SetSecret validates the name and creates or replaces the secret.
func (s *SecretService) SetSecret(ctx context.Context, name, value string) (*SecretMeta, error) {
	if err := ValidateSecretName(name); err != nil {
		return nil, err
	}
	if value == "" {
		return nil, secretValidationError("secret value must not be empty")
	}
	if len(value) > 8192 {
		return nil, secretValidationError("secret value too long (max 8192)")
	}
	blob, err := s.encrypt(value)
	if err != nil {
		return nil, fmt.Errorf("encrypting secret: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO secrets (name, value_encrypted) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET value_encrypted = excluded.value_encrypted,
		                                updated_at = CURRENT_TIMESTAMP`,
		name, blob); err != nil {
		return nil, fmt.Errorf("storing secret: %w", err)
	}
	return s.GetSecretMeta(ctx, name)
}

// GetSecretMeta returns name+timestamp only.
func (s *SecretService) GetSecretMeta(ctx context.Context, name string) (*SecretMeta, error) {
	var m SecretMeta
	err := s.db.QueryRowContext(ctx,
		`SELECT name, updated_at FROM secrets WHERE name = ?`, name).Scan(&m.Name, &m.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrScriptNotFound // not used by handler; placeholder
	}
	if err != nil {
		return nil, fmt.Errorf("getting secret: %w", err)
	}
	return &m, nil
}

// ListSecrets returns all secrets' metadata (never values).
func (s *SecretService) ListSecrets(ctx context.Context) ([]*SecretMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, updated_at FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing secrets: %w", err)
	}
	defer rows.Close()
	var out []*SecretMeta
	for rows.Next() {
		var m SecretMeta
		if err := rows.Scan(&m.Name, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning secret: %w", err)
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// DeleteSecret removes a secret.
func (s *SecretService) DeleteSecret(ctx context.Context, name string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM secrets WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("deleting secret: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrScriptNotFound
	}
	return nil
}

// EnvPairs decrypts all secrets into NAME=value environment pairs for script
// execution. Secrets that fail to decrypt are skipped with the error returned
// alongside (execution proceeds without them — a rotated key file must not
// take down every scheduled script).
func (s *SecretService) EnvPairs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, value_encrypted FROM secrets ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying secrets: %w", err)
	}
	defer rows.Close()
	var out []string
	var skipped string
	for rows.Next() {
		var name string
		var blob []byte
		if err := rows.Scan(&name, &blob); err != nil {
			return nil, fmt.Errorf("scanning secret row: %w", err)
		}
		value, err := s.decrypt(blob)
		if err != nil {
			skipped += " " + name
			continue
		}
		out = append(out, name+"="+value)
	}
	if skipped != "" {
		return out, fmt.Errorf("undecryptable secrets skipped:%s", skipped)
	}
	return out, rows.Err()
}
