package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
)

// base58Alphabet is the Bitcoin-style base58 alphabet, omitting the ambiguous
// characters 0 (zero), O (capital o), I (capital i), and l (lowercase L).
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// publicTokenLength is the number of base58 characters used for a public_token.
// 22 base58 chars ≈ 128 bits of entropy (log2(58^22) ≈ 128.7).
const publicTokenLength = 22

// maxTokenRetries is the maximum number of UNIQUE-collision retries before
// GeneratePublicToken gives up and returns an error.
const maxTokenRetries = 5

// backfillBatchSize is the number of files processed per backfill transaction.
const backfillBatchSize = 500

// DBChecker reports whether a public_token already exists in the files table.
// It is implemented by sqlTokenChecker and mocked in tests.
type DBChecker interface {
	TokenExists(ctx context.Context, token string) (bool, error)
}

// sqlTokenChecker checks token uniqueness against a *sql.DB.
type sqlTokenChecker struct {
	db *sql.DB
}

// TokenExists reports whether the given public_token is already present.
func (c *sqlTokenChecker) TokenExists(ctx context.Context, token string) (bool, error) {
	var one int
	err := c.db.QueryRowContext(ctx,
		"SELECT 1 FROM files WHERE public_token = ?", token).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("checking public_token uniqueness: %w", err)
	}
	return true, nil
}

// GeneratePublicToken generates a cryptographically random public_token using
// base58 encoding, 22 characters (~128 bits of entropy). It checks the database
// for UNIQUE collisions and retries up to maxTokenRetries times.
func GeneratePublicToken(db DBChecker) (string, error) {
	for attempt := 0; attempt < maxTokenRetries; attempt++ {
		token, err := randomToken()
		if err != nil {
			return "", err
		}
		exists, err := db.TokenExists(context.Background(), token)
		if err != nil {
			return "", err
		}
		if !exists {
			return token, nil
		}
	}
	return "", fmt.Errorf("failed to generate unique public_token after %d attempts", maxTokenRetries)
}

// randomToken reads 16 bytes from crypto/rand, encodes them to base58, and
// returns the first publicTokenLength characters.
func randomToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	encoded := base58Encode(buf)
	if len(encoded) > publicTokenLength {
		encoded = encoded[:publicTokenLength]
	}
	return encoded, nil
}

// base58Encode encodes a byte slice using the Bitcoin-style base58 alphabet.
func base58Encode(input []byte) string {
	// Count leading zero bytes (they map to leading '1' characters).
	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}

	// Allocate enough space for the base58 representation.
	size := (len(input)-zeros)*138/100 + 1
	buffer := make([]byte, size)

	// Repeatedly divide the big-endian number by 58.
	for _, b := range input[zeros:] {
		carry := int(b)
		for i := size - 1; i >= 0; i-- {
			carry += int(buffer[i]) << 8
			buffer[i] = byte(carry % 58)
			carry /= 58
		}
	}

	// Skip leading zeros in the buffer.
	i := 0
	for i < size && buffer[i] == 0 {
		i++
	}

	// Build the result string.
	result := make([]byte, zeros+size-i)
	for j := 0; j < zeros; j++ {
		result[j] = base58Alphabet[0]
	}
	for j := i; j < size; j++ {
		result[j-i+zeros] = base58Alphabet[buffer[j]]
	}
	return string(result)
}

// BackfillPublicTokens scans files with a NULL public_token and generates a
// token for each. It runs in batches of backfillBatchSize with individual
// UPDATE statements inside a single transaction per batch, looping until no
// NULL rows remain. Writes must go through the writeDB pool.
func BackfillPublicTokens(db *sql.DB) error {
	checker := &sqlTokenChecker{db: db}
	for {
		rows, err := db.Query(
			"SELECT id FROM files WHERE public_token IS NULL LIMIT ?", backfillBatchSize)
		if err != nil {
			return fmt.Errorf("querying files with NULL public_token: %w", err)
		}

		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return fmt.Errorf("scanning file id: %w", err)
			}
			ids = append(ids, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterating files with NULL public_token: %w", err)
		}

		if len(ids) == 0 {
			return nil
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("beginning backfill transaction: %w", err)
		}

		// Track tokens generated within this batch so uncommitted rows (not yet
		// visible to the checker) cannot collide with each other.
		used := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			token, err := GeneratePublicToken(checker)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("generating public_token for file %d: %w", id, err)
			}
			for {
				if _, dup := used[token]; !dup {
					break
				}
				token, err = GeneratePublicToken(checker)
				if err != nil {
					tx.Rollback()
					return fmt.Errorf("generating public_token for file %d: %w", id, err)
				}
			}
			used[token] = struct{}{}

			if _, err := tx.Exec(
				"UPDATE files SET public_token = ? WHERE id = ?", token, id); err != nil {
				tx.Rollback()
				return fmt.Errorf("updating public_token for file %d: %w", id, err)
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing backfill transaction: %w", err)
		}
		slog.Info("public_token backfill batch complete", "updated", len(ids))
	}
}