const Deploy = (function () {
  'use strict';
  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  // ── Sub-navigation tabs (shared HTML) ────────────────────────────────
  function _subNav(active) {
    return '<nav class="module-tabs">' +
      '<a href="#/deploy" class="module-tab' + (active === 'configs' ? ' active' : '') + '" data-tooltip="' + t('tooltip_deploy') + '">' + t('nav_deploy_configs') + '</a>' +
      '<a href="#/deploy/iso" class="module-tab' + (active === 'iso' ? ' active' : '') + '">' + t('nav_deploy_iso') + '</a></nav>';
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
