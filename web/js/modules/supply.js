// Module: modules/supply — Ops-tool supply layer UI (Preact)
// Shows what artifacts MiBeeHive serves to external servers, grouped by type,
// and provides copy-paste config snippets (APT source line) for clients.
var Supply = (function () {
  'use strict';

  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;

  var _S = 'color:var(--color-text-secondary)';
  var _C = 'background:var(--color-bg-tertiary);padding:0.125rem 0.5rem;border-radius:var(--radius-sm)';

  // Public supply endpoints live at the root (not under /api/v1), so fetch them
  // directly rather than via the Api helper (which prepends /api/v1 + auth).
  function fetchJSON(url) {
    return fetch(url).then(function (r) { return r.ok ? r.json() : null; }).catch(function () { return null; });
  }

  function hostOrigin() {
    return window.location.origin;
  }

  // ── Copy-to-clipboard row ───────────────────────────────────────────────
  function CopyRow(props) {
    var _copied = useState(false);
    var copied = _copied[0], setCopied = _copied[1];
    function handleCopy() {
      Helpers.copyToClipboard(props.value).then(function () {
        setCopied(true);
        setTimeout(function () { setCopied(false); }, 1500);
      });
    }
    return html`
      <div class="flex items-center gap-2">
        <code class="text-xs flex-1" style="${_C};word-break:break-all">${Helpers.escapeHtml(props.value)}</code>
        <button class="btn btn-sm" onClick=${handleCopy}>${copied ? t('common_copied') : t('common_copyUrl')}</button>
      </div>`;
  }

  // ── APT config card ──────────────────────────────────────────────────────
  function AptCard(props) {
    var aptLine = 'deb [trusted=yes] ' + hostOrigin() + '/apt stable main';
    var debCount = props.debCount || 0;
    return html`
      <div class="card p-4 mb-4">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-base font-semibold">APT ${t('supply_repository')}</h3>
          <span class="text-xs" style="${_S}">${debCount} .deb</span>
        </div>
        <p class="text-sm mb-3" style="${_S}">${t('supply_apt_desc')}</p>
        <div class="text-xs mb-1" style="${_S}">${t('supply_source_line')}</div>
        <${CopyRow} value=${aptLine} />
        ${debCount === 0 ? html`<div class="text-xs mt-2" style="color:var(--color-warning)">${t('supply_apt_empty')}</div>` : null}
      </div>`;
  }

  // ── File list grouped by extension ───────────────────────────────────────
  function extGroups(items) {
    var groups = {};
    (items || []).forEach(function (it) {
      var e = it.ext || t('supply_unknown');
      if (!groups[e]) groups[e] = [];
      groups[e].push(it);
    });
    // Order: deb, rpm, tar.gz, zip, then the rest.
    var order = ['deb', 'rpm', 'tar.gz', 'zip', 'gz'];
    var keys = Object.keys(groups).sort(function (a, b) {
      var ia = order.indexOf(a), ib = order.indexOf(b);
      if (ia === -1 && ib === -1) return a < b ? -1 : 1;
      if (ia === -1) return 1;
      if (ib === -1) return -1;
      return ia - ib;
    });
    return keys.map(function (k) { return { ext: k, files: groups[k] }; });
  }

  function formatSize(bytes) {
    if (typeof Helpers.formatSize === 'function') return Helpers.formatSize(bytes);
    if (!bytes) return '0 B';
    var u = ['B', 'KB', 'MB', 'GB']; var i = 0; var n = bytes;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return n.toFixed(i === 0 ? 0 : 1) + ' ' + u[i];
  }

  function FileGroup(props) {
    var g = props.group;
    return html`
      <div class="mb-4">
        <div class="flex items-center gap-2 mb-2">
          <span class="badge">${Helpers.escapeHtml(g.ext)}</span>
          <span class="text-xs" style="${_S}">${g.files.length}</span>
        </div>
        <div class="space-y-1">
          ${g.files.slice(0, 50).map(function (f) {
            return html`
              <div class="flex items-center justify-between text-sm py-1 border-b" style="border-color:var(--color-border)">
                <span class="truncate" style="max-width:60%">${Helpers.escapeHtml(f.filename)}</span>
                <span class="flex items-center gap-3" style="${_S}">
                  ${f.os ? html`<span class="text-xs">${Helpers.escapeHtml(f.os)}/${Helpers.escapeHtml(f.arch || '')}</span>` : null}
                  ${f.version ? html`<span class="text-xs">${Helpers.escapeHtml(f.version)}</span>` : null}
                  <span class="text-xs">${formatSize(f.size_bytes)}</span>
                  <a href="${f.download_url}" target="_blank" class="text-xs" style="color:var(--color-primary)">${t('common_download')}</a>
                </span>
              </div>`;
          })}
          ${g.files.length > 50 ? html`<div class="text-xs text-center py-1" style="${_S}">+${g.files.length - 50}</div>` : null}
        </div>
      </div>`;
  }

  // ── Main page ─────────────────────────────────────────────────────────────
  function SupplyPage() {
    var _st = useState({ loading: true, error: null, items: [] });
    var st = _st[0], setSt = _st[1];

    useEffect(function () {
      fetchJSON('/repo/index').then(function (data) {
        if (data && data.items) setSt({ loading: false, error: null, items: data.items });
        else setSt({ loading: false, error: null, items: [] });
      });
    }, []);

    if (st.loading) {
      return html`<div class="p-6" dangerouslySetInnerHTML=${{ __html: Helpers.loadingSpinner() }} />`;
    }

    var groups = extGroups(st.items);
    var debCount = (st.items.filter(function (i) { return i.ext === 'deb'; }) || []).length;

    return html`
      <div class="p-4 md:p-6 max-w-4xl mx-auto">
        <div class="mb-4">
          <h1 class="text-xl font-bold mb-1">${t('title_supply')}</h1>
          <p class="text-sm" style="${_S}">${t('supply_subtitle')}</p>
        </div>

        <${AptCard} debCount=${debCount} />

        <div class="card p-4">
          <div class="flex items-center justify-between mb-3">
            <h3 class="text-base font-semibold">${t('supply_artifacts')}</h3>
            <span class="text-xs" style="${_S}">${st.items.length}</span>
          </div>
          ${st.items.length === 0
            ? html`<div class="text-sm text-center py-6" style="${_S}">${t('supply_empty')}</div>`
            : groups.map(function (g) { return html`<${FileGroup} group=${g} />`; })}
        </div>

        <div class="card p-4 mt-4">
          <div class="text-xs mb-1" style="${_S}">${t('supply_generic_endpoint')}</div>
          <${CopyRow} value=${hostOrigin() + '/repo/index'} />
        </div>
      </div>`;
  }

  // ── Public API ──────────────────────────────────────────────────────────
  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${SupplyPage} />`, app);
  }

  function cleanup() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, cleanup: cleanup };
})();
