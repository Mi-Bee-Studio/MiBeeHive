const SystemStatus = (function () {
  'use strict';
  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useEffect = PreactBridge.useEffect;

  var _cd = null;

  function _cleanup() {
    if (_cd) { _cd(); _cd = null; }
  }

  function _nav(a) {
    return '<nav class="module-tabs">' +
      '<a href="#/system-status" class="module-tab' + (a === 'dashboard' ? ' active' : '') + '">' + t('nav_dashboard') + '</a>' +
      '<a href="#/system-status/logs" class="module-tab' + (a === 'logs' ? ' active' : '') + '">' + t('nav_logs') + '</a>' +
      '<a href="#/system-status/tasks" class="module-tab' + (a === 'tasks' ? ' active' : '') + '">' + t('nav_tasks') + '</a></nav>';
  }

  function SSComponent(props) {
    var tab = props.initialTab || 'dashboard';

    useEffect(function () {
      _cleanup();
      if (tab === 'dashboard') {
        if (typeof DashboardPreact !== 'undefined' && DashboardPreact) {
          DashboardPreact.render();
        } else if (typeof Dashboard !== 'undefined' && Dashboard) {
          Dashboard.render();
          _cd = Dashboard.destroy.bind(Dashboard);
        }
      } else if (tab === 'logs') {
        if (typeof Logs !== 'undefined' && Logs) Logs.render();
      } else if (tab === 'tasks') {
        if (typeof Tasks !== 'undefined' && Tasks) Tasks.render();
      }
    }, []);

    return html`<nav class="module-tabs">
      <a href="#/system-status" class="module-tab ${tab === 'dashboard' ? 'active' : ''}">${t('nav_dashboard')}</a>
      <a href="#/system-status/logs" class="module-tab ${tab === 'logs' ? 'active' : ''}">${t('nav_logs')}</a>
      <a href="#/system-status/tasks" class="module-tab ${tab === 'tasks' ? 'active' : ''}">${t('nav_tasks')}</a>
    </nav>`;
  }

  function _render(tab) {
    var app = document.getElementById('main-content');
    if (!app) return;
    pRender(null, app);
    pRender(html`<${SSComponent} initialTab=${tab} />`, app);
  }

  function render() { _render('dashboard'); }
  function renderLogs() { _render('logs'); }
  function renderTasks() { _render('tasks'); }

  function destroy() {
    _cleanup();
    var app = document.getElementById('main-content');
    if (app) pRender(null, app);
  }

  return { render: render, renderLogs: renderLogs, renderTasks: renderTasks, _nav: _nav, destroy: destroy };
})();
