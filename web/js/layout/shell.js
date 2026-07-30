// Module: layout/shell — App shell composing sidebar + bottom-tab + content (Preact)
var Shell = (function () {
  'use strict';

  var html = PreactBridge.html;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;

  // ── Route title maps ──────────────────────────────────────────────

  var subPageTitleMap = {
    '/files/projects/:id': 'project_detail',
    '/files/queue': 'file_queue',
    '/files/crawl': 'file_crawl',
    '/deploy/configs': 'deploy_configs',
    '/deploy/iso': 'deploy_iso',
    '/share/files': 'share_files',
    '/share/settings': 'share_settings',
    '/containers/images': 'nav_containers_images',
    '/containers/templates': 'nav_containers_templates',
    '/containers/:id': 'nav_containers_detail',
  };

  var documentTitleMap = {
    '/login': 'title_login',
    '/dashboard': 'title_dashboard',
    '/files': 'title_files',
    '/deploy': 'title_deploy',
    '/share': 'title_share',
    '/settings': 'title_settings',
    '/containers': 'title_containers',
    '/system-status': 'title_system_status',
    '/search': 'title_search',
  };

  function _getPageTitle(routeInfo) {
    if (!routeInfo || !routeInfo.routeDef) return '';
    var pattern = routeInfo.routeDef.pattern || '';

    if (subPageTitleMap[pattern]) return t(subPageTitleMap[pattern]);

    var parts = pattern.split('/').filter(Boolean);
    if (parts.length > 0) {
      var i18nKey = 'nav_' + parts[0];
      var translated = t(i18nKey);
      if (translated !== i18nKey) return translated;
    }

    var segments = pattern.split('/').filter(Boolean);
    if (segments.length > 0) {
      var fallbackKey = 'nav_' + segments[segments.length - 1];
      var fallbackTranslated = t(fallbackKey);
      if (fallbackTranslated !== fallbackKey) return fallbackTranslated;
      return segments[segments.length - 1].replace(/[_-]/g, ' ').replace(/\b\w/g, function (c) {
        return c.toUpperCase();
      });
    }

    return '';
  }

  function _updateDocumentTitle(routeInfo) {
    if (!routeInfo || !routeInfo.routeDef) return;
    var pattern = routeInfo.routeDef.pattern || '';
    var firstSegment = '/' + pattern.split('/').filter(Boolean)[0];
    var key = documentTitleMap[firstSegment] || documentTitleMap[pattern];
    if (key) {
      document.title = t(key);
    } else {
      document.title = t('app_name');
    }
  }

  // ── Preact MobileTopBar wrapper ───────────────────────────────────

  function MobileTopBarWrapper() {
    var _t = useState('');
    var title = _t[0];
    var setTitle = _t[1];

    var _v = useState(false);
    var visible = _v[0];
    var setVisible = _v[1];

    useEffect(function () {
      function onRouteChange(routeInfo) {
        var hash = window.location.hash || '#/overview';
        var isSub = BottomTab.isSubPage(hash);
        if (isSub) {
          setTitle(_getPageTitle(routeInfo));
          setVisible(true);
        } else {
          setVisible(false);
        }
      }
      App.on('route:change', onRouteChange);
      return function () { App.off('route:change', onRouteChange); };
    }, []);

    function handleBack() {
      window.history.back();
    }

    if (!visible) {
      return html`<div style=${{ display: 'none' }}></div>`;
    }

    return html`
      <div class="mobile-top-bar">
        <button class="mobile-top-bar-back"
                aria-label=${t('common_back')}
                onClick=${handleBack}>
          <svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20"
               fill="none" stroke="currentColor" stroke-width="1.5"
               stroke-linecap="round" stroke-linejoin="round" width="18" height="18">
            <polyline points="12,4 6,10 12,16"/>
          </svg>
          <span>${t('common_back')}</span>
        </button>
        <span class="mobile-top-bar-title">${title}</span>
      </div>
    `;
  }

  // ── Preact BottomTabVisibility wrapper ─────────────────────────────

  function BottomTabVisibility() {
    var _v = useState(true);
    var visible = _v[0];
    var setVisible = _v[1];

    useEffect(function () {
      function onRouteChange() {
        var hash = window.location.hash || '#/overview';
        setVisible(!BottomTab.isSubPage(hash));
      }
      App.on('route:change', onRouteChange);
      return function () { App.off('route:change', onRouteChange); };
    }, []);

    return html`
      <nav style=${{ display: visible ? '' : 'none' }}>
        <${BottomTab.BottomTabComponent} />
      </nav>
    `;
  }

  // ── init() — mount Preact components into existing containers ─────

  // Wrap a component tree with AppProvider + I18nProvider so the shell
  // components (sidebar / bottom-tab / mobile-top-bar) re-render reactively
  // on theme / language / auth state changes. Each mount point gets its own
  // provider subtree; they all share the same App singleton + i18n globals.
  function withProviders(component) {
    var h = PreactBridge.h;
    var tree = component;
    if (typeof window.AppProvider === 'function') {
      tree = h(window.AppProvider, null, tree);
    }
    if (typeof window.I18nProvider === 'function') {
      tree = h(window.I18nProvider, null, tree);
    }
    return tree;
  }

  function init() {
    var appShell = document.getElementById('app-shell');
    if (!appShell) return;

    var sidebarEl = document.getElementById('sidebar');
    var bottomTabEl = document.getElementById('bottom-tab');
    var mobileTopBarEl = document.getElementById('mobile-top-bar');

    // Mount sidebar component into #sidebar
    if (sidebarEl) {
      PreactBridge.render(withProviders(PreactBridge.h(Sidebar.SidebarComponent)), sidebarEl);
    }

    // Initialize global search
    if (typeof GlobalSearch !== 'undefined') GlobalSearch.init();

    // Mount bottom tab component into #bottom-tab
    if (bottomTabEl) {
      PreactBridge.render(withProviders(PreactBridge.h(BottomTabVisibility)), bottomTabEl);
    }

    // Mount mobile top bar component into #mobile-top-bar
    if (mobileTopBarEl) {
      PreactBridge.render(withProviders(PreactBridge.h(MobileTopBarWrapper)), mobileTopBarEl);
    }

    // Listen for route changes to update document title
    Router.onRouteChange(function (routeInfo) {
      _updateDocumentTitle(routeInfo);
    });

    // Initial route resolve
    Router.resolve();
  }

  // ── Public API (backward compatible) ──────────────────────────────

  function updateLayout(routeInfo) {
    // No-op: Preact components handle layout updates reactively
  }

  function updatePageTitle(routeInfo) {
    _updateDocumentTitle(routeInfo);
  }

  return {
    init: init,
    updateLayout: updateLayout,
    updatePageTitle: updatePageTitle,
  };
})();
