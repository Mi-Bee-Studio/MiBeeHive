// Module: layout/bottom-tab — Mobile bottom tab bar & top bar (Preact)
var BottomTab = (function () {
  'use strict';

  var html = PreactBridge.html;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;

  // Mobile bottom tabs: File Center (home) → External Services → Foraging → Status → Containers.
  var TABS = [
    {
      id: 'file-center',
      path: '#/',
      match: function (hash) { return hash === '#' || hash === '#/' || hash === ''; },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M3 4.5A1.5 1.5 0 0 1 4.5 3h3.672a1.5 1.5 0 0 1 1.06.44L10.94 4.5H15.5A1.5 1.5 0 0 1 17 6v9.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 3 15.5z"/></svg>',
      label: function () { return t('nav.file_center'); },
    },
    {
      id: 'external-services',
      path: '#/share',
      match: function (hash) { return hash.startsWith('#/share'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M4 12v8a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-8"/><polyline points="16 6 12 2 8 6"/><line x1="12" y1="2" x2="12" y2="15"/></svg>',
      label: function () { return t('nav.external_services'); },
    },
    {
      id: 'foraging',
      path: '#/foraging',
      match: function (hash) { return hash.startsWith('#/foraging'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.5 13.33V6.67a1.67 1.67 0 0 0-.83-1.44l-5.84-3.34a1.67 1.67 0 0 0-1.66 0L3.33 5.23A1.67 1.67 0 0 0 2.5 6.67v6.66a1.67 1.67 0 0 0 .83 1.44l5.84 3.34a1.67 1.67 0 0 0 1.66 0l5.84-3.34a1.67 1.67 0 0 0 .83-1.44z"/><path d="M2.72 5.8 10 10l7.28-4.2M10 17.57V10"/></svg>',
      label: function () { return t('nav.foraging'); },
    },
    {
      id: 'system-status',
      path: '#/system-status',
      match: function (hash) { return hash.startsWith('#/system-status'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125z"/></svg>',
      label: function () { return t('nav_system_status'); },
    },
    {
      id: 'containers',
      path: '#/containers',
      match: function (hash) { return hash.startsWith('#/containers'); },
      icon: '<svg xmlns="http://www.w3.org/2000/svg" aria-hidden="true" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.5 13.33V6.67a1.67 1.67 0 0 0-.83-1.44l-5.84-3.34a1.67 1.67 0 0 0-1.66 0L3.33 5.23A1.67 1.67 0 0 0 2.5 6.67v6.66a1.67 1.67 0 0 0 .83 1.44l5.84 3.34a1.67 1.67 0 0 0 1.66 0l5.84-3.34a1.67 1.67 0 0 0 .83-1.44z"/><path d="M2.72 5.8 10 10l7.28-4.2M10 17.57V10"/></svg>',
      label: function () { return t('nav_containers'); },
    },
  ];

  function isSubPage(hash) {
    if (!hash || hash === '#' || hash === '#/') return false;
    var segments = hash.replace(/^#\/?/, '').split('/');
    return segments.length > 1;
  }

  // ── Preact BottomTabComponent ──────────────────────────────────────

  function BottomTabComponent() {
    var _a = useState(window.location.hash || '#/');
    var activeHash = _a[0];
    var setActiveHash = _a[1];

    useEffect(function () {
      function onRouteChange() {
        setActiveHash(window.location.hash || '#/');
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
