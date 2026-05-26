-- 011_containers.sql
-- Container management and application templates tables.

CREATE TABLE IF NOT EXISTS container_apps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    image TEXT NOT NULL,
    command TEXT DEFAULT '',
    env TEXT DEFAULT '{}',
    ports TEXT DEFAULT '[]',
    volumes TEXT DEFAULT '[]',
    networks TEXT DEFAULT '[]',
    restart_policy TEXT DEFAULT 'unless-stopped',
    memory_limit TEXT DEFAULT '',
    cpu_limit REAL DEFAULT 0,
    status TEXT DEFAULT 'stopped',
    container_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    image TEXT NOT NULL,
    command TEXT DEFAULT '',
    env TEXT DEFAULT '{}',
    ports TEXT DEFAULT '[]',
    volumes TEXT DEFAULT '[]',
    networks TEXT DEFAULT '[]',
    restart_policy TEXT DEFAULT 'unless-stopped',
    category TEXT DEFAULT 'general',
    icon TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
