-- 030_secrets.sql
-- Named secret store (#70 follow-up): tokens managed in the web UI, stored
-- encrypted-at-rest, injected as environment variables into every scheduled
-- script execution (one-time write-only input; values are never returned by
-- any API).
--
-- value_encrypted = AES-256-GCM(nonce || ciphertext+tag), key in
-- <data-dir>/.secret.key (0600, generated on first use). A DB backup alone
-- cannot reveal values; losing the key file means re-entering secrets.

CREATE TABLE IF NOT EXISTS secrets (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,
    value_encrypted BLOB NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);
