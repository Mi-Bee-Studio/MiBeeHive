/**
 * Shared Preact hooks — polling lifecycle tied to AbortSignal + scoped timers.
 *
 * Exposes window.Hooks with:
 *   - usePolling(fn, intervalMs, options?) : runs `fn` immediately and every
 *     intervalMs, auto-stops when `options.signal` aborts (e.g. route change)
 *     or the component unmounts. Timers are registered under `options.scope`
 *     so App.clearScope() can clean them up without nuking unrelated polls.
 *
 * Loaded after state.js (needs window.App) and preact.js (needs PreactBridge).
 */
(function () {
  'use strict';

  var PB = window.PreactBridge;
  var useEffect = PB.useEffect;
  var useRef = PB.useRef;

  /**
   * @param {Function} fn           async or sync function to run
   * @param {number}   intervalMs   polling interval; <=0 runs once
   * @param {{signal?:AbortSignal, scope?:string, immediate?:boolean}} [options]
   */
  function usePolling(fn, intervalMs, options) {
    options = options || {};
    var signal = options.signal || null;
    var scope = options.scope || '';
    var immediate = options.immediate !== false;
    var fnRef = useRef(fn);
    fnRef.current = fn;

    useEffect(function () {
      var cancelled = false;
      var timerId = null;

      function run() {
        if (cancelled) return;
        if (signal && signal.aborted) { stop(); return; }
        try {
          var ret = fnRef.current();
          if (ret && typeof ret.catch === 'function') {
            ret.catch(function (e) {
              if (e && e.name === 'AbortError') return; // expected on route change
              console.error('[usePolling] error:', e);
            });
          }
        } catch (e) {
          console.error('[usePolling] error:', e);
        }
      }

      function stop() {
        cancelled = true;
        if (timerId) {
          clearInterval(timerId);
          if (window.App) {
            // Best-effort remove from scope bucket; clearScope handles cleanup
            // wholesale on route change, so this is only for unmount-within-route.
          }
          timerId = null;
        }
      }

      if (immediate) run();
      if (intervalMs && intervalMs > 0) {
        timerId = setInterval(run, intervalMs);
        if (window.App && scope) window.App.addTimer(timerId, scope);
      }

      // Stop when the route's abort signal fires.
      function onAbort() { stop(); }
      if (signal) {
        if (signal.aborted) { stop(); return stop; }
        signal.addEventListener('abort', onAbort);
      }

      return function cleanup() {
        cancelled = true;
        if (signal) signal.removeEventListener('abort', onAbort);
        if (timerId) clearInterval(timerId);
      };
    }, [intervalMs, options.scope, signal]);
  }

  window.Hooks = { usePolling: usePolling };
})();
