/**
 * Auth utilities — JWT login/logout/refresh
 *
 * Pure utility module (no Preact hooks).
 * Notifies App event bus on auth state changes.
 * Exposed as window.Auth for backward compatibility.
 */
(function () {
  'use strict';

  var Auth = {
    // ── Login ─────────────────────────────────────────────────────
    login: function (password) {
      var self = this;
      return Api.post('/auth/login', { password: password })
        .then(function (result) {
          if (result.success) {
            self.setToken(result.data.token);
            self.setUsername(result.data.username || 'admin');
            self.startTokenRefresh();
            self._notifyAuth(true);
            return true;
          } else if (result.data && result.data.token) {
            // Default password case: 409 with token still provided
            self.setToken(result.data.token);
            self.setUsername('admin');
            self.startTokenRefresh();
            self._notifyAuth(true);
            var err = new Error(result.message || 'Password change required');
            err.requirePasswordChange = true;
            throw err;
          } else {
            throw new Error(result.message || 'Login failed');
          }
        });
    },

    // ── Logout ────────────────────────────────────────────────────
    logout: function () {
      this.stopTokenRefresh();
      this.clearToken();
      this.clearUsername();
      this._notifyAuth(false);
      if (window.location.hash !== '#/login') {
        window.location.hash = '#/login';
      }
      // Clear timers
      if (window.App && window.App.clearAllTimers) {
        window.App.clearAllTimers();
      }
    },

    // ── Token management ──────────────────────────────────────────
    getToken: function () {
      return localStorage.getItem('mibeehive_token');
    },
    setToken: function (token) {
      localStorage.setItem('mibeehive_token', token);
    },
    clearToken: function () {
      localStorage.removeItem('mibeehive_token');
    },

    // ── Username management ───────────────────────────────────────
    getUsername: function () {
      return localStorage.getItem('username') || 'admin';
    },
    setUsername: function (username) {
      localStorage.setItem('username', username);
    },
    clearUsername: function () {
      localStorage.removeItem('username');
    },

    // ── Auth check ────────────────────────────────────────────────
    isAuthenticated: function () {
      return !!this.getToken();
    },

    // ── JWT helpers ───────────────────────────────────────────────
    _decodePayload: function (token) {
      try {
        var parts = token.split('.');
        if (parts.length !== 3) return null;
        return JSON.parse(atob(parts[1]));
      } catch (e) { return null; }
    },

    isTokenExpiringSoon: function (thresholdMs) {
      var token = this.getToken();
      if (!token) return false;
      var payload = this._decodePayload(token);
      if (!payload || !payload.exp) return false;
      var expiresAt = payload.exp * 1000;
      var threshold = thresholdMs || (60 * 60 * 1000);
      return Date.now() > (expiresAt - threshold);
    },

    // ── Token refresh ─────────────────────────────────────────────
    refreshToken: function () {
      var self = this;
      var token = this.getToken();
      if (!token) return Promise.reject(new Error('no token'));

      return fetch('/api/v1/auth/refresh', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer ' + token,
        },
      })
      .then(function (res) {
        if (!res.ok) throw new Error('refresh failed');
        return res.json();
      })
      .then(function (data) {
        if (data.success && data.data && data.data.token) {
          self.setToken(data.data.token);
          return data.data.token;
        }
        throw new Error(data.message || 'refresh failed');
      })
      .catch(function (err) {
        console.warn('Token refresh failed:', err.message);
        self.logout();
        throw err;
      });
    },

    // ── Periodic refresh ──────────────────────────────────────────
    _refreshTimer: null,

    startTokenRefresh: function () {
      var self = this;
      this.stopTokenRefresh();
      this._refreshTimer = setInterval(function () {
        if (self.isAuthenticated() && self.isTokenExpiringSoon()) {
          self.refreshToken().catch(function () {});
        }
      }, 30 * 60 * 1000);
      if (this.isAuthenticated() && this.isTokenExpiringSoon()) {
        this.refreshToken().catch(function () {});
      }
    },

    stopTokenRefresh: function () {
      if (this._refreshTimer) {
        clearInterval(this._refreshTimer);
        this._refreshTimer = null;
      }
    },

    // ── Auth state notification ───────────────────────────────────
    _notifyAuth: function (authenticated) {
      if (window.App) {
        window.App.emit('auth:change', { authenticated: authenticated });
        window.App.setState('token', authenticated ? this.getToken() : null);
      }
    },
  };

  // ── Expose global ───────────────────────────────────────────────
  window.Auth = Auth;
})();
