// Core: cache — Simple in-memory API response cache with TTL
// Exposes singleton via global `Cache` and `App.cache`

const Cache = {
  _store: {},

  // Returns cached data if key exists and TTL not expired, null otherwise
  get(key) {
    var entry = this._store[key];
    if (!entry) return null;
    if (entry.expires && Date.now() > entry.expires) {
      delete this._store[key];
      return null;
    }
    return entry.data;
  },

  // Stores data with optional TTL in milliseconds
  set(key, data, ttlMs) {
    this._store[key] = {
      data: data,
      expires: ttlMs ? Date.now() + ttlMs : null,
    };
  },

  // Remove a specific cache entry by exact key
  invalidate(key) {
    delete this._store[key];
  },

  // Remove all cache entries whose key starts with the given prefix
  invalidatePattern(prefix) {
    var keys = Object.keys(this._store);
    for (var i = 0; i < keys.length; i++) {
      if (keys[i].indexOf(prefix) === 0) {
        delete this._store[keys[i]];
      }
    }
  },

  // Clear all cached entries
  clear() {
    this._store = {};
  },
};
