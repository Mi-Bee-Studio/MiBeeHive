const Containers = (function () {
  'use strict';

  var html = PreactBridge.html;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  var PS = 50;

  // ── Sub-nav shared tabs ───────────────────────────────────────────────

  function SubNav(props) {
    var tabs = [
      { key: 'containers', href: '#/containers', label: t('nav_containers') },
      { key: 'images', href: '#/containers/images', label: t('nav_containers_images') },
      { key: 'templates', href: '#/containers/templates', label: t('nav_containers_templates') },
      { key: 'registries', href: '#/containers/registries', label: t('nav_registries') }
    ];
    return html`
      <nav class="module-tabs">
        ${tabs.map(function (tab) {
          return html`<a href=${tab.href} class=${'module-tab' + (props.active === tab.key ? ' active' : '')}>${tab.label}</a>`;
        })}
      </nav>
    `;
  }

  // ── Status helpers ────────────────────────────────────────────────────

  function statusInfo(s) {
    s = (s || '').toLowerCase();
    var r = s === 'running';
    return {
      cls: r ? 'status-running' : 'status-stopped',
      color: r ? 'var(--color-success)' : 'var(--color-text-tertiary)',
      dot: r ? 'status-dot-success' : 'status-dot-neutral',
      label: t('container_status_' + s) || t('container_unknown_status')
    };
  }

  // ── Action button ─────────────────────────────────────────────────────

  function ActionBtn(props) {
    var loadingState = useState(false);
    var loading = loadingState[0], setLoading = loadingState[1];
    var textRef = useRef(props.label);

    var handleClick = useCallback(function () {
      if (props.action === 'delete') {
        Components.showConfirmModal(t('common_delete') + '?', function () {
          execAction(props.id, 'delete');
        });
        return;
      }
      execAction(props.id, props.action);
    }, [props.id, props.action]);

    async function execAction(id, action) {
      setLoading(true);
      try {
        var res = action === 'delete'
          ? await Api.delete('/admin/containers/' + id)
          : await Api.post('/admin/containers/' + id + '/' + action);
        if (res && res.success) {
          showToast(t('container_' + action + 'ed'), 'success');
          if (props.onDone) props.onDone();
        }
      } finally {
        setLoading(false);
      }
    }

    if (!props.enabled) {
      return html`<button class="btn btn-sm" disabled style="opacity:0.3;cursor:default" aria-label=${props.label}>${props.label}</button>`;
    }

    return html`
      <button
        class=${'btn btn-sm ' + (props.cls || 'btn-secondary')}
        onClick=${handleClick}
        disabled=${loading}
        aria-label=${props.label}
      >
        ${loading ? html`<div class="spinner" style="width:0.875rem;height:0.875rem;border-width:2px;display:inline-block;vertical-align:middle"></div>` : props.label}
      </button>
    `;
  }

  // ── Container item ────────────────────────────────────────────────────

  function ContainerItem(props) {
    var c = props.item;
    var si = statusInfo(c.status);
    var isRun = (c.status || '').toLowerCase() === 'running';
    var ports = '';
    if (c.ports && c.ports.length) {
      ports = c.ports.map(function (p) { return Helpers.escapeHtml(p.public_port + ':' + p.private_port + '/' + p.type); }).join(', ');
    }
    var eh = Helpers.escapeHtml;

    return html`
      <div class="queue-item" data-id=${eh(c.id)} data-status=${eh(c.status || '')}>
        <div class="flex flex-col gap-1 min-w-0 flex-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium truncate" style="color:var(--color-text)" title=${eh(c.name || '')}>${eh(c.name || c.id.substr(0, 12))}</span>
            <span class="inline-flex items-center queue-status ${si.cls}" style=${'background:' + si.color + '20;color:' + si.color}>
              <span class="status-dot ${si.dot}"></span>${si.label}
            </span>
          </div>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${eh(c.image || '')}${ports ? ' \u00b7 ' + ports : ''}</span>
        </div>
        <div class="flex items-center gap-1 flex-shrink-0">
          <${ActionBtn} id=${c.id} action="start" label=${t('container_action_start')} enabled=${!isRun} onDone=${props.onRefresh} />
          <${ActionBtn} id=${c.id} action="stop" label=${t('container_action_stop')} enabled=${isRun} cls="btn-secondary" onDone=${props.onRefresh} />
          <${ActionBtn} id=${c.id} action="restart" label=${t('container_action_restart')} enabled=${true} cls="btn-secondary" onDone=${props.onRefresh} />
          <${ActionBtn} id=${c.id} action="delete" label=${'\u00d7'} enabled=${true} cls="btn-secondary" onDone=${props.onRefresh}
            ariaLabel=${t('container_action_delete')}
            style=${'color:var(--color-error);border-color:var(--color-error)'} />
        </div>
      </div>
    `;
  }

  // ── Empty state ───────────────────────────────────────────────────────

  function EmptyState(props) {
    var ctaRef = useRef(null);
    useEffect(function () {
      if (ctaRef.current) ctaRef.current.addEventListener('click', props.onCta);
    }, []);
    return html`
      <div class="empty-state">
        <p class="text-sm" style="color:var(--color-text-tertiary)">${props.message}</p>
        <div style="margin-top:0.75rem">
          <button ref=${ctaRef} class="btn btn-primary btn-sm">${t('cta_create_container')}</button>
        </div>
        <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_create_container_desc')}</p>
        <p class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.25rem">${t('containers_empty_docker')}</p>
      </div>
    `;
  }

  // ── Create container modal ────────────────────────────────────────────

  function showCreateModal(onDone) {
    var errStyle = 'display:none;color:var(--color-error);font-size:0.75rem;margin-top:0.25rem';
    var helpStyle = 'class="text-xs" style="color:var(--color-text-tertiary)"';
    var labelCls = 'text-xs font-medium';
    var inputCls = 'class="input"';

    var bodyHtml = '<div class="flex flex-col gap-3">' +
      '<div><label class="' + labelCls + '" style="color:var(--color-text-secondary)">' + t('container_image') + ' *</label><input id="ct-img" ' + inputCls + ' placeholder="nginx:latest"><span ' + helpStyle + '>' + t('help_ct_image') + '</span></div>' +
      '<div><label class="' + labelCls + '" style="color:var(--color-text-secondary)">' + t('container_name') + ' *</label><input id="ct-name" ' + inputCls + ' placeholder="my-container"><span ' + helpStyle + '>' + t('help_ct_name') + '</span></div>' +
      '<div><label class="' + labelCls + '" style="color:var(--color-text-secondary)">' + t('container_ports') + '</label><input id="ct-port" ' + inputCls + ' placeholder="8080:80"><span ' + helpStyle + '>' + t('help_ct_port') + '</span><p id="ct-port-err" style="' + errStyle + '"></p></div>' +
      '<div><label class="' + labelCls + '" style="color:var(--color-text-secondary)">' + t('container_name') + ' KEY=VAL</label><textarea id="ct-env" ' + inputCls + ' rows="2"></textarea><span ' + helpStyle + '>' + t('help_ct_env') + '</span><p id="ct-env-err" style="' + errStyle + '"></p></div>' +
      '</div><div class="flex justify-end gap-3 mt-4"><button id="ct-cancel" class="btn btn-secondary">' + t('cancel') + '</button><button id="ct-ok" class="btn btn-primary">' + t('confirm') + '</button></div>';

    function setFieldError(inputEl, errEl, msg) {
      if (msg) {
        inputEl.style.borderColor = 'var(--color-error)';
        errEl.textContent = msg;
        errEl.style.display = '';
      } else {
        inputEl.style.borderColor = '';
        errEl.textContent = '';
        errEl.style.display = 'none';
      }
    }

    var modal = Components.createModal({
      title: t('container_create'),
      bodyHtml: bodyHtml,
      onMount: function (overlay) {
        overlay.querySelector('#ct-cancel').addEventListener('click', modal.close);

        var portInput = overlay.querySelector('#ct-port');
        var portErr = overlay.querySelector('#ct-port-err');
        var envInput = overlay.querySelector('#ct-env');
        var envErr = overlay.querySelector('#ct-env-err');

        if (portInput) portInput.addEventListener('input', function () {
          var v = portInput.value.trim();
          if (!v) { setFieldError(portInput, portErr, ''); return; }
          var portRe = /^\d{1,5}:\d{1,5}(\/\w+)?$/;
          setFieldError(portInput, portErr, portRe.test(v) ? '' : t('validation_port_format'));
        });
        if (envInput) envInput.addEventListener('input', function () {
          var v = envInput.value.trim();
          if (!v) { setFieldError(envInput, envErr, ''); return; }
          var lines = v.split('\n').filter(function (l) { return l.trim(); });
          var envRe = /^[A-Za-z_][A-Za-z0-9_]*=.+$/;
          var hasInvalid = lines.some(function (l) { return !envRe.test(l.trim()); });
          setFieldError(envInput, envErr, hasInvalid ? t('validation_env_format') : '');
        });

        overlay.querySelector('#ct-ok').addEventListener('click', async function () {
          var img = document.getElementById('ct-img').value.trim();
          var nm = document.getElementById('ct-name').value.trim();
          if (!img || !nm) { showToast(t('container_image_name_required'), 'error'); return; }
          var pt = document.getElementById('ct-port').value.trim();
          if (pt && portErr && portErr.textContent) return;
          if (envInput && envErr && envErr.textContent) return;
          var b = { image: img, name: nm };
          if (pt) b.ports = [pt];
          var ev = document.getElementById('ct-env').value.trim();
          if (ev) b.env = ev.split('\n').filter(function (l) { return l.trim(); });
          var r = await Api.post('/admin/containers', b);
          modal.close();
          if (r && r.success) { showToast(t('container_created'), 'success'); onDone(); }
        });
        overlay.querySelector('#ct-img').focus();
      }
    });
  }

  // ── Main page component ───────────────────────────────────────────────

  function ContainersPage() {
    var dataState = useState({ containers: [], total: 0, offset: 0, loading: true, error: null, dockerUnavailable: false });
    var data = dataState[0], setData = dataState[1];

    var filterState = useState('');
    var filterStatus = filterState[0], setFilterStatus = filterState[1];

    var filterBarRef = useRef(null);
    var timerRef = useRef(null);

    async function loadData() {
      var res = await Api.getWithHeaders('/admin/containers?limit=' + PS + '&offset=0', { silent: true });
      if (!res || !res.data || !res.data.success) {
        var isDockerUnavailable = res && res.data && res.data.error_code === 'DOCKER_UNAVAILABLE';
        setData(function (prev) {
          return Object.assign({}, prev, { loading: false, error: isDockerUnavailable ? null : t('error_load_failed'), dockerUnavailable: isDockerUnavailable });
        });
        return;
      }
      var items = res.data.data || [];
      var total = res.total || 0;
      setData({ containers: items, total: total, offset: items.length, loading: false, error: null, dockerUnavailable: false });
    }

    useEffect(function () {
      loadData();
      timerRef.current = setInterval(loadData, 10000);
      App.addTimer(timerRef.current);
      return function () { App.clearAllTimers(); };
    }, []);

    var filtered = useMemo(function () {
      var result = data.containers;
      if (filterStatus) {
        result = result.filter(function (c) {
          var isRunning = (c.status || '').toLowerCase() === 'running';
          return filterStatus === 'running' ? isRunning : !isRunning;
        });
      }
      result = result.slice().sort(function (a, b) {
        var na = (a.name || a.id || '').toLowerCase();
        var nb = (b.name || b.id || '').toLowerCase();
        return na < nb ? -1 : na > nb ? 1 : 0;
      });
      return result;
    }, [data.containers, filterStatus]);

    // Filter bar
    useEffect(function () {
      if (!filterBarRef.current) return;
      Components.FilterBar.init(filterBarRef.current, {
        id: 'containers-filter',
        filters: [
          { key: '', label: t('common_all') },
          { key: 'running', label: t('filter_running') },
          { key: 'stopped', label: t('filter_stopped') }
        ],
        onChange: function (activeKey) { setFilterStatus(activeKey); }
      });
      return function () { Components.FilterBar.destroy('containers-filter'); };
    }, []);

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <${SubNav} active="containers" />
        <div class="flex items-center justify-between mb-4">
          <h1 class="text-xl font-bold tracking-tight" style="color:var(--color-text)" data-tooltip=${t('tooltip_containers')}>${t('nav_containers')}</h1>
          <button class="btn btn-primary btn-sm" onClick=${function () { showCreateModal(loadData); }}>${t('cta_create_container')}</button>
        </div>
        <div ref=${filterBarRef} class="mb-4"></div>
        <div class="card">
          <div>
            ${data.loading ? html`
              <div class="skeleton" style="height:3rem;margin:0.5rem"></div>
              <div class="skeleton" style="height:3rem;margin:0.5rem"></div>
              <div class="skeleton" style="height:3rem;margin:0.5rem"></div>
            ` : data.dockerUnavailable ? html`
              <${EmptyState} message=${t('error_docker_unavailable')} />
            ` : data.error ? html`
              <div class="anim-fade-in empty-state">
                <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem"
                     dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
                <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${data.error}</p>
                <button class="btn btn-primary btn-sm" onClick=${loadData}>${t('error_retry')}</button>
              </div>
            ` : filtered.length === 0 ? html`
              <${EmptyState} message=${t('error_docker_unavailable')} onCta=${function () { showCreateModal(loadData); }} />
            ` : filtered.map(function (c) {
              return html`<${ContainerItem} key=${c.id} item=${c} onRefresh=${loadData} />`;
            })}
          </div>
        </div>
      </div>
    `;
  }

  // ── Module API ────────────────────────────────────────────────────────

  var _mounted = false;
  var _rootEl = null;

  function renderPage() {
    var el = document.getElementById('main-content');
    if (!el) return;
    _rootEl = el;
    _mounted = true;
    render(html`<${ContainersPage} />`, el);
  }

  function cleanup() {
    if (_mounted && _rootEl) {
      render(null, _rootEl);
      _mounted = false;
      _rootEl = null;
    }
  }

  return { render: renderPage, cleanup: cleanup };
})();
