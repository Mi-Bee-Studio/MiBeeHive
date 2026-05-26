/**
 * Global App state with Preact Context integration
 *
 * Provides:
 * - window.App singleton (vanilla JS backward compat): state, event bus, timer management
 * - window.AppContext  — Preact context for component tree
 * - window.AppProvider — Preact component wrapping the app
 */
(function () {
  'use strict';

  var PB = window.PreactBridge;

  // ── Internal state ──────────────────────────────────────────────
  var _state = {
    user: null,
    token: localStorage.getItem('mibeehive_token'),
    theme: localStorage.getItem('theme') || 'system',
    lang: localStorage.getItem('lang') || 'zh',
    loading: false,
  };

  // ── Event bus ───────────────────────────────────────────────────
  var _listeners = {};

  // ── Timer management ────────────────────────────────────────────
  var _timers = [];

  // ── App singleton (backward compatible) ─────────────────────────
  var App = {
    state: _state,

    setState: function (key, value) {
      _state[key] = value;
      this.emit('state:' + key, value);
      this.emit('state:sync');
    },

    // Timer management
    addTimer: function (id) { _timers.push(id); },
    clearAllTimers: function () {
      _timers.forEach(function (id) {
        clearInterval(id);
        clearTimeout(id);
      });
      _timers = [];
    },

    // Event bus
    on: function (event, fn) {
      if (!_listeners[event]) _listeners[event] = [];
      _listeners[event].push(fn);
      return this;
    },
    off: function (event, fn) {
      if (!_listeners[event]) return this;
      if (!fn) { delete _listeners[event]; return this; }
      _listeners[event] = _listeners[event].filter(function (f) { return f !== fn; });
      return this;
    },
    emit: function (event, data) {
      var fns = _listeners[event];
      if (!fns || !fns.length) return;
      for (var i = 0; i < fns.length; i++) {
        try { fns[i](data); } catch (e) { console.error('[App] Event error:', e); }
      }
    },
  };

  // Expose cache singleton
  App.cache = typeof Cache !== 'undefined' ? Cache : null;

  // ── Preact Context ──────────────────────────────────────────────
  var AppContext = PB.createContext(_state);

  // ── AppProvider Preact component ────────────────────────────────
  // Wraps the app tree and syncs App singleton state into Preact context.
  // Usage: html`<${AppProvider}>...children...<//>`
  function AppProvider(props) {
    var _s = PB.useState(Object.assign({}, _state));
    var state = _s[0];
    var setState = _s[1];

    PB.useEffect(function () {
      function sync() { setState(Object.assign({}, _state)); }
      App.on('state:sync', sync);
      return function () { App.off('state:sync', sync); };
    }, []);

    return PB.h(AppContext.Provider, { value: state }, props.children);
  }

  // ── Expose globals ──────────────────────────────────────────────
  window.App = App;
  window.AppContext = AppContext;
  window.AppProvider = AppProvider;
})();
