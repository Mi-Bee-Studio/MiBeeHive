// Module: layout/sidebar — Desktop sidebar navigation (Preact)
var Sidebar = (function () {
  'use strict';

  var html = PreactBridge.html;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;

  var STORAGE_KEY = 'sidebar-collapsed';

  var icons = {
    brand: '<svg aria-hidden="true" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2l8.5 5v10L12 22l-8.5-5V7z"/><circle cx="12" cy="12" r="2.5" fill="currentColor" stroke="none"/></svg>',
    dashboard: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="12" width="4" height="9" rx="1"/><rect x="10" y="7" width="4" height="14" rx="1"/><rect x="17" y="3" width="4" height="18" rx="1"/></svg>',
    files: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7v12a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-7l-2-2H5a2 2 0 00-2 2z"/></svg>',
    deploy: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="2" width="16" height="6" rx="1"/><rect x="4" y="10" width="16" height="6" rx="1"/><circle cx="8" cy="5" r="0.5" fill="currentColor" stroke="none"/><circle cx="8" cy="13" r="0.5" fill="currentColor" stroke="none"/><path d="M10 20h4"/><path d="M12 16v4"/></svg>',
    share: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12v8a2 2 0 002 2h12a2 2 0 002-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>',
    settings: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 01-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/></svg>',
    chevronLeft: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>',
    chevronRight: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>',
    user: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 00-4-4H8a4 4 0 00-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>',
    key: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 11-7.78 7.78 5.5 5.5 0 017.78-7.78zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>',
    logout: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>',
    containers: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/></svg>',
    search: '<svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>',
  };

  var navItems = [
    { route: '/system-status', hash: '#/system-status', i18nKey: 'nav_system_status', icon: icons.dashboard },
    { route: '/files',         hash: '#/files',         i18nKey: 'nav_files',         icon: icons.files },
    { route: '/deploy',        hash: '#/deploy',        i18nKey: 'nav_deploy',        icon: icons.deploy },
    { route: '/share',         hash: '#/share',         i18nKey: 'nav_share',         icon: icons.share },
    { route: '/containers',    hash: '#/containers',    i18nKey: 'nav_containers',    icon: icons.containers },
  ];
  var settingsItem = { route: '/settings', hash: '#/settings', i18nKey: 'nav_settings', icon: icons.settings };

  function isCollapsed() {
    return localStorage.getItem(STORAGE_KEY) === 'true';
  }

  function setCollapsed(collapsed) {
    localStorage.setItem(STORAGE_KEY, collapsed ? 'true' : 'false');
  }

  function getActiveRoute(hash) {
    var raw = (hash || window.location.hash || '').replace(/^#/, '') || '/system-status';
    var firstSegment = '/' + raw.split('/').filter(Boolean)[0];
    var allItems = navItems.concat(settingsItem);
    for (var i = 0; i < allItems.length; i++) {
      var item = allItems[i];
      if (raw === item.route || firstSegment === item.route) return item.route;
      if (item.route === '/system-status' && (raw === '/system-status' || raw === '/')) return '/system-status';
    }
    return null;
  }

  function escapeHtml(str) {
    if (typeof window.escapeHtml === 'function') return window.escapeHtml(str);
    var div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  }

  // ── Preact SidebarComponent ──────────────────────────────────────────

  function SidebarComponent() {
    var _c = useState(isCollapsed());
    var collapsed = _c[0];
    var setColl = _c[1];

    var _a = useState(window.location.hash || '#/system-status');
    var activeHash = _a[0];
    var setActiveHash = _a[1];

    var _u = useState(false);
    var userMenuOpen = _u[0];
    var setUserMenuOpen = _u[1];

    var sidebarRef = useRef(null);

    // Listen for route changes via App event bus
    useEffect(function () {
      function onRouteChange(info) {
        var hash = window.location.hash || '#/dashboard';
        setActiveHash(hash);
      }
      App.on('route:change', onRouteChange);
      return function () { App.off('route:change', onRouteChange); };
    }, []);

    // Close user menu on outside click
    useEffect(function () {
      function onClick(e) {
        if (sidebarRef.current && !sidebarRef.current.contains(e.target)) {
          setUserMenuOpen(false);
        }
      }
      document.addEventListener('click', onClick);
      return function () { document.removeEventListener('click', onClick); };
    }, []);
    // Set initial grid columns based on collapsed state
    useEffect(function () {
      if (collapsed) {
        var appLayout = document.querySelector('.app-layout');
        if (appLayout) {
          appLayout.style.gridTemplateColumns = 'var(--sidebar-collapsed-width) 1fr';
        }
      }
    }, []);

    var activeRoute = getActiveRoute(activeHash);

    function toggleCollapse() {
      var next = !collapsed;
      setColl(next);
      setCollapsed(next);
      if (typeof GlobalSearch !== 'undefined') GlobalSearch.closePopup();
      // Update app-layout grid to match sidebar width
      var appLayout = document.querySelector('.app-layout');
      if (appLayout) {
        appLayout.style.gridTemplateColumns = next
          ? 'var(--sidebar-collapsed-width) 1fr'
          : 'var(--sidebar-width) 1fr';
      }
    }

    function toggleUserMenu(e) {
      e.stopPropagation();
      setUserMenuOpen(!userMenuOpen);
    }

    function handleSearch(e) {
      e.stopPropagation();
      if (typeof GlobalSearch !== 'undefined') {
        if (GlobalSearch.isPopupOpen && GlobalSearch.isPopupOpen()) {
          GlobalSearch.closePopup();
        } else {
          GlobalSearch.openPopup();
        }
      }
    }

    function handleLogout() {
      Auth.logout();
    }

    function handlePassword() {
      setUserMenuOpen(false);
    }

    var collapseIcon = collapsed ? icons.chevronRight : icons.chevronLeft;
    var collapseTitleKey = collapsed ? 'nav_expand' : 'nav_collapse';
    var username = Auth.getUsername();

    return html`
      <aside class=${'sidebar' + (collapsed ? ' sidebar-collapsed' : '')}
             ref=${sidebarRef}
             data-sidebar data-hide-mobile>
        <div class="sidebar-brand">
          <span class="sidebar-brand-icon" dangerouslySetInnerHTML=${{ __html: icons.brand }}></span>
          <span class="sidebar-brand-text gradient-text"
                style="font-weight:var(--font-weight-bold,700);font-size:var(--font-size-lg);letter-spacing:-0.02em">
            MiBeeHive
          </span>
        </div>

        <div id="global-search-slot" class="sidebar-search-slot"></div>

        <button class="sidebar-search-btn" onClick=${handleSearch} title=${t('search')}>
          <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: icons.search }}></span>
        </button>

        <nav class="sidebar-nav-section" aria-label="Main navigation">
          ${navItems.map(function (item) {
            var isActive = activeRoute === item.route;
            return html`
              <a href=${item.hash}
                 class=${'sidebar-nav-item' + (isActive ? ' active' : '')}
                 data-route=${item.route}>
                <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: item.icon }}></span>
                <span class="sidebar-nav-label">${t(item.i18nKey)}</span>
              </a>
            `;
          })}
          <div class="divider"></div>
          <a href=${settingsItem.hash}
             class=${'sidebar-nav-item' + (activeRoute === settingsItem.route ? ' active' : '')}
             data-route=${settingsItem.route}>
            <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: settingsItem.icon }}></span>
            <span class="sidebar-nav-label">${t(settingsItem.i18nKey)}</span>
          </a>
        </nav>

        <button class="sidebar-nav-item sidebar-collapse-btn"
                onClick=${toggleCollapse}
                title=${t(collapseTitleKey)}>
          <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: collapseIcon }}></span>
          <span class="sidebar-nav-label">${t(collapseTitleKey)}</span>
        </button>

        <div class="sidebar-user-menu" data-sidebar-section="user-menu">
          <button class="sidebar-nav-item sidebar-user-trigger"
                  onClick=${toggleUserMenu}
                  title=${escapeHtml(username)}>
            <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: icons.user }}></span>
            <span class="sidebar-nav-label">${escapeHtml(username)}</span>
          </button>
          <div class="sidebar-user-dropdown"
               data-sidebar-section="user-dropdown"
               style=${{ display: userMenuOpen ? 'block' : 'none' }}>
            <a href="#/settings" class="sidebar-nav-item" onClick=${handlePassword}>
              <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: icons.key }}></span>
              <span class="sidebar-nav-label">${t('user_password')}</span>
            </a>
            <button class="sidebar-nav-item" onClick=${handleLogout}>
              <span class="sidebar-nav-icon" dangerouslySetInnerHTML=${{ __html: icons.logout }}></span>
              <span class="sidebar-nav-label">${t('user_logout')}</span>
            </button>
          </div>
        </div>
      </aside>
    `;
  }

  // ── Public API (backward compatible) ────────────────────────────────

  function render() {
    // For backward compat with Shell.init() that sets innerHTML
    // We return a placeholder; PreactBridge.render handles actual mounting
    return '<aside data-sidebar data-hide-mobile></aside>';
  }

  function bindEvents(container) {
    // No-op: Preact handles events via component
  }

  function updateActive(hash) {
    // No-op: Preact reactivity handles active state
  }

  return {
    render: render,
    bindEvents: bindEvents,
    updateActive: updateActive,
    isCollapsed: isCollapsed,
    SidebarComponent: SidebarComponent,
  };
})();
