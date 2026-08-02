const Foraging = (function () {
  'use strict';
  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  // ── Nav tabs (shared HTML) ────────────────────────────────────────
  var FORAGING_TABS = [
    { hash: '/foraging', i18nKey: 'foraging.tab_sources', tooltipKey: 'tooltip_files' },
    { hash: '/foraging/queue', i18nKey: 'foraging.tab_queue' },
    { hash: '/foraging/credentials', i18nKey: 'foraging.tab_credentials' },
    { hash: '/foraging/projects', i18nKey: 'foraging.tab_projects' },
  ];
  function _nav() {
    return Components.moduleTabs(FORAGING_TABS, 'foraging.tab_sources');
  }

  // ── Default Tool Catalog Section ─────────────────────────────────────
  function DefaultCatalogItem(props) {
    var tool = props.tool;
    var _toggling = useState(false);
    var toggling = _toggling[0], setToggling = _toggling[1];
    var enabled = tool.enabled !== false;

    function handleToggle(e) {
      e.stopPropagation();
      if (toggling) return;
      setToggling(true);
      var endpoint = '/admin/tool-catalog/' + tool.id + (enabled ? '/disable' : '/enable');
      Api.post(endpoint, {}).then(function (res) {
        setToggling(false);
        if (!res || !res.success) {
          Components.showToast((res && res.message) || t('error'), 'error');
        } else {
          Components.showToast(t(enabled ? 'foraging.disable' : 'foraging.enable'), 'success');
          if (props.onRefresh) props.onRefresh();
        }
      }).catch(function (err) {
        setToggling(false);
        Components.showToast(t('error'), 'error');
      });
    }

    return html`
      <div class="card card-hover anim-fade-in" style="padding:1rem">
        <div class="flex items-start justify-between mb-2">
          <div style="min-width:0;flex:1">
            <span class="text-sm font-medium" style="color:var(--color-text)">${Helpers.escapeHtml(tool.name)}</span>
            <span class="text-xs" style="color:var(--color-text-tertiary);display:block;margin-top:0.25rem">
              ${Helpers.escapeHtml(tool.category || '')}
            </span>
          </div>
          <button class="btn btn-${enabled ? 'secondary' : 'primary'} btn-sm"
                  disabled=${toggling}
                  onClick=${handleToggle}>
            ${toggling ? '...' : t(enabled ? 'foraging.disable' : 'foraging.enable')}
          </button>
        </div>
        <div class="text-xs" style="color:var(--color-text-quaternary)">
          ${tool.description ? Helpers.escapeHtml(tool.description) : ''}
        </div>
      </div>`;
  }

  // ── Custom Source Row ────────────────────────────────────────────────
  function CustomSourceRow(props) {
    var source = props.source;
    var _editing = useState(false);
    var editing = _editing[0], setEditing = _editing[1];
    var _deleting = useState(false);
    var deleting = _deleting[0], setDeleting = _deleting[1];

    function handleDelete() {
      setDeleting(true);
      Api.delete('/admin/crawl-sources/' + source.id).then(function (res) {
        setDeleting(false);
        if (!res || !res.success) {
          Components.showToast((res && res.message) || t('error'), 'error');
        } else {
          Components.showToast(t('proj_deleted'), 'success');
          if (props.onRefresh) props.onRefresh();
        }
      }).catch(function (err) {
        setDeleting(false);
        Components.showToast(t('error'), 'error');
      });
    }

    return html`
      <div class="card" style="padding:1rem;margin-bottom:0.75rem">
        <div class="flex items-start justify-between">
          <div style="min-width:0;flex:1">
            <div class="flex items-center gap-2 mb-1">
              <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(source.type) }} />
              <span class="text-sm font-medium" style="color:var(--color-text)">
                ${Helpers.escapeHtml(source.name)}
              </span>
            </div>
            <span class="text-xs" style="color:var(--color-text-tertiary);display:block">
              ${source.url ? Helpers.escapeHtml(source.url) : ''}
            </span>
            ${source.storage_subdir ? html`
              <span class="text-xs" style="color:var(--color-text-quaternary);display:block">
                ${t('foraging.storage_dir')}: ${Helpers.escapeHtml(source.storage_subdir)}
              </span>
            ` : null}
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary btn-sm" disabled>${t('common_edit')}</button>
            <button class="btn btn-danger btn-sm" disabled=${deleting} onClick=${handleDelete}>
              ${deleting ? '...' : t('common_delete')}
            </button>
          </div>
        </div>
      </div>`;
  }

  // ── Main Component ────────────────────────────────────────────────
  function ForagingComponent() {
    var _activeTab = useState('sources');
    var activeTab = _activeTab[0], setActiveTab = _activeTab[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var _defaultCatalog = useState([]);
    var defaultCatalog = _defaultCatalog[0], setDefaultCatalog = _defaultCatalog[1];
    var _customSources = useState([]);
    var customSources = _customSources[0], setCustomSources = _customSources[1];
    var _credentials = useState([]);
    var credentials = _credentials[0], setCredentials = _credentials[1];
    var _projects = useState([]);
    var projects = _projects[0], setProjects = _projects[1];

    var mountedRef = useRef(true);

    async function loadDefaultCatalog() {
      try {
        var res = await Api.get('/admin/tool-catalog', { silent: true, cache: true, cacheTtl: 60000 });
        if (!mountedRef.current) return;
        if (!res || !res.success) {
          setError(t('error_load_failed'));
        } else {
          setDefaultCatalog(res.data || []);
          setError(null);
        }
      } catch (e) {
        if (mountedRef.current) setError(t('error_load_failed'));
      }
    }

    async function loadCustomSources() {
      try {
        var res = await Api.get('/admin/crawl-sources', { silent: true });
        if (!mountedRef.current) return;
        if (res && res.success) {
          setCustomSources(res.data || []);
        }
      } catch (e) { /* ignore */ }
    }

    async function loadCredentials() {
      try {
        var res = await Api.get('/admin/credentials', { silent: true });
        if (!mountedRef.current) return;
        if (res && res.success) {
          setCredentials(res.data || []);
        }
      } catch (e) { /* ignore */ }
    }

    async function loadProjects() {
      try {
        var res = await Api.get('/admin/projects', { silent: true });
        if (!mountedRef.current) return;
        if (res && res.success) {
          setProjects(res.data || []);
        }
      } catch (e) { /* ignore */ }
    }

    useEffect(function () {
      mountedRef.current = true;
      loadDefaultCatalog();
      loadCustomSources();
      loadCredentials();
      loadProjects();
      setLoading(false);
      return function () {
        mountedRef.current = false;
      };
    }, []);

    function handleRefresh() {
      loadDefaultCatalog();
      loadCustomSources();
      loadCredentials();
      loadProjects();
    }

    // ── Loading skeleton ───────────────────────────────────────────
    if (loading) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="grid gap-4" style="grid-template-columns:repeat(auto-fill,minmax(260px,1fr))">
            ${[1,2,3,4,5,6].map(function () { return html`<div class="skeleton skeleton-card"></div>`; })}
          </div>
        </div>`;
    }

    // ── Error state ────────────────────────────────────────────────
    if (error) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="anim-fade-in empty-state">
            <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem"
                 dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
            <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${error}</p>
            <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setError(null); handleRefresh(); }}>
              ${t('error_retry')}
            </button>
          </div>
        </div>`;
    }

    // ── Sources tab content ─────────────────────────────────────────────
    var sourcesContent;
    if (activeTab === 'sources') {
      var defaultCatalogContent;
      if (!defaultCatalog.length) {
        defaultCatalogContent = html`
          <div class="empty-state">
            <p class="text-sm" style="color:var(--color-text-tertiary)">${t('foraging.no_sources')}</p>
          </div>`;
      } else {
        defaultCatalogContent = defaultCatalog.map(function (tool) {
          return html`<${DefaultCatalogItem} key=${tool.id} tool=${tool} onRefresh=${loadDefaultCatalog} />`;
        });
      }

      sourcesContent = html`
        <div>
          <h2 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('foraging.default_catalog')}</h2>
          <div class="grid gap-4" style="grid-template-columns:repeat(auto-fill,minmax(280px,1fr));margin-bottom:2rem">
            ${defaultCatalogContent}
          </div>

          <div class="flex items-center justify-between mb-3">
            <h2 class="text-sm font-semibold" style="color:var(--color-text)">${t('foraging.custom_sources')}</h2>
            <button class="btn btn-primary btn-sm">${t('foraging.new_source')}</button>
          </div>
          ${customSources.map(function (source) {
            return html`<${CustomSourceRow} key=${source.id} source=${source} onRefresh=${loadCustomSources} />`;
          })}
        </div>`;
    } else if (activeTab === 'queue') {
      // Queue tab content (migrated from files-queue.js pattern)
      sourcesContent = html`
        <div class="empty-state">
          <p class="text-sm" style="color:var(--color-text-tertiary)">${t('queue_empty')}</p>
        </div>`;
    } else if (activeTab === 'credentials') {
      // Credentials tab content (migrated from files-crawl.js pattern)
      var credentialsContent;
      if (!credentials.length) {
        credentialsContent = html`<p class="text-sm" style="color:var(--color-text-tertiary);padding:1rem">${t('tokens_empty')}</p>`;
      } else {
        credentialsContent = html`
          <div class="space-y-3">
            ${credentials.map(function (c) {
              return html`
                <div class="card" style="padding:1rem">
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <span class="badge badge-default">${Helpers.escapeHtml(c.source_type)}</span>
                    </div>
                    <button class="btn btn-secondary btn-sm">${t('tokens_edit')}</button>
                  </div>
                </div>`;
            })}
          </div>`;
      }
      sourcesContent = html`
        <div>
          <h2 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('tokens_title')}</h2>
          ${credentialsContent}
        </div>`;
    } else if (activeTab === 'projects') {
      // Projects tab content (migrated from files.js pattern)
      var projectsContent;
      if (!projects.length) {
        projectsContent = html`
          <div class="empty-state">
            <p class="text-sm" style="color:var(--color-text-tertiary)">${t('proj_empty')}</p>
          </div>`;
      } else {
        projectsContent = html`
          <div class="grid gap-4" style="grid-template-columns:repeat(auto-fill,minmax(260px,1fr))">
            ${projects.map(function (p) {
              return html`
                <div class="card card-hover" style="padding:1rem">
                  <div class="flex items-start justify-between mb-2">
                    <span class="text-sm font-medium" style="color:var(--color-text)">
                      ${Helpers.escapeHtml(p.display_name || p.name)}
                    </span>
                    <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(p.source_type) }} />
                  </div>
                  <div class="text-xs" style="color:var(--color-text-tertiary)">
                    ${p.file_count || 0} ${t('files')}
                  </div>
                </div>`;
            })}
          </div>`;
      }
      sourcesContent = html`
        <div>
          <h2 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('nav_projects')}</h2>
          ${projectsContent}
        </div>`;
    }

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
        ${sourcesContent}
      </div>`;
  }

  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${ForagingComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();