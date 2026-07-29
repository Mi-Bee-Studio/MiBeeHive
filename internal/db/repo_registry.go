package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

const registryColumns = `id, name, url, type, username, encrypted_password, enabled, created_at, updated_at`

// RegistryRepo provides CRUD operations for container registries.
type RegistryRepo struct {
	db            *sql.DB
	encryptionKey []byte
}

// NewRegistryRepo creates a new RegistryRepo with an encryption key for AES-256 password encryption.
func NewRegistryRepo(db *sql.DB, encryptionKey string) *RegistryRepo {
	key := fmt.Sprintf("%-32s", encryptionKey)[:32]
	return &RegistryRepo{db: db, encryptionKey: []byte(key)}
}

// List returns all registries ordered by name.
func (r *RegistryRepo) List(ctx context.Context) ([]model.Registry, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT "+registryColumns+" FROM registries ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("listing registries: %w", err)
	}
	defer rows.Close()

	var registries []model.Registry
	for rows.Next() {
		reg, err := scanRegistry(rows)
		if err != nil {
			return nil, err
		}
		registries = append(registries, reg)
	}
	return registries, rows.Err()
}

// GetByID retrieves a registry by its ID. Returns nil, nil if not found.
func (r *RegistryRepo) GetByID(ctx context.Context, id int64) (*model.Registry, error) {
	return r.getOne(ctx, "SELECT "+registryColumns+" FROM registries WHERE id = ?", id)
}

// GetByURL retrieves a registry by its URL. Returns nil, nil if not found.
func (r *RegistryRepo) GetByURL(ctx context.Context, url string) (*model.Registry, error) {
	return r.getOne(ctx, "SELECT "+registryColumns+" FROM registries WHERE url = ?", url)
}

// Create inserts a new registry with encrypted password and returns the generated ID.
func (r *RegistryRepo) Create(ctx context.Context, reg *model.Registry) (int64, error) {
	encPass, err := encryptPassword(reg.EncryptedPassword, r.encryptionKey)
	if err != nil {
		return 0, fmt.Errorf("encrypting password for registry %q: %w", reg.Name, err)
	}

	query := `INSERT INTO registries (name, url, type, username, encrypted_password, enabled)
	          VALUES (?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		reg.Name, reg.URL, string(reg.Type), reg.Username, encPass, reg.Enabled)
	if err != nil {
		return 0, fmt.Errorf("inserting registry %q: %w", reg.Name, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("getting last insert id: %w", err)
	}
	return id, nil
}

// Update updates a registry. If EncryptedPassword is non-empty, it is encrypted before storing.
func (r *RegistryRepo) Update(ctx context.Context, reg *model.Registry) error {
	var encPass string
	var err error
	if reg.EncryptedPassword != "" {
		encPass, err = encryptPassword(reg.EncryptedPassword, r.encryptionKey)
		if err != nil {
			return fmt.Errorf("encrypting password for registry %d: %w", reg.ID, err)
		}
	} else {
		// Keep existing password — fetch current.
		var current string
		if err := r.db.QueryRowContext(ctx,
			"SELECT encrypted_password FROM registries WHERE id = ?", reg.ID).Scan(&current); err != nil {
			return fmt.Errorf("fetching current password for registry %d: %w", reg.ID, err)
		}
		encPass = current
	}

	query := `UPDATE registries SET name = ?, url = ?, type = ?, username = ?,
	          encrypted_password = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err = r.db.ExecContext(ctx, query,
		reg.Name, reg.URL, string(reg.Type), reg.Username, encPass, reg.Enabled, reg.ID)
	if err != nil {
		return fmt.Errorf("updating registry %d: %w", reg.ID, err)
	}
	return nil
}

// Delete removes a registry by ID.
func (r *RegistryRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM registries WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("deleting registry %d: %w", id, err)
	}
	return nil
}

// DecryptPassword decrypts the stored encrypted password for a registry.
func (r *RegistryRepo) DecryptPassword(ctx context.Context, id int64) (string, error) {
	var encPass string
	err := r.db.QueryRowContext(ctx,
		"SELECT encrypted_password FROM registries WHERE id = ?", id).Scan(&encPass)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("fetching password for registry %d: %w", id, err)
	}
	if encPass == "" {
		return "", nil
	}
	return decryptPassword(encPass, r.encryptionKey)
}

func (r *RegistryRepo) getOne(ctx context.Context, query string, args ...any) (*model.Registry, error) {
	row := r.db.QueryRowContext(ctx, query, args...)
	reg, err := scanRegistry(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &reg, nil
}

func scanRegistry(s interface{ Scan(dest ...any) error }) (model.Registry, error) {
	var reg model.Registry
	var regType string
	err := s.Scan(
		&reg.ID, &reg.Name, &reg.URL, &regType, &reg.Username,
		&reg.EncryptedPassword, &reg.Enabled, &reg.CreatedAt, &reg.UpdatedAt,
	)
	if err != nil {
		return reg, fmt.Errorf("scanning registry: %w", err)
	}
	reg.Type = model.RegistryType(regType)
	return reg, nil
}

// encryptPassword encrypts a plaintext password using AES-GCM and returns hex-encoded nonce+ciphertext.
func encryptPassword(plaintext string, key []byte) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decryptPassword decrypts a hex-encoded AES-GCM encrypted password.
func decryptPassword(encoded string, key []byte) (string, error) {
	if encoded == "" {
		return "", nil
	}

	data, err := hex.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decoding hex: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting password: %w", err)
	}

	return string(plaintext), nil
}
