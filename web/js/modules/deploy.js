const Deploy = (function () {
  'use strict';
  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  // ── Sub-navigation tabs (shared HTML) ────────────────────────────────
  var DEPLOY_TABS = [
    { hash: '/deploy', i18nKey: 'nav_deploy_configs', tooltipKey: 'tooltip_deploy' },
  ];
  function _subNav(active) {
    return Components.moduleTabs(DEPLOY_TABS, 'nav_deploy_configs');
  }

  // ── Deploy Component ─────────────────────────────────────────────────
  function DeployPage() {
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _subNav('configs') }} />
      </div>`;
  }

  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    // Default to configs sub-module
    if (typeof DeployConfigs !== 'undefined' && DeployConfigs) {
      DeployConfigs.render();
    } else {
      preactRender(html`<${DeployPage} />`, app);
    }
  }

  function destroy() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, destroy: destroy };
})();
