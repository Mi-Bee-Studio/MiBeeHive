
const Share = (function () {
  'use strict';

  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  // ── Style constants ─────────────────────────────────────────────────────
  var _S = 'color:var(--color-text-secondary)';
  var _C = 'background:var(--color-bg-tertiary);padding:0.125rem 0.5rem;border-radius:var(--radius-sm)';

  // ── SVG paths for platform guides ───────────────────────────────────────
  var WINDOWS_PATH = 'M3 12V6.75l6-1.32v6.48L3 12zm17-9v8.75l-10 .08V5.21L20 3zM3 13l6 .09v6.81l-6-1.15V13zm7 .18L20 13v8.5l-10-1.84V13.18z';
  var MACOS_PATH = 'M18.71 19.5c-.83 1.24-1.71 2.45-3.05 2.47-1.34.03-1.77-.79-3.29-.79-1.53 0-2 .77-3.27.82-1.31.05-2.3-1.32-3.14-2.53C4.25 17 2.94 12.45 4.7 9.39c.87-1.52 2.43-2.48 4.12-2.51 1.28-.02 2.5.87 3.29.87.78 0 2.26-1.07 3.8-.91.65.03 2.47.26 3.64 1.98-.09.06-2.17 1.28-2.15 3.81.03 3.02 2.65 4.03 2.68 4.04-.03.07-.42 1.44-1.38 2.83M13 3.5c.73-.83 1.94-1.46 2.94-1.5.13 1.17-.34 2.35-1.04 3.19-.69.85-1.83 1.51-2.95 1.42-.15-1.15.41-2.35 1.05-3.11z';
  var LINUX_PATH = 'M12.5 2c-1.5 0-2.5 1.5-2.5 3.5S11 9 12.5 9 15 7.5 15 5.5 14 2 12.5 2zm-4 4c-1 0-2.5.5-3.5 2s-.5 3.5.5 4.5 3 1 4 0 1.5-2.5 1.5-4-.5-2.5-2.5-2.5zm8.5 0c-2 0-2.5 1-2.5 2.5s.5 3 1.5 4 3 .5 4-.5 1.5-3 .5-4.5-2.5-2-3.5-2zM12 10c-1.5 0-3 1-3 3s1 3.5 2 4.5 2 1.5 2 2.5c0 .5-.5 1.5-.5 2s.5 1 1 1 .5-.5.5-1 0-1.5 0-2c0-1 .5-1.5 1.5-2.5s2-2.5 2-4.5-1.5-3-3-3z';

  // ── Nav tabs ────────────────────────────────────────────────────────────
  function NavTabs(props) {
    return html`
      <nav class="module-tabs">
        <a href="#/share" class="module-tab ${props.conn ? 'active' : ''}" data-tooltip="${t('tooltip_share')}">${t('webdav_connection')}</a>
        <a href="#/share/files" class="module-tab ${props.files ? 'active' : ''}">${t('webdav_files')}</a>
        <a href="#/share/settings" class="module-tab ${props.sett ? 'active' : ''}">${t('settings')}</a>
      </nav>`;
  }

  // ── Status badge ────────────────────────────────────────────────────────
  function StatusBadge(props) {
    var on = props.enabled;
    return html`
      <span class="inline-flex items-center gap-1.5">
        <span class="status-dot ${on ? 'status-dot-success' : 'status-dot-error'}"></span>
        <span class="text-sm font-medium" style="color:var(--color-${on ? 'success' : 'error'})">${t(on ? 'webdav_enabled' : 'webdav_disabled')}</span>
      </span>`;
  }

  // ── Info row ────────────────────────────────────────────────────────────
  function InfoRow(props) {
    return html`
      <div class="flex items-center justify-between py-2 border-b" style="border-color:var(--color-border)">
        <span class="text-sm" style="${_S}">${props.label}</span>
        <div>${props.children}</div>
      </div>`;
  }

  // ── Code value ──────────────────────────────────────────────────────────
  function CodeVal(props) {
    return html`<code class="text-xs" style="${_C}">${Helpers.escapeHtml(props.value)}</code>`;
  }

  // ── URL row with copy button ────────────────────────────────────────────
  function UrlRow(props) {
    var _copied = useState(false);
    var copied = _copied[0], setCopied = _copied[1];

    function handleCopy() {
      Helpers.copyToClipboard(props.url).then(function () {
        Components.showToast(t('webdav_url_copied'), 'success');
        setCopied(true);
        setTimeout(function () { setCopied(false); }, 2000);
      });
    }

    return html`
      <div class="flex items-center justify-between py-2 border-b" style="border-color:var(--color-border)">
        <span class="text-sm" style="${_S}">${props.label}</span>
        <div class="flex items-center gap-2">
          <code class="text-xs" style="${_C}">${Helpers.escapeHtml(props.url)}</code>
          <button class="btn btn-ghost share-copy-btn"
            style="padding:0.125rem 0.5rem;font-size:0.6875rem;${copied ? 'background:var(--color-success);color:var(--color-text-inverse)' : ''}"
            onClick=${handleCopy}>
            ${copied ? 'Copied!' : t('webdav_copy_url')}
          </button>
        </div>
      </div>`;
  }

  // ── Platform guide (details/summary) ────────────────────────────────────
  function Guide(props) {
    var _expanded = useState(true);
    var expanded = _expanded[0], setExpanded = _expanded[1];

    return html`
      <details open=${expanded} onToggle=${function (e) { setExpanded(e.target.open); }}
        style="border:1px solid var(--color-border);border-radius:var(--radius-md);margin-bottom:${props.last ? '0' : '0.5rem'}">
        <summary class="flex items-center gap-2 px-3 py-2.5 cursor-pointer"
          aria-expanded="${expanded}"
          style="color:var(--color-text);font-size:0.875rem;font-weight:var(--font-weight-medium)">
          <svg aria-hidden="true" width="1rem" height="1rem" viewBox="0 0 24 24" fill="currentColor" style="flex-shrink:0"><path d="${props.iconPath}"/></svg>
          ${props.title}
        </summary>
        <div class="px-3 pb-3" style="font-size:0.8125rem;color:var(--color-text-secondary)">
          ${props.url ? html`<code class="text-xs" style="${_C};display:inline-block;margin-bottom:0.5rem">${Helpers.escapeHtml(props.url)}</code>` : null}
          <p class="mt-1">${props.desc}</p>
        </div>
      </details>`;
  }

  // ── Connection view ─────────────────────────────────────────────────────
  function ConnectionView(props) {
    var d = props.data;
    var best = d.https_url || d.http_url || '';
    var http = d.http_url || '';

    return html`
      <div>
        <div class="card" style="padding:1.25rem">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-base font-semibold" style="color:var(--color-text)">${t('webdav_title')}</h3>
            <${StatusBadge} enabled=${d.enabled} />
          </div>
          ${d.http_url ? html`<${UrlRow} label=${t('webdav_http_url')} url=${d.http_url} />` : null}
          ${d.https_url
            ? html`<${UrlRow} label=${t('webdav_https_url')} url=${d.https_url} />`
            : html`<${InfoRow} label=${t('webdav_https_url')}>
                <span class="text-sm" style="color:var(--color-text-tertiary)">${t('webdav_no_https')}</span>
              <//>`}
          ${d.https_url ? html`
            <div class="flex items-center justify-between py-2 border-b" style="border-color:var(--color-border)">
              <span class="text-xs" style="color:var(--color-warning)">${t('webdav_https_only')}</span>
            </div>` : null}
          <div class="py-2">
            <span class="text-sm" style="${_S}">${t('webdav_auth_note')}</span>
          </div>
        </div>
        <div class="card" style="padding:1.25rem;margin-top:1rem">
          <h3 class="text-base font-semibold mb-3" style="color:var(--color-text)">${t('webdav_connection')}</h3>
          <${Guide} title=${t('webdav_windows')} url=${best} desc=${t('webdav_windows_desc')} iconPath=${WINDOWS_PATH} />
          <${Guide} title=${t('webdav_macos')} url=${http} desc=${t('webdav_macos_desc')} iconPath=${MACOS_PATH} />
          <${Guide} title=${t('webdav_linux')} url=${http} desc=${t('webdav_linux_desc')} iconPath=${LINUX_PATH} last=${true} />
        </div>
      </div>`;
  }

  // ── Settings view ───────────────────────────────────────────────────────
  function SettingsView(props) {
    var d = props.data;
    return html`
      <div>
        <div class="card" style="padding:1.25rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('webdav_title')}</h3>
          <${InfoRow} label=${t('webdav_status')}><${StatusBadge} enabled=${d.enabled} /><//>
          <${InfoRow} label=${t('webdav_http_url')}>${d.http_url ? html`<${CodeVal} value=${d.http_url} />` : '-'}<//>
          <${InfoRow} label=${t('webdav_https_url')}>${d.https_url ? html`<${CodeVal} value=${d.https_url} />` : html`<span style="color:var(--color-text-tertiary)">${t('webdav_no_https')}</span>`}<//>
          ${d.https_url ? html`
            <div class="flex items-center justify-between py-2 border-b" style="border-color:var(--color-border)">
              <span style="color:var(--color-warning);font-size:0.8125rem">${t('webdav_https_only')}</span>
            </div>` : null}
        </div>
        <div class="card" style="padding:1.25rem;margin-top:1rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('webdav_https_cert')}</h3>
          <${InfoRow} label=${t('webdav_cert_type')}><span class="badge badge-default">ECDSA P256</span><//>
          <${InfoRow} label=${t('webdav_cert_source')}><span style="color:var(--color-text-secondary);font-size:0.8125rem">${t('webdav_cert_self_signed')}</span><//>
        </div>
      </div>`;
  }

  // ── Share page component ────────────────────────────────────────────────
  function SharePage(props) {
    var _data = useState(null);
    var data = _data[0], setData = _data[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var mountedRef = useRef(true);

    useEffect(function () {
      mountedRef.current = true;
      Api.get('/admin/webdav/status').then(function (r) {
        if (!mountedRef.current) return;
        if (!r || !r.success) {
          setError((r && r.message) || t('error'));
          setLoading(false);
          return;
        }
        setData(r.data || {});
        setLoading(false);
      });
      return function () { mountedRef.current = false; };
    }, []);

    var mode = props.mode;

    return html`
      <div class="p-4 md:p-6 max-w-4xl mx-auto">
        <${NavTabs} conn=${mode === 'connection'} files=${false} sett=${mode === 'settings'} />
        ${loading ? html`<div dangerouslySetInnerHTML=${{ __html: Helpers.loadingSpinner() }} />` : null}
        ${error ? html`<div dangerouslySetInnerHTML=${{ __html: Helpers.errorMessage(error) }} />` : null}
        ${!loading && !error && data
          ? (mode === 'settings'
            ? html`<${SettingsView} data=${data} />`
            : html`<${ConnectionView} data=${data} />`)
          : null}
      </div>`;
  }

  // ── Public API ──────────────────────────────────────────────────────────
  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${SharePage} mode="connection" />`, app);
  }

  function renderSettings() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${SharePage} mode="settings" />`, app);
  }

  function cleanup() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, renderSettings: renderSettings, cleanup: cleanup };
})();
