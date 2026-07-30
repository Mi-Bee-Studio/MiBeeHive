const API_BASE = '/api/v1';
var _handling401 = false;

function _sleep(ms) {
  return new Promise(function(resolve) { setTimeout(resolve, ms); });
}

const Api = {
  async get(url, options) { return this._request('GET', url, null, options); },
  async post(url, body, options) { return this._request('POST', url, body, options); },
  async put(url, body, options) { return this._request('PUT', url, body, options); },
  async delete(url, options) { return this._request('DELETE', url, null, options); },

  // getWithHeaders performs a GET and returns {data, headers} for reading response headers.
  // Supports options param with cache/cacheTtl like get().
  async getWithHeaders(url, options) {
    // ---- Cache check for GET requests ----
    var cacheKey = null;
    if (options && options.cache === true) {
      cacheKey = 'GET:' + url;
      var cached = Cache.get(cacheKey);
      if (cached !== null) {
        return cached;
      }
    }

    const token = Auth.getToken();
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;
    var lastErr = null;
    for (var attempt = 0; attempt <= 2; attempt++) {
      try {
        const res = await fetch(API_BASE + url, { method: 'GET', headers });
        if (res.status === 503 && attempt < 2) {
          await _sleep(attempt === 0 ? 1000 : 2000);
          continue;
        }
        if (res.status === 401) {
          return this._handle401(res, 'GET', url, null, true);
        }
        let data;
        try {
          data = await res.json();
        } catch (_jsonErr) {
          throw new Error(t('error_invalid_response'));
        }
        _handling401 = false;
        var result = { data, total: parseInt(res.headers.get('X-Total-Count') || '0', 10) };
        // ---- Cache successful GET responses ----
        if (options && options.cache === true && cacheKey) {
          Cache.set(cacheKey, result, options.cacheTtl || 30000);
        }
        return result;
      } catch (err) {
        lastErr = err;
        if (attempt < 2) {
          await _sleep(attempt === 0 ? 1000 : 2000);
          continue;
        }
      }
    }
    // All retries exhausted
    if (typeof Components !== "undefined" && Components.showToast) Components.showToast(lastErr ? lastErr.message || lastErr : t('network_error'), 'error');
    return null;
  },
  async patch(url, body, options) { return this._request('PATCH', url, body, options); },

  async _request(method, url, body, options = {}) {
    // ---- Cache check for GET requests ----
    var cacheKey = null;
    if (method === 'GET' && options.cache === true) {
      cacheKey = method + ':' + url;
      var cached = Cache.get(cacheKey);
      if (cached !== null) {
        return cached;
      }
    }

    const token = Auth.getToken();
    const headers = { 'Content-Type': 'application/json' };
    if (token) headers['Authorization'] = `Bearer ${token}`;

    // Loading bar control (skip for silent requests)
    const loadingBar = options.silent ? null : document.getElementById('global-loading-bar');
    if (loadingBar) {
      loadingBar.classList.add('active');
    }

    const isRetryable = method === 'GET';
    var lastErr = null;

    try {
      // AbortSignal passthrough: callers pass { signal } (e.g. from
      // Router.getSignal()) so in-flight requests are cancelled on route change.
      const fetchOpts = { method, headers, body: body ? JSON.stringify(body) : undefined };
      if (options.signal) fetchOpts.signal = options.signal;
      for (var attempt = 0; attempt <= (isRetryable ? 2 : 0); attempt++) {
        try {
          const res = await fetch(API_BASE + url, fetchOpts);

          // 503 Service Unavailable — retry for GET
          if (res.status === 503 && attempt < 2 && isRetryable) {
            await _sleep(attempt === 0 ? 1000 : 2000);
            continue;
          }

          if (res.status === 401) {
            _handling401 = false;
            return this._handle401(res, method, url, body, false);
          }

          let data;
          try {
            data = await res.json();
          } catch (_jsonErr) {
            throw new Error(t('error_invalid_response'));
          }

          // ---- Cache successful GET responses ----
          if (method === 'GET' && options.cache === true && cacheKey) {
            Cache.set(cacheKey, data, options.cacheTtl || 30000);
          }

          _handling401 = false;
          return data;
        } catch (err) {
          lastErr = err;
          if (isRetryable && attempt < 2) {
            await _sleep(attempt === 0 ? 1000 : 2000);
            continue;
          }
          throw err;
        }
      }
    } catch (err) {
      // AbortError is expected when a route change cancels in-flight requests;
      // do not treat it as a network failure.
      if (err && err.name === 'AbortError') {
        return { success: false, data: null, message: 'aborted', aborted: true };
      }
      if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('network_error') + ': ' + (err.message || err), 'error');
      return { success: false, data: null, message: err.message };
    } finally {
      if (loadingBar) {
        loadingBar.classList.remove('active');
        setTimeout(function() { loadingBar.classList.remove('done'); }, 300);
      }
    }
  }
,
  async _handle401(res, method, url, body, returnHeaders) {
    let errorCode = null;
    try {
      const cloned = res.clone();
      const parsed = await cloned.json();
      errorCode = parsed.error_code;
    } catch (e) {
      // Response body not parseable
    }

    if (_handling401) return null;
    _handling401 = true;

    if (errorCode === 'TOKEN_EXPIRED') {
      try {
        await Auth.refreshToken();
        // Retry original request with fresh token using raw fetch
        const token = Auth.getToken();
        const retryHeaders = { 'Content-Type': 'application/json' };
        if (token) retryHeaders['Authorization'] = 'Bearer ' + token;
        const retryRes = await fetch(API_BASE + url, { method, headers: retryHeaders, body: body ? JSON.stringify(body) : undefined });
        _handling401 = false;

        if (retryRes.status === 401) {
          // Retry also failed — force re-login
          Auth.logout();
          if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('auth_expired'), 'error');
          return null;
        }

        let retryData;
        try {
          retryData = await retryRes.json();
        } catch (_jsonErr) {
          _handling401 = false;
          Auth.logout();
          if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('error_invalid_response'), 'error');
          return null;
        }
        if (returnHeaders) {
          return { data: retryData, total: parseInt(retryRes.headers.get('X-Total-Count') || '0', 10) };
        }
        return retryData;
      } catch (e) {
        // Refresh failed
        _handling401 = false;
        Auth.logout();
        if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('auth_refresh_failed'), 'error');
        return null;
      }
    }

    _handling401 = false;

    if (errorCode === 'PASSWORD_CHANGED') {
      if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('auth_password_changed'), 'error');
      Auth.logout();
      return null;
    }

    // UNAUTHORIZED or unknown code — silent redirect
    Auth.logout();
    return null;
  }
};

// Global loading bar utilities
function showLoading() {
  const loadingBar = document.getElementById('global-loading-bar');
  if (loadingBar) loadingBar.classList.add('active');
}

function hideLoading() {
  const loadingBar = document.getElementById('global-loading-bar');
  if (loadingBar) {
    loadingBar.classList.remove('active');
    setTimeout(() => loadingBar.classList.remove('done'), 300);
  }
}

// Clean up loading bar on navigation events
window.addEventListener('hashchange', hideLoading);
window.addEventListener('beforeunload', hideLoading);