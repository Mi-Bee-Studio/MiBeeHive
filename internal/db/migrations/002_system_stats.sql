CREATE TABLE IF NOT EXISTS system_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sampled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cpu_usage_percent REAL NOT NULL,
    memory_total_bytes INTEGER NOT NULL,
    memory_used_bytes INTEGER NOT NULL,
    memory_usage_percent REAL NOT NULL,
    network_rx_bytes INTEGER NOT NULL,
    network_tx_bytes INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_system_stats_sampled ON system_stats(sampled_at);
