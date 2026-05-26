// Module: layout/bottom-tab — Mobile bottom tab bar & top bar (Preact)
var BottomTab = (function () {
  'use strict';

  var html = PreactBridge.html;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;

  var TABS = [
    {
      id: 'system-status',
      path: '#/system-status',
      match: function (hash) { return hash.startsWith('#/system-status'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"/></svg>',
      label: function () { return t('nav_system_status'); },
    },
    {
      id: 'files',
      path: '#/files',
      match: function (hash) { return hash.startsWith('#/files'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 4.5A1.5 1.5 0 0 1 4.5 3h3.672a1.5 1.5 0 0 1 1.06.44L10.94 4.5H15.5A1.5 1.5 0 0 1 17 6v9.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 15.5z"/></svg>',
      label: function () { return t('nav_files'); },
    },
    {
      id: 'deploy',
      path: '#/deploy',
      match: function (hash) { return hash.startsWith('#/deploy'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="6" width="14" height="11" rx="1.5"/><path d="M7 6V4.5A1.5 1.5 0 0 1 8.5 3h3A1.5 1.5 0 0 1 13 4.5V6"/><line x1="10" y1="9" x2="10" y2="13"/><polyline points="8,11 10,9 12,11"/></svg>',
      label: function () { return t('nav_deploy'); },
    },
    {
      id: 'share',
      path: '#/share',
      match: function (hash) { return hash.startsWith('#/share'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="5" r="2.5"/><circle cx="4" cy="14" r="2.5"/><circle cx="16" cy="14" r="2.5"/><line x1="8.2" y1="6.7" x2="5.8" y2="12.3"/><line x1="11.8" y1="6.7" x2="14.2" y2="12.3"/></svg>',
      label: function () { return t('nav_share'); },
    },
    {
      id: 'containers',
      path: '#/containers',
      match: function (hash) { return hash.startsWith('#/containers'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.5 13.33V6.67a1.67 1.67 0 00-.83-1.44l-5.84-3.34a1.67 1.67 0 00-1.66 0L3.33 5.23A1.67 1.67 0 002.5 6.67v6.66a1.67 1.67 0 00.83 1.44l5.84 3.34a1.67 1.67 0 001.66 0l5.84-3.34a1.67 1.67 0 00.83-1.44z"/><path d="M2.72 5.8 10 10l7.28-4.2M10 17.57V10"/></svg>',
      label: function () { return t('nav_containers'); },
    },
    {
      id: 'settings',
      path: '#/settings',
      match: function (hash) { return hash === '#/settings'; },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="2.5"/><path d="M10 2v2.5M10 15.5V18M2 10h2.5M15.5 10H18M4.22 4.22l1.77 1.77M14.01 14.01l1.77 1.77M4.22 15.78l1.77-1.77M14.01 5.99l1.77-1.77"/></svg>',
      label: function () { return t('nav_settings'); },
    },
  ];

  function isSubPage(hash) {
    if (!hash || hash === '#' || hash === '#/') return false;
    var segments = hash.replace(/^#\/?/, '').split('/');
    return segments.length > 1;
  }

  // ── Preact BottomTabComponent ──────────────────────────────────────

  function BottomTabComponent() {
    var _a = useState(window.location.hash || '#/system-status');
    var activeHash = _a[0];
    var setActiveHash = _a[1];

    useEffect(function () {
      function onRouteChange() {
        setActiveHash(window.location.hash || '#/system-status');
      }
      App.on('route:change', onRouteChange);
      return function () { App.off('route:change', onRouteChange); };
    }, []);

    function handleTabClick(path) {
      window.location.hash = path;
    }

    return html`
      <nav class="bottom-tab-bar" role="tablist" aria-label="Mobile navigation">
        ${TABS.map(function (tab) {
          var isActive = tab.match(activeHash);
          return html`
            <button class=${'bottom-tab-item' + (isActive ? ' active' : '')}
                    role="tab"
                    aria-label=${tab.label()}
                    aria-selected=${isActive ? 'true' : undefined}
                    data-tab=${tab.id}
                    onClick=${function () { handleTabClick(tab.path); }}>
              <span dangerouslySetInnerHTML=${{ __html: tab.icon }}></span>
              <span>${tab.label()}</span>
            </button>
          `;
        })}
      </nav>
    `;
  }

  // ── Preact MobileTopBarComponent ──────────────────────────────────

  function MobileTopBarComponent(props) {
    function handleBack() {
      window.history.back();
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
        <span class="mobile-top-bar-title">${props.title || ''}</span>
      </div>
    `;
  }

  // ── Public API (backward compatible) ──────────────────────────────

  function renderBottomTab() {
    // For backward compat: returns a nav element, but actual rendering via Preact
    var nav = document.createElement('nav');
    nav.className = 'bottom-tab-bar';
    return nav;
  }

  function renderMobileTopBar(title) {
    var bar = document.createElement('div');
    bar.className = 'mobile-top-bar';
    return bar;
  }

  function updateActive(hash) {
    // No-op: Preact reactivity handles active state
  }

  return {
    renderBottomTab: renderBottomTab,
    renderMobileTopBar: renderMobileTopBar,
    updateActive: updateActive,
    isSubPage: isSubPage,
    BottomTabComponent: BottomTabComponent,
    MobileTopBarComponent: MobileTopBarComponent,
    TABS: TABS,
  };
})();
