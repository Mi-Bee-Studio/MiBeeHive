/**
 * Login page — Preact + HTM via PreactBridge.
 * Drop-in replacement with same API: window.Login = { render, cleanup }.
 */
const Login = (function () {
  'use strict';

  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;

  var SUN_SVG = '<svg aria-hidden="true" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="12" cy="12" r="5"/><path d="M12 1v2M12 21v2M4.22 4.22l1.42 1.42M18.36 18.36l1.42 1.42M1 12h2M21 12h2M4.22 19.78l1.42-1.42M18.36 5.64l1.42-1.42"/></svg>';
  var MOON_SVG = '<svg aria-hidden="true" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z"/></svg>';
  var SYSTEM_SVG = '<svg aria-hidden="true" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>';
  var SPINNER_SVG = '<svg aria-hidden="true" class="login-spinner" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M12 2a10 10 0 010 20" stroke-linecap="round"/></svg>';

  // ── Shake animation (DOM-imperative, triggered on error) ──────────────

  function shakeCard() {
    var card = document.querySelector('.login-card');
    if (!card) return;
    card.classList.add('login-shake');
    card.addEventListener('animationend', function handler() {
      card.classList.remove('login-shake');
      card.removeEventListener('animationend', handler);
    });
  }

  // ── LoginForm Preact Component ────────────────────────────────────────

  function LoginForm() {
    var _pw = useState('');
    var password = _pw[0], setPassword = _pw[1];
    var _err = useState('');
    var error = _err[0], setError = _err[1];
    var _load = useState(false);
    var loading = _load[0], setLoading = _load[1];
    var _showPw = useState(false);
    var showPw = _showPw[0], setShowPw = _showPw[1];
    var _lang = useState(localStorage.getItem('mibeehive_lang') || 'zh');
    var lang = _lang[0], setLangState = _lang[1];
    var _theme = useState(window._mibeeTheme || 'system');
    var theme = _theme[0], setThemeState = _theme[1];
    var pwRef = useRef(null);

    // Hide chrome on mount, restore on unmount
    useEffect(function () {
      var sidebar = document.getElementById('sidebar');
      if (sidebar) sidebar.style.display = 'none';
      var bottomTab = document.getElementById('bottom-tab');
      if (bottomTab) bottomTab.style.display = 'none';
      var mobileTopBar = document.getElementById('mobile-top-bar');
      if (mobileTopBar) mobileTopBar.style.display = 'none';
      var appShell = document.getElementById('app-shell');
      if (appShell) appShell.style.gridTemplateColumns = '1fr';

      return function () {
        var sb = document.getElementById('sidebar');
        if (sb) sb.style.display = '';
        var bt = document.getElementById('bottom-tab');
        if (bt) bt.style.display = '';
        var mtb = document.getElementById('mobile-top-bar');
        if (mtb) mtb.style.display = '';
        var as = document.getElementById('app-shell');
        if (as) as.style.gridTemplateColumns = '';
      };
    }, []);

    var handleSubmit = useCallback(function (e) {
      e.preventDefault();
      if (!password.trim()) {
        setError(t('login_error_general'));
        if (pwRef.current) pwRef.current.focus();
        return;
      }
      setLoading(true);
      setError('');

      Auth.login(password).then(function () {
        var sidebar = document.getElementById('sidebar');
        if (sidebar) sidebar.style.display = '';
        var bottomTab = document.getElementById('bottom-tab');
        if (bottomTab) bottomTab.style.display = '';
        var mobileTopBar = document.getElementById('mobile-top-bar');
        if (mobileTopBar) mobileTopBar.style.display = '';
        var appShell = document.getElementById('app-shell');
        if (appShell) appShell.style.gridTemplateColumns = '';

        Api.get('/auth/password-status').then(function (resp) {
          if (resp && resp.success && resp.data && resp.data.is_default) {
            Components.showToast(t('password_default_warning'), 'warning');
            window.location.hash = '#/settings';
          } else {
            window.location.hash = '#/dashboard';
            Components.showToast(t('login_success'), 'success');
          }
        }).catch(function () {
          window.location.hash = '#/dashboard';
        });
      }).catch(function (err) {
        if (err.requirePasswordChange) {
          Components.showToast(t('password_change_required'), 'warning');
          window.location.hash = '#/settings';
          return;
        }
        setError(err.message || t('login_error_invalid'));
        shakeCard();
        setLoading(false);
        if (pwRef.current) {
          pwRef.current.focus();
          pwRef.current.select();
        }
      });
    }, [password]);

    var toggleLang = useCallback(function () {
      var next = lang === 'zh' ? 'en' : 'zh';
      setLang(next);
      setLangState(next);
    }, [lang]);

    var toggleTheme = useCallback(function () {
      var next = theme === 'light' ? 'dark' : theme === 'dark' ? 'system' : 'light';
      window._mibeeTheme = next;
      localStorage.setItem('mibeehive_theme', next);
      window._applyTheme(next);
      setThemeState(next);
      if (typeof window._updateThemeIcon === 'function') {
        window._updateThemeIcon();
      }
    }, [theme]);

    var themeIcon = theme === 'dark' ? MOON_SVG : theme === 'light' ? SUN_SVG : SYSTEM_SVG;
    var langHtml = '<span style="font-weight:var(--font-weight-medium)">' +
      (lang === 'zh' ? 'EN' : '\u4E2D') +
      '</span>' +
      '<span style="color:var(--color-text-quaternary);margin:0 0.25rem">|</span>' +
      '<span style="color:var(--color-text-tertiary)">' +
      (lang === 'zh' ? '\u4E2D' : 'EN') +
      '</span>';

    return html`
      <div class="login-bg">
        <div class="login-card anim-fade-in-scale">

          <div class="login-brand">
            <h1 class="login-brand-name gradient-text">MiBeeHive</h1>
            <p class="login-brand-subtitle">${t('login_subtitle')}</p>
          </div>

          <form id="login-form" class="login-form" autocomplete="on"
                onSubmit=${handleSubmit}>
            <div class="login-field">
              <label for="login-password" class="login-label">${t('password')}</label>
              <div class="login-input-wrap">
                <input ref=${pwRef} id="login-password"
                       type=${showPw ? 'text' : 'password'}
                       class="input login-input"
                       placeholder=${t('password')}
                       autocomplete="current-password" required
                       disabled=${loading}
                       value=${password}
                       onInput=${function (e) { setPassword(e.target.value); }} />
                <button type="button" class="password-toggle" tabindex="0"
                        aria-label="Toggle password visibility"
                        disabled=${loading}
                        onClick=${function () { setShowPw(!showPw); }}>
                  <span dangerouslySetInnerHTML=${{
                    __html: (showPw ? Helpers.ICONS.eyeOff : Helpers.ICONS.eye)
                  }} />
                </button>
              </div>
            </div>

            <button type="submit" id="login-btn" class="btn btn-primary login-btn"
                    disabled=${loading}>
              ${loading
                ? html`<span dangerouslySetInnerHTML=${{ __html: SPINNER_SVG }} />`
                : html`<span class="login-btn-text">${t('login_btn')}</span>`}
            </button>

            ${error
              ? html`<div id="login-error" class="login-error">${error}</div>`
              : html`<div id="login-error" class="login-error hidden"></div>`}
          </form>

        </div>

        <div class="login-bottom">
          <button class="login-toggle-btn" type="button"
                  onClick=${toggleLang}
                  dangerouslySetInnerHTML=${{ __html: langHtml }} />
          <button class="login-toggle-btn" type="button"
                  aria-label="Toggle theme"
                  onClick=${toggleTheme}
                  title=${t('theme_' + theme)}
                  dangerouslySetInnerHTML=${{ __html: themeIcon }} />
        </div>
      </div>`;
  }

  // ── Public API ────────────────────────────────────────────────────────

  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${LoginForm} />`, app);
  }

  function cleanup() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return {
    render: render,
    cleanup: cleanup,
  };
})();
