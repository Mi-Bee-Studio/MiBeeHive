-- ISO catalog for auto-discovery of popular Linux distro ISOs
CREATE TABLE IF NOT EXISTS iso_catalog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    distro TEXT NOT NULL,
    variant TEXT NOT NULL DEFAULT '',
    arch TEXT NOT NULL DEFAULT 'amd64',
    check_url TEXT NOT NULL,
    filename_pattern TEXT NOT NULL,
    current_url TEXT DEFAULT '',
    auto_update INTEGER DEFAULT 0,
    check_interval_hours INTEGER DEFAULT 24,
    last_checked DATETIME,
    last_error TEXT DEFAULT '',
    status TEXT DEFAULT 'available',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);