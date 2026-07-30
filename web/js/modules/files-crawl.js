const FilesCrawl = (function () {
  'use strict';
  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;

  var STATUS_MAP = {
    running: { dot: 'status-dot-warning', iconKey: 'statusRunning', labelKey: 'crawl_running' },
    idle: { dot: 'status-dot-neutral', iconKey: 'statusIdle', labelKey: 'crawl_idle' },
    paused: { dot: 'status-dot-neutral', iconKey: 'statusPaused', labelKey: 'crawl_paused' },
    error: { dot: 'status-dot-error', iconKey: 'statusError', labelKey: 'crawl_error' }
  };

  // ── Nav tabs (shared HTML) ────────────────────────────────────────
  var FILES_TABS = [
    { hash: '/files', i18nKey: 'nav_files', tooltipKey: 'tooltip_files' },
    { hash: '/files/queue', i18nKey: 'file_queue' },
    { hash: '/files/crawl', i18nKey: 'file_crawl' },
  ];
  function _nav() {
    return Components.moduleTabs(FILES_TABS, 'file_crawl');
  }

  // ── Status Dot ────────────────────────────────────────────────────
  function StatusDot(props) {
    var st = props.status;
    var info = STATUS_MAP[st] || STATUS_MAP.idle;
    return html`
      <span class="inline-flex items-center" style="font-size:0.8125rem">
        <span class="status-dot ${info.dot}"></span>
        <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS[info.iconKey] }} />
        <span>${t(info.labelKey)}</span>
      </span>`;
  }

  // ── Masked Token ──────────────────────────────────────────────────
  function MaskedToken(props) {
    var tk = props.token;
    if (!tk) return html`<span style="color:var(--color-text-tertiary)">---</span>`;
    if (tk.length <= 4) return html`<span>${Helpers.escapeHtml(tk)}</span>`;
    return html`
      <span>
        <span style="color:var(--color-text-tertiary);letter-spacing:2px">${'*'.repeat(tk.length - 4)}</span>
        <span>${Helpers.escapeHtml(tk.slice(-4))}</span>
      </span>`;
  }

  // ── Credential Row ────────────────────────────────────────────────
  function CredentialRow(props) {
    var c = props.credential;
    var _editing = useState(false);
    var editing = _editing[0], setEditing = _editing[1];
    var _tokenVal = useState('');
    var tokenVal = _tokenVal[0], setTokenVal = _tokenVal[1];
    var _saving = useState(false);
    var saving = _saving[0], setSaving = _saving[1];

    function handleSave() {
      if (!tokenVal.trim()) return;
      setSaving(true);
      Api.post('/admin/credentials', { source_type: c.source_type, token: tokenVal }).then(function (r) {
        setSaving(false);
        if (r && r.success) {
          Components.showToast(t('tokens_saved'), 'success');
          setEditing(false);
          setTokenVal('');
          if (props.onRefresh) props.onRefresh();
        } else {
          Components.showToast((r && r.message) || t('error'), 'error');
        }
      });
    }

    function handleDelete() {
      showConfirmModal(t('crawl_trigger_confirm'), function () {
        Api.delete('/admin/credentials?source_type=' + encodeURIComponent(c.source_type)).then(function (r) {
          if (r && r.success) {
            Components.showToast(t('tokens_saved'), 'success');
            if (props.onRefresh) props.onRefresh();
          } else {
            Components.showToast((r && r.message) || t('error'), 'error');
          }
        });
      });
    }

    return html`
      <div class="credential-row" data-src="${Helpers.escapeHtml(c.source_type)}">
        <div class="credential-info">
          <div class="flex items-center gap-2">
            <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(c.source_type) }} />
            <span class="text-sm font-medium" style="color:var(--color-text)">${Helpers.escapeHtml(c.source_type)}</span>
          </div>
          ${!editing ? html`
            <span class="credential-token" id="fc-tk-${Helpers.escapeHtml(c.source_type)}">
              <${MaskedToken} token=${c.token_masked} />
            </span>
          ` : null}
        </div>
        ${editing ? html`
          <div class="token-edit-form anim-fade-in">
            <div style="position:relative">
              <input type="password" class="input token-input" placeholder="${t('tokens_placeholder')}"
                     style="font-size:0.8125rem;padding:0.375rem 0.75rem"
                     value=${tokenVal} onInput=${function (e) { setTokenVal(e.target.value); }} />
            </div>
            <div class="flex gap-2">
              <button class="btn btn-primary btn-sm" onClick=${handleSave} disabled=${saving}>
                ${saving ? '...' : t('tokens_save')}
              </button>
              <button class="btn btn-secondary btn-sm" onClick=${function () { setEditing(false); setTokenVal(''); }}>
                ${t('tokens_cancel')}
              </button>
            </div>
          </div>
        ` : html`
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary btn-sm" onClick=${function () { setEditing(true); }}>
              ${t('tokens_edit')}
            </button>
            <button class="btn btn-danger btn-sm" onClick=${handleDelete}>
              ${t('tokens_edit')}
            </button>
          </div>
        `}
      </div>`;
  }

  // ── Project Row ───────────────────────────────────────────────────
  function ProjectRow(props) {
    var p = props.project;
    var _busy = useState(false);
    var busy = _busy[0], setBusy = _busy[1];
    var ss = p.running ? 'running' : 'paused';

    async function handleAction(url, successMsg) {
      setBusy(true);
      var r = await Api.post(url + encodeURIComponent(p.project_name));
      setBusy(false);
      if (r && r.success) {
        Components.showToast(successMsg, 'success');
        if (props.onRefresh) props.onRefresh();
      } else {
        Components.showToast((r && r.message) || t('error'), 'error');
      }
    }

    return html`
      <tr data-cn="${Helpers.escapeHtml(p.project_name)}">
        <td style="color:var(--color-text);font-weight:500">${Helpers.escapeHtml(p.project_name)}</td>
        <td><${StatusDot} status=${ss} /></td>
        <td class="text-xs">${Helpers.formatTime(p.last_crawled_at)}</td>
        <td>${p.latest_version ? Helpers.escapeHtml(p.latest_version) : html`<span style="color:var(--color-text-tertiary)">---</span>`}</td>
        <td style="text-align:right">
          <div class="flex items-center justify-end gap-1">
            <button class="btn btn-secondary btn-sm" disabled=${busy}
                    onClick=${function () { handleAction('/admin/crawl/trigger/', t('crawl_triggered')); }}>
              ${busy ? '...' : t('crawl_trigger')}
            </button>
            ${p.running ? html`
              <button class="btn btn-secondary btn-sm" disabled=${busy}
                      onClick=${function () { handleAction('/admin/crawl/pause/', t('crawl_paused_ok')); }}>
                ${t('crawl_pause')}
              </button>
            ` : html`
              <button class="btn btn-primary btn-sm" disabled=${busy}
                      onClick=${function () { handleAction('/admin/crawl/resume/', t('crawl_resumed_ok')); }}>
                ${t('crawl_resume')}
              </button>
            `}
          </div>
        </td>
      </tr>`;
  }

  // ── Main Component ────────────────────────────────────────────────
  function CrawlComponent() {
    var _projects = useState([]);
    var projects = _projects[0], setProjects = _projects[1];
    var _credentials = useState([]);
    var credentials = _credentials[0], setCredentials = _credentials[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];

    var mountedRef = useRef(true);

    async function loadCrawl() {
      try {
        var r = await Api.get('/admin/crawl/status', { silent: true });
        if (!mountedRef.current) return;
        if (!r || !r.success) {
          setError(r && r.message || t('error'));
          setLoading(false);
          return;
        }
        setProjects(Object.values(r.data || {}));
        setError(null);
        setLoading(false);
      } catch (e) {
        if (mountedRef.current) { setError(e.message || t('error')); setLoading(false); }
      }
    }

    async function loadTokens() {
      try {
        var r = await Api.get('/admin/credentials');
        if (!mountedRef.current) return;
        if (!r || !r.success) return;
        setCredentials(r.data || []);
      } catch (e) { /* ignore */ }
    }

    // Initial load + auto-refresh
    useEffect(function () {
      mountedRef.current = true;
      loadCrawl();
      loadTokens();
      var tid = setInterval(function () {
        if (mountedRef.current) loadCrawl();
      }, 15000);
      return function () {
        mountedRef.current = false;
        clearInterval(tid);
      };
    }, []);

    function handleTriggerAll() {
      showConfirmModal(t('crawl_trigger_confirm'), function (mb) {
        if (mb) { mb.disabled = true; mb.textContent = '...'; }
        Api.post('/admin/crawl/trigger-all').then(function (r) {
          if (r && r.success) {
            Components.showToast(t('crawl_triggered'), 'success');
            loadCrawl();
          } else {
            Components.showToast((r && r.message) || t('error'), 'error');
          }
        });
      });
    }

    // ── Loading skeleton ───────────────────────────────────────────
    if (loading) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('crawl_status_title')}</h1>
            <div class="flex flex-wrap gap-2 mb-4">
              <button class="btn btn-primary btn-sm">${t('crawl_trigger_all')}</button>
            </div>
            <div class="card" style="padding:1rem">
              ${[1,2,3].map(function () { return html`<div class="skeleton" style="height:3rem;margin:0.5rem"></div>`; })}
            </div>
            <h2 class="text-sm font-semibold mt-6 mb-3" style="color:var(--color-text)">${t('tokens_title')}</h2>
            <div class="card" style="padding:1rem">
              ${[1,2].map(function () { return html`<div class="skeleton" style="height:3rem;margin:0.5rem"></div>`; })}
            </div>
          </div>
        </div>`;
    }

    // ── Error state ────────────────────────────────────────────────
    if (error) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('crawl_status_title')}</h1>
            <div class="card" style="padding:1rem">
              <div class="anim-fade-in empty-state">
                <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem"
                     dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
                <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${t('error_load_failed')}</p>
                <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setError(null); loadCrawl(); }}>
                  ${t('error_retry')}
                </button>
              </div>
            </div>
          </div>
        </div>`;
    }

    // ── Projects table ─────────────────────────────────────────────
    var projectsContent;
    if (!projects.length) {
      projectsContent = html`
        <div class="empty-state">
          <p class="text-sm" style="color:var(--color-text-tertiary)">${t('crawl_no_projects')}</p>
          <div style="margin-top:0.75rem">
            <a href="#/files" class="btn btn-primary btn-sm">${t('cta_add_first_project')}</a>
          </div>
          <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_add_first_project_desc')}</p>
          <p class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.25rem">${t('crawl_empty_help')}</p>
        </div>`;
    } else {
      projectsContent = html`
        <div class="table-wrap overflow-x-auto">
          <table>
            <thead>
              <tr>
                <th>${t('project')}</th>
                <th>${t('crawl_status')}</th>
                <th>${t('crawl_last_crawl')}</th>
                <th>${t('crawl_versions_found')}</th>
                <th style="text-align:right">${t('toggle')}</th>
              </tr>
            </thead>
            <tbody>
              ${projects.map(function (p) {
                return html`<${ProjectRow} key=${p.project_name} project=${p} onRefresh=${loadCrawl} />`;
              })}
            </tbody>
          </table>
        </div>`;
    }

    // ── Credentials section ────────────────────────────────────────
    var tokensContent;
    if (!credentials.length) {
      tokensContent = html`<p class="text-sm" style="color:var(--color-text-tertiary);padding:1rem">${t('tokens_empty')}</p>`;
    } else {
      tokensContent = html`
        <div class="space-y-3">
          ${credentials.map(function (c) {
            return html`<${CredentialRow} key=${c.source_type} credential=${c} onRefresh=${loadTokens} />`;
          })}
        </div>`;
    }

    return html`
      <div>
        <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('crawl_status_title')}</h1>
          <div class="flex flex-wrap gap-2 mb-4">
            <button class="btn btn-primary btn-sm" onClick=${handleTriggerAll}>${t('crawl_trigger_all')}</button>
          </div>
          <div class="card" style="padding:1rem">${projectsContent}</div>
          <h2 class="text-sm font-semibold mt-6 mb-3" style="color:var(--color-text)">${t('tokens_title')}</h2>
          <div class="card" style="padding:1rem">${tokensContent}</div>
        </div>
      </div>`;
  }

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${CrawlComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
