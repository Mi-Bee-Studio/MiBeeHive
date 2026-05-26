const Registries = (function () {
  'use strict';
  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  var _typeUrls = { dockerhub: 'https://registry-1.docker.io', ghcr: 'https://ghcr.io', quay: 'https://quay.io' };

  function _subNav() {
    return '<nav class="module-tabs">' +
      '<a href="#/containers" class="module-tab">' + t('nav_containers') + '</a>' +
      '<a href="#/containers/images" class="module-tab">' + t('nav_containers_images') + '</a>' +
      '<a href="#/containers/templates" class="module-tab">' + t('nav_containers_templates') + '</a>' +
      '<a href="#/containers/registries" class="module-tab active">' + t('nav_registries') + '</a></nav>';
  }

  var _tabLabels = { connections: 'reg_connections', browse: 'reg_browse', sync: 'reg_sync', cleanup: 'reg_cleanup' };

  // ── Registry Modal (portal) ────────────────────────────────────────────
  function RegistryModal(props) {
    var reg = props.registry;
    var isEditing = !!reg;
    var formState = useState({
      name: reg ? reg.name : '',
      type: reg ? reg.type : 'dockerhub',
      url: reg ? (reg.url || '') : '',
      username: reg ? (reg.username || '') : '',
      password: ''
    });
    var form = formState[0], setForm = formState[1];
    var testState = useState(null);
    var testResult = testState[0], setTestResult = testState[1];
    var showPwState = useState(false);
    var showPw = showPwState[0], setShowPw = showPwState[1];
    var busyState = useState(false);
    var busy = busyState[0], setBusy = busyState[1];
    var ref = useRef(null);

    useEffect(function () {
      var el = ref.current && ref.current.querySelector('input');
      if (el) el.focus();
    }, []);

    function up(k, v) { setForm(Object.assign({}, form, { [k]: v })); }

    function handleTypeChange(tp) {
      var url = form.url;
      if (_typeUrls[tp] && (!url || Object.values(_typeUrls).indexOf(url) >= 0)) url = _typeUrls[tp];
      setForm(Object.assign({}, form, { type: tp, url: url }));
    }

    function handleSave() {
      if (!form.name.trim()) { Components.showToast(t('reg_name_required'), 'error'); return; }
      setBusy(true);
      var data = { name: form.name.trim(), url: form.url.trim(), type: form.type, username: form.username.trim() };
      if (form.password) data.password = form.password;
      if (isEditing) data.enabled = reg.enabled;
      var req = isEditing ? Api.put('/admin/registries/' + reg.id, data) : Api.post('/admin/registries', data);
      req.then(function (r) {
        setBusy(false);
        if (r && r.success) { Components.showToast(isEditing ? t('reg_updated') : t('reg_created'), 'success'); props.onSave(); props.onClose(); }
        else Components.showToast((r && r.message) || t('reg_failed'), 'error');
      });
    }

    function handleTest() {
      setTestResult({ testing: true });
      var data = { name: form.name.trim(), url: form.url.trim(), type: form.type, username: form.username.trim() };
      if (form.password) data.password = form.password;
      Api.post('/admin/registries/test', data).then(function (r) {
        setTestResult(r && r.success
          ? { ok: true, message: r.data || t('reg_connection_ok') }
          : { ok: false, message: (r && r.message) || t('reg_connection_failed') });
      });
    }

    function handleKey(e) {
      if (e.key === 'Escape') { props.onClose(); return; }
      if (e.key === 'Tab') {
        var els = ref.current.querySelectorAll('button, input, select, textarea');
        if (!els.length) return;
        if (e.shiftKey && document.activeElement === els[0]) { e.preventDefault(); els[els.length - 1].focus(); }
        else if (!e.shiftKey && document.activeElement === els[els.length - 1]) { e.preventDefault(); els[0].focus(); }
      }
    }

    var lbl = 'display:block;margin-bottom:0.25rem;font-size:0.75rem;font-weight:var(--font-weight-medium);color:var(--color-text-secondary)';
    var inp = { width: '100%', marginTop: '2px' };
    var typeOpts = ['dockerhub', 'ghcr', 'acr', 'tcr', 'quay'];

    return html`
      <div ref=${ref} class="modal-overlay" onClick=${function (e) { if (e.target === e.currentTarget) props.onClose(); }} onKeyDown=${handleKey}>
        <div class="modal-content" role="dialog" aria-modal="true">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${isEditing ? t('reg_edit_registry') : t('reg_add_registry')}</h3>
          <div class="flex flex-col gap-3">
            <div><label style=${lbl}>${t('reg_name')} *</label>
              <input class="input" style=${inp} value=${form.name} onInput=${function (e) { up('name', e.target.value); }} /></div>
            <div><label style=${lbl}>${t('reg_type')}</label>
              <select class="input" style=${inp} value=${form.type} onChange=${function (e) { handleTypeChange(e.target.value); }}>
                ${typeOpts.map(function (tp) { return html`<option key=${tp} value=${tp}>${t('reg_' + tp)}</option>`; })}
              </select></div>
            <div><label style=${lbl}>${t('reg_url')}</label>
              <input class="input" style=${inp} value=${form.url} placeholder="https://registry.example.com"
                onInput=${function (e) { up('url', e.target.value); }} />
              <span style="font-size:0.6875rem;color:var(--color-text-tertiary)">${t('help_reg_url')}</span></div>
            <div><label style=${lbl}>${t('reg_username')}</label>
              <input class="input" style=${inp} value=${form.username} onInput=${function (e) { up('username', e.target.value); }} /></div>
            <div><label style=${lbl}>${t('reg_password')}</label>
              <input type=${showPw ? 'text' : 'password'} class="input" style=${inp} value=${form.password}
                placeholder=${isEditing ? t('reg_password_keep_existing') : ''}
                onInput=${function (e) { up('password', e.target.value); }} />
              <span style="font-size:0.6875rem;color:var(--color-text-tertiary)">${t('help_reg_auth')}</span>
              <a style="font-size:0.75rem;cursor:pointer;color:var(--color-primary);margin-left:0.5rem" onClick=${function () { setShowPw(!showPw); }}>${t('reg_show')}</a></div>
          </div>
          ${testResult ? html`
            <div style="margin-top:0.5rem;padding:0.5rem;border-radius:var(--radius-sm);font-size:0.8125rem;background:${testResult.ok ? 'var(--color-success-light)' : testResult.testing ? 'var(--color-bg-tertiary)' : 'var(--color-error-light)'};color:${testResult.ok ? 'var(--color-success)' : testResult.testing ? 'var(--color-text-tertiary)' : 'var(--color-error)'}">
              ${testResult.testing ? '...' : testResult.message}
            </div>` : null}
          <div class="flex justify-end gap-3 mt-4">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('reg_cancel')}</button>
            <button class="btn btn-secondary" disabled=${busy} onClick=${handleTest}>${t('reg_test_connection')}</button>
            <button class="btn btn-primary" disabled=${busy} onClick=${handleSave}>${busy ? '...' : t('common_save')}</button>
          </div>
        </div>
      </div>`;
  }

  function _showRegModal(registry, onSave) {
    var root = document.createElement('div');
    document.body.appendChild(root);
    var prev = document.activeElement;
    function close() { pRender(null, root); root.remove(); if (prev && prev.focus) prev.focus(); }
    pRender(html`<${RegistryModal} registry=${registry} onClose=${close} onSave=${function () { onSave(); close(); }} />`, root);
  }

  // ── Connections Panel ──────────────────────────────────────────────────
  function ConnectionsPanel() {
    var dataState = useState({ list: [], loading: true });
    var data = dataState[0], setData = dataState[1];

    function loadRegs() {
      Api.get('/admin/registries').then(function (r) {
        if (!r || !r.success) { Components.showToast(t('error_registry_load'), 'error'); setData({ list: [], loading: false }); return; }
        setData({ list: r.data || [], loading: false });
      });
    }

    useEffect(function () { loadRegs(); }, []);

    function toggleEnabled(reg) {
      Api.put('/admin/registries/' + reg.id, {
        name: reg.name, url: reg.url, type: reg.type, username: reg.username || '', enabled: !reg.enabled
      }).then(function (r) { if (r && r.success) loadRegs(); });
    }

    function deleteReg(reg) {
      Components.showConfirmModal(t('reg_delete_registry') + ' "' + reg.name + '"?', function () {
        Api.delete('/admin/registries/' + reg.id).then(function (r) {
          if (r && r.success) { Components.showToast(t('reg_delete_registry'), 'success'); loadRegs(); }
          else Components.showToast((r && r.message) || t('reg_failed'), 'error');
        });
      });
    }

    if (data.loading) return html`<div class="flex-center" style="padding:2rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;

    return html`
      <div>
        <div style="display:flex;justify-content:flex-end;margin-bottom:0.75rem">
          <button class="btn btn-primary btn-sm" onClick=${function () { _showRegModal(null, loadRegs); }}>+ ${t('reg_add_registry')}</button>
        </div>
        ${!data.list.length ? html`
          <div class="empty-state">
            <button class="btn btn-primary btn-sm" onClick=${function () { _showRegModal(null, loadRegs); }}>+ ${t('reg_add_first')}</button>
            <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('reg_no_registries')}</p>
          </div>
        ` : data.list.map(function (reg) {
          var on = reg.enabled;
          return html`
            <div key=${reg.id} class="queue-item">
              <div class="flex-1 min-w-0 flex flex-col gap-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium truncate" style="color:var(--color-text)">${Helpers.escapeHtml(reg.name)}</span>
                  <span class="text-xs" style="color:var(--color-primary)">${Helpers.escapeHtml(reg.type)}</span>
                </div>
                <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(reg.url || '')}</span>
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button class="btn btn-sm" style=${{ background: on ? 'var(--color-success-light)' : 'var(--color-bg-tertiary)', color: on ? 'var(--color-success)' : 'var(--color-text-tertiary)' }}
                  onClick=${function () { toggleEnabled(reg); }} aria-label=${on ? t('reg_disable') : t('reg_enable')}>${on ? t('reg_on') : t('reg_off')}</button>
                <button class="btn btn-secondary btn-sm" onClick=${function () { _showRegModal(reg, loadRegs); }}>${t('reg_edit_registry')}</button>
                <button class="btn btn-secondary btn-sm" style="color:var(--color-error);border-color:var(--color-error)"
                  onClick=${function () { deleteReg(reg); }} aria-label=${t('reg_delete_registry')}>${t('reg_delete_registry')}</button>
              </div>
            </div>`;
        })}
      </div>`;
  }

  // ── Main Page Component ────────────────────────────────────────────────
  function RegistriesPage(props) {
    var tabState = useState(props.initialTab || 'connections');
    var activeTab = tabState[0], setActiveTab = tabState[1];
    var visState = useState({ connections: true });
    var visited = visState[0], setVisited = visState[1];

    function switchTab(tab) {
      setActiveTab(tab);
      if (!visited[tab]) setVisited(Object.assign({}, visited, { [tab]: true }));
    }

    var tabBtnStyle = function (isActive) {
      return {
        padding: '0.5rem 1rem', fontSize: '0.8125rem', border: 'none', background: 'none', cursor: 'pointer',
        borderBottom: '2px solid ' + (isActive ? 'var(--color-primary)' : 'transparent'),
        color: isActive ? 'var(--color-primary)' : 'var(--color-text-secondary)',
        fontWeight: isActive ? '500' : 'normal'
      };
    };

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _subNav() }} />
        <h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('nav_registries')}</h2>
        <div style="display:flex;gap:0.5rem;margin-bottom:1rem;border-bottom:1px solid var(--color-border)">
          ${Object.keys(_tabLabels).map(function (tabId) {
            return html`<button key=${tabId} style=${tabBtnStyle(activeTab === tabId)} onClick=${function () { switchTab(tabId); }}>${t(_tabLabels[tabId])}</button>`;
          })}
        </div>
        ${visited.connections ? html`<div style=${{ display: activeTab === 'connections' ? 'block' : 'none' }}><${ConnectionsPanel} /></div>` : null}
        ${visited.browse && typeof RegistriesRepos !== 'undefined' ? html`<div style=${{ display: activeTab === 'browse' ? 'block' : 'none' }}><${RegistriesRepos} /></div>` : null}
        ${visited.sync && typeof RegistriesSync !== 'undefined' ? html`<div style=${{ display: activeTab === 'sync' ? 'block' : 'none' }}><${RegistriesSync} active=${activeTab === 'sync'} /></div>` : null}
        ${visited.cleanup && typeof RegistriesCleanup !== 'undefined' ? html`<div style=${{ display: activeTab === 'cleanup' ? 'block' : 'none' }}><${RegistriesCleanup} /></div>` : null}
      </div>`;
  }

  // ── Module API ─────────────────────────────────────────────────────────
  var _mounted = false;
  var _rootEl = null;

  function render(subtab) {
    var el = document.getElementById('main-content');
    if (!el) return;
    destroy();
    _rootEl = el;
    _mounted = true;
    pRender(html`<${RegistriesPage} initialTab=${subtab || 'connections'} />`, el);
  }

  function destroy() {
    if (_mounted && _rootEl) {
      pRender(null, _rootEl);
      _mounted = false;
      _rootEl = null;
    }
  }

  return { render: render, destroy: destroy };
})();
