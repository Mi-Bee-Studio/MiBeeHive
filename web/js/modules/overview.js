// Module: modules/overview — Supply-chain overview home page (Preact)
//
// Landing page that tells the product's storyline end-to-end:
//   Foraging (采集) → Storage → Supply (供应)
// It shows what MiBeeHive has collected, how full the storage is, and how
// external servers consume the served artifacts (APT source line, WebDAV).
// Backed by /api/v1/admin/dashboard/summary + /repo/index.
var Overview = (function () {
  'use strict';

  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useMemo = PreactBridge.useMemo;

  var usePolling = (window.Hooks && Hooks.usePolling) || null;
  var SCOPE = '/overview';

  function _fetchJSON(url) {
    return fetch(url).then(function (r) { return r.ok ? r.json() : null; }).catch(function () { return null; });
  }
  function hostOrigin() { return window.location.origin; }
  function _pct(v) { return (Math.round((v || 0) * 10) / 10) + '%'; }
  function _fmtBytes(n) {
    if (!n) return '0 B';
    var units = ['B', 'KB', 'MB', 'GB', 'TB'];
    var i = 0; var v = n;
    while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
    return (Math.round(v * 10) / 10) + ' ' + units[i];
  }
  function _ago(ts) {
    if (!ts) return '';
    var then = new Date(ts).getTime();
    if (isNaN(then)) return '';
    var s = Math.max(0, Math.floor((Date.now() - then) / 1000));
    if (s < 60) return Math.max(1, s) + 's';
    if (s < 3600) return Math.floor(s / 60) + 'm';
    if (s < 86400) return Math.floor(s / 3600) + 'h';
    return Math.floor(s / 86400) + 'd';
  }

  function Copyable(props) {
    var _c = useState(false);
    var copied = _c[0], setCopied = _c[1];
    function handleCopy() {
      Helpers.copyToClipboard(props.value).then(function () {
        setCopied(true);
        setTimeout(function () { setCopied(false); }, 1500);
      });
    }
    return html`
      <div class="flex items-center gap-2">
        <code class="text-xs flex-1" style="background:var(--color-bg-tertiary);padding:0.25rem 0.5rem;border-radius:var(--radius-sm);word-break:break-all">${props.value}</code>
        <button class="btn btn-sm" onClick=${handleCopy}>${copied ? t('common_copied') : t('common_copyUrl')}</button>
      </div>`;
  }

  // One stage of the collect → store → supply pipeline.
  function StageCard(props) {
    return html`
      <a href=${props.href} class="card card-hover anim-fade-in" style="padding:1.25rem;display:block;text-decoration:none;color:inherit">
        <div class="flex items-center justify-between mb-2">
          <span class="text-sm font-semibold" style="color:var(--color-text)">${props.title}</span>
          <span class="text-xs" style="color:var(--color-text-quaternary)">${props.tag}</span>
        </div>
        <div class="flex items-baseline gap-1 mb-1">
          <span class="text-2xl font-bold" style="color:var(--color-primary)">${props.big}</span>
          ${props.bigLabel ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${props.bigLabel}</span>` : null}
        </div>
        <div class="text-xs" style="color:var(--color-text-secondary)">${props.desc}</div>
      </a>`;
  }

  function OverviewPage() {
    var _d = useState(null);
    var data = _d[0], setData = _d[1];
    var _s = useState(null);
    var supply = _s[0], setSupply = _s[1];
    var _e = useState(false);
    var err = _e[0], setErr = _e[1];

    function load(signal) {
      Promise.all([
        Api.get('/admin/dashboard/summary', { signal: signal, silent: true }),
        _fetchJSON(hostOrigin() + '/repo/index')
      ]).then(function (results) {
        var sum = results[0];
        var idx = results[1];
        if (!sum || !sum.success || !sum.data) {
          if (!signal || !signal.aborted) setErr(true);
          return;
        }
        setErr(false);
        setData(sum.data);
        // repo/index returns { artifacts: [...] } or similar; count deb/rpm/etc.
        if (idx && idx.success !== false) {
          setSupply(idx);
        }
      });
    }

    // initial + polling
    useEffect(function () {
      var signal = Router.getSignal ? Router.getSignal() : null;
      load(signal);
      if (usePolling) {
        usePolling(function () { load(Router.getSignal ? Router.getSignal() : null); }, 15000, { scope: SCOPE, signal: signal });
      } else {
        var id = setInterval(function () { load(null); }, 15000);
        if (window.App) App.addTimer(id, SCOPE);
        return function () { clearInterval(id); };
      }
    }, []);

    var debCount = useMemo(function () {
      if (!supply) return 0;
      // /repo/index returns { count, items:[{filename, ext, ...}] }
      var arts = supply.items || supply.artifacts || supply.data || supply.files || [];
      if (!Array.isArray(arts)) return 0;
      return arts.filter(function (a) {
        return a.ext === 'deb' || /\.deb$/i.test(a.filename || a.name || '');
      }).length;
    }, [supply]);

    if (err && !data) {
      return html`<div class="p-6">
        <div class="empty-state"><p class="text-sm" style="color:var(--color-text-tertiary)">${t('error_load_failed')}</p>
        <button class="btn btn-primary btn-sm mt-3" onClick=${function () { load(null); }}>${t('error_retry')}</button></div>
      </div>`;
    }
    if (!data) {
      return html`<div class="p-6"><div class="skeleton skeleton-heading" style="width:240px"></div>
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
          <div class="skeleton skeleton-card"></div><div class="skeleton skeleton-card"></div><div class="skeleton skeleton-card"></div>
        </div></div>`;
    }

    var sys = data.system || {};
    var files = data.files || {};
    var share = data.share || {};
    var activity = data.activity || [];
    var aptLine = 'deb [trusted=yes] ' + hostOrigin() + '/apt stable main';
    var webdavUrl = hostOrigin().replace('9090', '9090') + '/webdav/';

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="anim-fade-in mb-6">
          <h1 class="text-2xl font-bold tracking-tight" style="color:var(--color-text)">${t('overview_title')}</h1>
          <p class="text-sm mt-1" style="color:var(--color-text-secondary)">${t('overview_subtitle')}</p>
        </div>

        <!-- Collect → Store → Supply pipeline -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
          <${StageCard}
            href="#/files"
            title=${t('overview_stage_collect')}
            tag=${t('nav_group_foraging')}
            big=${String(files.project_count || 0)}
            bigLabel=${t('overview_projects')}
            desc=${t('overview_stage_collect_desc', { files: String(files.total_files || 0), pending: String((files.queue_pending || 0) + (files.queue_downloading || 0)) })} />
          <${StageCard}
            href="#/system-status"
            title=${t('overview_stage_store')}
            tag=${t('nav_system_status')}
            big=${_pct(sys.disk_usage_percent || 0)}
            desc=${t('overview_stage_store_desc', { used: _fmtBytes(sys.disk_used_bytes), total: _fmtBytes(sys.disk_total_bytes) })} />
          <${StageCard}
            href="#/supply"
            title=${t('overview_stage_supply')}
            tag=${t('nav_group_supply')}
            big=${String(debCount)}
            bigLabel=".deb"
            desc=${t('overview_stage_supply_desc', { webdav: String(share.file_count || 0) })} />
        </div>

        <!-- Supply endpoints quick-copy -->
        <div class="card mb-6" style="padding:1.25rem">
          <h2 class="text-base font-semibold mb-3" style="color:var(--color-text)">${t('overview_endpoints')}</h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <div class="text-xs mb-1" style="color:var(--color-text-secondary)">APT ${t('supply_repository')}</div>
              <${Copyable} value=${aptLine} />
            </div>
            <div>
              <div class="text-xs mb-1" style="color:var(--color-text-secondary)">WebDAV</div>
              <${Copyable} value=${webdavUrl} />
            </div>
          </div>
        </div>

        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <!-- Recent activity -->
          <div class="card" style="padding:1.25rem">
            <div class="flex items-center justify-between mb-3">
              <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('dash_activity_title')}</h2>
              <a class="text-xs" href="#/system-status/logs" style="color:var(--color-primary)">${t('overview_view_all')}</a>
            </div>
            ${activity.length === 0 ? html`<p class="text-sm" style="color:var(--color-text-tertiary)">${t('activity_empty')}</p>`
              : activity.slice(0, 6).map(function (evt) {
                  var labelKey = 'activity_' + evt.type;
                  var label = t(labelKey, { name: evt.title || '' });
                  return html`
                    <div key=${evt.id} class="flex items-start gap-2 py-1.5" style="border-bottom:1px solid var(--color-border)">
                      <div style="flex:1;min-width:0">
                        <div class="text-sm" style="color:var(--color-text)">${label}</div>
                        ${evt.subtitle ? html`<div class="text-xs" style="color:var(--color-text-tertiary)">${evt.subtitle}</div>` : null}
                      </div>
                      <span class="text-xs" style="color:var(--color-text-quaternary);flex-shrink:0">${_ago(evt.timestamp)}</span>
                    </div>`;
                })}
          </div>

          <!-- System resources -->
          <div class="card" style="padding:1.25rem">
            <div class="flex items-center justify-between mb-3">
              <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('overview_resources')}</h2>
              <a class="text-xs" href="#/system-status" style="color:var(--color-primary)">${t('overview_view_all')}</a>
            </div>
            <div class="grid grid-cols-3 gap-3 text-center">
              <div>
                <div class="text-xs" style="color:var(--color-text-tertiary)">CPU</div>
                <div class="text-lg font-semibold" style="color:var(--color-text)">${_pct(sys.cpu_usage_percent)}</div>
              </div>
              <div>
                <div class="text-xs" style="color:var(--color-text-tertiary)">${t('mem_usage')}</div>
                <div class="text-lg font-semibold" style="color:var(--color-text)">${_pct(sys.memory_usage_percent)}</div>
              </div>
              <div>
                <div class="text-xs" style="color:var(--color-text-tertiary)">${t('disk_usage')}</div>
                <div class="text-lg font-semibold" style="color:var(--color-text)">${_pct(sys.disk_usage_percent)}</div>
              </div>
            </div>
            <div class="text-xs mt-3" style="color:var(--color-text-quaternary)">${t('dash_version')} ${sys.version || '-'} · ${t('dash_uptime')} ${sys.uptime || '-'}</div>
          </div>
        </div>
      </div>`;
  }

  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    pRender(null, app);
    pRender(html`<${OverviewPage} />`, app);
  }
  function destroy() {
    var app = document.getElementById('main-content');
    if (app) pRender(null, app);
  }

  return { render: render, destroy: destroy };
})();
