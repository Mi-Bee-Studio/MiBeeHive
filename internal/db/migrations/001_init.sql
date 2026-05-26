-- MiBeeHive initial schema

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK(source_type IN ('github','go','hashicorp','grafana')),
    source_url TEXT NOT NULL,
    config JSON NOT NULL DEFAULT '{}',
    latest_version TEXT DEFAULT '',
    last_crawled_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    version TEXT NOT NULL,
    filename TEXT NOT NULL,
    os TEXT DEFAULT '',
    arch TEXT DEFAULT '',
    ext TEXT DEFAULT '',
    size_bytes INTEGER DEFAULT 0,
    download_url TEXT NOT NULL,
    local_path TEXT NOT NULL,
    checksum TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','downloading','complete','error','imported')),
    error_message TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS crawl_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    status TEXT NOT NULL CHECK(status IN ('running','success','error','rate_limited')),
    versions_found INTEGER DEFAULT 0,
    files_downloaded INTEGER DEFAULT 0,
    error_message TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS os_install_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    config JSON NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);
CREATE INDEX IF NOT EXISTS idx_files_filename ON files(filename);
CREATE INDEX IF NOT EXISTS idx_crawl_logs_project_id ON crawl_logs(project_id);
