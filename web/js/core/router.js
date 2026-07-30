/**
 * Hash-based SPA router with Preact-compatible features
 *
 * Features:
 * - Hash-based routing (#/path format)
 * - Route guards (auth check, redirect unauthenticated to #/login)
 * - Parameterized routes (:id, :subtab)
 * - AbortController integration (new signal per route change)
 * - App event bus integration (emits route:change)
 * - Route change cleanup via window._clearPageTimers
 *
 * Exposed as window.Router for backward compatibility.
 */
(function () {
  'use strict';

  // ── State ───────────────────────────────────────────────────────
  var current = null;       // { path, params, query, routeDef, signal }
  var changeCallbacks = [];
  var _abortController = null;

  // ── Helpers ─────────────────────────────────────────────────────
  function _getToken() {
    if (typeof Auth !== 'undefined' && typeof Auth.getToken === 'function') {
      return Auth.getToken();
    }
    return localStorage.getItem('mibeehive_token');
  }

  function _showToast(msg, type) {
    if (typeof Components !== 'undefined' && Components.showToast) {
      Components.showToast(msg, type);
    } else {
      console.error('[Router]', msg);
    }
  }

  function _t(key) {
    if (typeof t === 'function') return t(key);
    return key;
  }

  // ── Route definitions ───────────────────────────────────────────
  var routes = [
    { pattern: '/login',                          handler: function() { Login.render(); },                                          public: true  },
    { pattern: '/',                               handler: function() { Overview.render(); }                                        },
    { pattern: '/overview',                       handler: function() { Overview.render(); }                                        },
    { pattern: '/dashboard',                      handler: function() { window.location.hash = '#/overview'; }                      },
    { pattern: '/files',                          handler: function() { Files.render(); }                                           },
    { pattern: '/files/projects/:id',             handler: function(p) { FilesProjects.render(p.id); }                              },
    { pattern: '/files/queue',                    handler: function() { FilesQueue.render(); }                                      },
    { pattern: '/files/crawl',                    handler: function() { FilesCrawl.render(); }                                      },
    { pattern: '/deploy',                         handler: function() { Deploy.render(); }                                          },
    { pattern: '/deploy/configs',                 handler: function() { DeployConfigs.render(); }                                   },
    { pattern: '/deploy/iso',                     handler: function() { DeployISO.render(); }                                       },
    { pattern: '/system-status',                  handler: function() { SystemStatus.render(); }                                    },
    { pattern: '/system-status/logs',             handler: function() { SystemStatus.renderLogs(); }                                },
    { pattern: '/system-status/tasks',            handler: function() { SystemStatus.renderTasks(); }                               },
    { pattern: '/share',                          handler: function() { Share.render(); }                                           },
    { pattern: '/share/files',                    handler: function() { ShareFiles.render(); }                                      },
    { pattern: '/share/settings',                 handler: function() { Share.renderSettings(); }                                   },
    { pattern: '/supply',                         handler: function() { Supply.render(); }                                          },
    { pattern: '/containers/images',              handler: function() { ContainersImages.render(); }                                },
    { pattern: '/containers/templates',           handler: function() { ContainersTemplates.render(); }                              },
    { pattern: '/containers/registries',          handler: function() { Registries.render(); }                                      },
    { pattern: '/containers/registries/:subtab',  handler: function(p) { Registries.render(p.subtab); }                             },
    { pattern: '/containers/:id',                 handler: function(p) { ContainersDetail.render(p.id); }                           },
    { pattern: '/containers',                     handler: function() { Containers.render(); }                                      },
    // Legacy /registries* paths now live under /containers/registries; redirect
    // for backward compatibility with old bookmarks.
    { pattern: '/registries',                     handler: function() { window.location.hash = '#/containers/registries'; }         },
    { pattern: '/registries/:subtab',             handler: function(p) { window.location.hash = '#/containers/registries/' + (p.subtab || ''); } },
    { pattern: '/logs',                           handler: function() { window.location.hash = '#/system-status/logs'; }            },
    { pattern: '/tasks',                          handler: function() { window.location.hash = '#/system-status/tasks'; }           },
    { pattern: '/settings',                       handler: function() { Settings.render(); }                                        },
  ];

  // ── Compile routes ──────────────────────────────────────────────
  var compiledRoutes = routes.map(function (r) {
    var names = [];
    var regexStr = '^' + r.pattern.replace(/:(\w+)/g, function (_, name) {
      names.push(name);
      return '([^/]+)';
    }) + '$';
    return {
      pattern: r.pattern,
      regex: new RegExp(regexStr),
      names: names,
      handler: r.handler,
      public: !!r.public,
    };
  });

  // ── Parse hash into route match ─────────────────────────────────
  function parseRoute(hash) {
    var raw = (hash || '').replace(/^#/, '') || '/overview';
    var qIndex = raw.indexOf('?');
    var path = qIndex >= 0 ? raw.substring(0, qIndex) : raw;
    var queryString = qIndex >= 0 ? raw.substring(qIndex + 1) : '';
    var query = {};

    if (queryString) {
      queryString.split('&').forEach(function (pair) {
        var eqIdx = pair.indexOf('=');
        if (eqIdx > 0) {
          query[decodeURIComponent(pair.substring(0, eqIdx))] =
            decodeURIComponent(pair.substring(eqIdx + 1));
        } else if (pair) {
          query[decodeURIComponent(pair)] = '';
        }
      });
    }

    // Exact matches first (skip parameterized)
    for (var i = 0; i < compiledRoutes.length; i++) {
      var cr = compiledRoutes[i];
      if (cr.names.length > 0) continue;
      if (cr.pattern === path) {
        return { handler: cr.handler, params: {}, query: query, routeDef: cr };
      }
    }

    // Parameterized routes second
    for (var j = 0; j < compiledRoutes.length; j++) {
      var pr = compiledRoutes[j];
      if (pr.names.length === 0) continue;
      var match = path.match(pr.regex);
      if (match) {
        var params = {};
        pr.names.forEach(function (name, idx) {
          params[name] = match[idx + 1];
        });
        return { handler: pr.handler, params: params, query: query, routeDef: pr };
      }
    }

    return null;
  }

  // ── Cleanup previous route ──────────────────────────────────────
  function _cleanup() {
    // Abort pending requests from previous route
    if (_abortController) {
      _abortController.abort();
      _abortController = null;
    }
    // Clear only the previous route's scoped timers (polling registered via
    // Hooks.usePolling with { scope }). `current` still holds the previous
    // route at this point.
    if (window.App && current && current.path) {
      App.clearScope(current.path);
    }
  }

  // ── Create new AbortController ──────────────────────────────────
  function _createAbortController() {
    _abortController = new AbortController();
    return _abortController.signal;
  }

  // ── Auth guard ──────────────────────────────────────────────────
  function _checkAuth(parsed) {
    var hasToken = !!_getToken();
    if (!hasToken && parsed && parsed.routeDef && !parsed.routeDef.public) {
      window.location.hash = '#/login';
      return false;
    }
    if (hasToken && parsed && parsed.routeDef && parsed.routeDef.public && parsed.path === '/login') {
      window.location.hash = '#/overview';
      return false;
    }
    return true;
  }

  // ── 404 page ────────────────────────────────────────────────────
  function _render404() {
    var app = document.getElementById('main-content');
    if (!app) return;
    app.innerHTML =
      '<div class="empty-state" style="min-height:60vh">' +
        '<svg aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24" style="width:3rem;height:3rem;color:var(--color-text-tertiary)">' +
          '<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"/>' +
        '</svg>' +
        '<p class="text-lg font-medium" style="color:var(--color-text)">' + _t('not_found') + '</p>' +
        '<p class="text-sm mt-2" style="color:var(--color-text-secondary)">' + _t('not_found_desc') + '</p>' +
        '<button class="btn btn-primary mt-4" onclick="Router.push(\'/overview\')">' + _t('back_to_dashboard') + '</button>' +
      '</div>';
  }

  // ── Fire change callbacks ───────────────────────────────────────
  function _fireChangeCallbacks(routeInfo) {
    for (var i = 0; i < changeCallbacks.length; i++) {
      try {
        changeCallbacks[i](routeInfo);
      } catch (e) {
        console.error('[Router] Change callback error:', e);
      }
    }
  }

  // ── Notify App event bus ────────────────────────────────────────
  function _notifyApp(routeInfo) {
    if (window.App) {
      window.App.emit('route:change', routeInfo);
    }
  }

  // ── Resolve current hash ────────────────────────────────────────
  function resolve() {
    var parsed = parseRoute(window.location.hash);

    // No matching route → 404
    if (!parsed) {
      _cleanup();
      current = {
        path: window.location.hash.replace(/^#/, '') || '/overview',
        params: {},
        query: {},
        routeDef: null,
      };
      _render404();
      _fireChangeCallbacks(current);
      _notifyApp(current);
      return;
    }

    // Reconstruct canonical path from pattern + params
    parsed.path = parsed.routeDef.pattern.replace(/:(\w+)/g, function (m) {
      return parsed.params[m.substring(1)] || m;
    });

    // Auth guard
    if (!_checkAuth(parsed)) return;

    // Cleanup previous route
    _cleanup();

    window.scrollTo(0, 0);

    // Create new AbortController for this route
    var signal = _createAbortController();

    current = parsed;
    current.signal = signal;

    _fireChangeCallbacks(current);
    _notifyApp(current);

    // Execute route handler with params, query, and abort signal
    try {
      var result = parsed.handler(parsed.params, parsed.query, signal);
      if (result && typeof result.catch === 'function') {
        result.catch(function (err) {
          console.error('[Router] Handler error:', err);
          _showToast(_t('error') + ': ' + (err.message || err), 'error');
        });
      }
    } catch (err) {
      console.error('[Router] Handler error:', err);
      _showToast(_t('error') + ': ' + (err.message || err), 'error');
    }
  }

  // ── Public API ──────────────────────────────────────────────────
  var api = {
    /**
     * Navigate to a new route.
     * @param {string} path - Target path (e.g. '/files/projects/42')
     */
    push: function (path) {
      window.location.hash = '#' + path.replace(/^#/, '');
    },

    /**
     * Go back in browser history.
     */
    back: function () {
      window.history.back();
    },

    /**
     * Returns current route info: { path, params, query, routeDef, signal }
     * Returns null if no route has been resolved yet.
     */
    getCurrentRoute: function () {
      return current;
    },

    /**
     * Returns the current route's AbortSignal (for fetch cancellation).
     * @returns {AbortSignal|null}
     */
    getSignal: function () {
      return _abortController ? _abortController.signal : null;
    },

    /**
     * Register a callback fired on every route change.
     * Callback receives { path, params, query, routeDef, signal }.
     * Returns an unsubscribe function.
     * @param {Function} callback
     * @returns {Function} unsubscribe
     */
    onRouteChange: function (callback) {
      if (typeof callback !== 'function') {
        console.warn('[Router] onRouteChange expects a function');
        return function () {};
      }
      changeCallbacks.push(callback);
      return function () {
        var idx = changeCallbacks.indexOf(callback);
        if (idx > -1) changeCallbacks.splice(idx, 1);
      };
    },

    /**
     * Returns a flat list of all registered route patterns.
     */
    getRoutes: function () {
      return routes.map(function (r) { return r.pattern; });
    },

    /**
     * Manually trigger route resolution.
     */
    resolve: resolve,

    /**
     * Parse a hash string without triggering navigation.
     * @param {string} hash
     * @returns {{ handler, params, query, routeDef }|null}
     */
    parseRoute: parseRoute,
  };

  // ── Listen for hash changes ─────────────────────────────────────
  window.addEventListener('hashchange', resolve);

  // ── Expose global ───────────────────────────────────────────────
  window.Router = api;
})();
