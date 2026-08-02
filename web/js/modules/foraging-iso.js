var ForagingISO = (function () {
  'use strict';
  var html = PreactBridge.html;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;

  // ── ISO Sources Table ─────────────────────────────────────────────────────
  function ISOSourcesTab() {
    var _sources = useState([]);
    var sources = _sources[0], setSources = _sources[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _showModal = useState(false);
    var showModal = _showModal[0], setShowModal = _showModal[1];

    useEffect(function () {
      Api.get('/admin/iso-sources').then(function (res) {
        if (res && res.success) {
          setSources(res.data || []);
        }
        setLoading(false);
      }).catch(function () { setLoading(false); });
    }, []);

    function handleAddSource() {
      setShowModal(true);
    }

    if (loading) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div class="flex items-center justify-between mb-5">
            <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('foraging.iso.title')}</h2>
            <button class="btn btn-primary btn-sm" disabled>${t('foraging.iso.add_source')}</button>
          </div>
          <div class="skeleton skeleton-table" style="margin-top:1rem"></div>
        </div>`;
    }

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('foraging.iso.title')}</h2>
          <button class="btn btn-primary btn-sm" onClick=${handleAddSource}>
            ${t('foraging.iso.add_source')}
          </button>
        </div>
        <div class="card" style="padding:1rem">
          ${sources.length ? html`
            <table class="table" style="width:100%">
              <thead>
                <tr>
                  <th class="text-left text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.distro')}</th>
                  <th class="text-left text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.version')}</th>
                  <th class="text-left text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.status')}</th>
                  <th class="text-left text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.last_crawl')}</th>
                </tr>
              </thead>
              <tbody>
                ${sources.map(function (source) {
                  return html`
                    <tr>
                      <td class="text-sm" style="color:var(--color-text)">${source.distro}</td>
                      <td class="text-sm" style="color:var(--color-text-secondary)">${source.version}</td>
                      <td class="text-sm">${Helpers.statusBadge(source.status)}</td>
                      <td class="text-sm" style="color:var(--color-text-tertiary)">${Helpers.formatTime(source.last_crawl)}</td>
                    </tr>`;
                })}
              </tbody>
            </table>
          ` : html`<p class="text-sm" style="color:var(--color-text-tertiary)">${t('foraging.no_data')}</p>`}
        </div>

        ${showModal ? html`
          <${AddISOSourceModal} onClose=${function () { setShowModal(false); }} onAdded=${function () {
            Api.get('/admin/iso-sources').then(function (res) {
              if (res && res.success) setSources(res.data || []);
            });
          }} />
        ` : null}
      </div>`;
  }

  // ── Add ISO Source Modal ────────────────────────────────────────────────
  function AddISOSourceModal(props) {
    var _distro = useState('');
    var distro = _distro[0], setDistro = _distro[1];
    var _version = useState('');
    var version = _version[0], setVersion = _version[1];
    var _submitting = useState(false);

    function handleSubmit() {
      if (!distro.trim() || !version.trim()) {
        Components.showToast(t('validation_required'), 'error');
        return;
      }
      _submitting[1](true);
      Api.post('/admin/iso-sources', {
        distro: distro.trim(),
        version: version.trim()
      }).then(function (res) {
        _submitting[1](false);
        if (res && res.success) {
          Components.showToast(t('proj_created'), 'success');
          if (props.onAdded) props.onAdded();
          props.onClose();
        } else {
          Components.showToast(res && res.message || t('error'), 'error');
        }
      }).catch(function () {
        _submitting[1](false);
        Components.showToast(t('error'), 'error');
      });
    }

    return html`
      <div class="modal-overlay" style="position:fixed;top:0;left:0;right:0;bottom:0;background:rgba(0,0,0,0.5);display:flex;align-items:center;justify-content:center;z-index:1000"
           onClick=${function (e) { if (e.target === e.currentTarget) props.onClose(); }}>
        <div class="modal-content card" style="max-width:32rem;width:100%;padding:1.5rem"
             onClick=${function (e) { e.stopPropagation(); }}>
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('foraging.iso.add_source')}</h3>
          <div class="grid gap-4">
            <div>
              <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.distro')}</label>
              <input class="input" value=${distro} onInput=${function (e) { setDistro(e.target.value); }} />
            </div>
            <div>
              <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('foraging.iso.version')}</label>
              <input class="input" value=${version} onInput=${function (e) { setVersion(e.target.value); }} />
            </div>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button class="btn btn-secondary" onClick=${props.onClose} disabled=${_submitting[0]}>${t('cancel')}</button>
            <button class="btn btn-primary" onClick=${handleSubmit} disabled=${_submitting[0]}>
              ${_submitting[0] ? '...' : t('save')}
            </button>
          </div>
        </div>
      </div>`;
  }

  // ── Main Component ───────────────────────────────────────────────────────
  function ForagingISOComponent(props) {
    return html`
      <div>
        <${ISOSourcesTab} />
      </div>`;
  }

  // ── Module API ───────────────────────────────────────────────────────────
  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (app) render(html`<${ForagingISOComponent} signal=${signal} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  var api = { render: renderFn, destroy: destroyFn, ForagingISOComponent: ForagingISOComponent };
  window.ForagingISO = api;
  return api;
})();
