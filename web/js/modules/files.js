const Files = (function () {
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
  var FILES_TABS = [
    { hash: '/files', i18nKey: 'nav_files', tooltipKey: 'tooltip_files' },
    { hash: '/files/queue', i18nKey: 'file_queue' },
    { hash: '/files/crawl', i18nKey: 'file_crawl' },
  ];
  function _nav() {
    return Components.moduleTabs(FILES_TABS, 'nav_files');
  }

  var SORT_OPTIONS = ['name', 'file_count', 'last_activity'];

  // Crawl statuses that mean the latest crawl did not produce files.
  var FAILED_CRAWL_STATUSES = { error: 1, network_error: 1, rate_limited: 1 };

  // ── Project Card ──────────────────────────────────────────────────
  function ProjectCard(props) {
    var p = props.project;
    var enabled = p.enabled !== false;
    var name = Helpers.escapeHtml(p.display_name || p.name);
    var fileCount = p.file_count || 0;
    var crawlFailed = FAILED_CRAWL_STATUSES[p.last_crawl_status] ? true : false;
    var crawlError = crawlFailed ? Helpers.escapeHtml(p.last_crawl_error || p.last_crawl_status) : '';
    var _toggling = useState(false);
    var toggling = _toggling[0], setToggling = _toggling[1];

    function handleToggle(e) {
      e.stopPropagation();
      if (toggling) return;
      setToggling(true);
      Api.post('/admin/projects/' + p.id + '/toggle', {}).then(function (res) {
        setToggling(false);
        if (!res || !res.success) {
          Components.showToast((res && res.message) || t('error'), 'error');
        } else {
          Components.showToast(t('proj_toggled'), 'success');
        }
      });
    }

    function handleClick() {
      Router.push('/files/projects/' + p.id);
    }

    return html`
      <div class="card card-hover anim-fade-in" style="padding:1rem;cursor:pointer;transition:box-shadow 0.15s"
           onClick=${handleClick}>
        <div class="flex items-start justify-between mb-2">
          <div class="flex items-center gap-2" style="min-width:0">
            <div style="color:var(--color-text-quaternary);flex-shrink:0"
                 dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.folder }} />
            <span class="font-medium text-sm truncate" style="color:var(--color-text)">${name}</span>
          </div>
          <label class="toggle-switch" style="flex-shrink:0" onClick=${function (e) { e.stopPropagation(); }}>
            <input type="checkbox" class="files-toggle" role="switch"
                   checked=${enabled} aria-checked="${enabled ? 'true' : 'false'}"
                   disabled=${toggling}
                   onChange=${handleToggle} />
            <span class="toggle-slider"></span>
          </label>
        </div>
        <div class="flex items-center gap-2 mb-3">
          <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(p.source_type) }} />
          <span class="text-xs" style="color:var(--color-text-tertiary)">
            ${fileCount} ${t('files')}
          </span>
        </div>
        ${crawlFailed ? html`
          <div class="flex items-start gap-1 text-xs mb-3" style="color:var(--color-error)"
               title=${crawlError}>
            <span aria-hidden="true">⚠</span>
            <span class="truncate">${t('proj_last_crawl_failed')}: ${crawlError}</span>
          </div>
        ` : null}
        <div class="flex items-center gap-1 text-xs" style="color:var(--color-text-quaternary)">
          <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.clock }} />
          <span>${Helpers.formatTime(p.last_crawled_at)}</span>
        </div>
      </div>`;
  }

  // ── New Project Modal Content ─────────────────────────────────────
  function NewProjectForm(props) {
    var _name = useState('');
    var name = _name[0], setName = _name[1];
    var _displayName = useState('');
    var displayName = _displayName[0], setDisplayName = _displayName[1];
    var _sourceType = useState('');
    var sourceType = _sourceType[0], setSourceType = _sourceType[1];
    var _interval = useState('60');
    var interval = _interval[0], setInterval2 = _interval[1];
    var _sourceUrl = useState('');
    var sourceUrl = _sourceUrl[0], setSourceUrl = _sourceUrl[1];
    var _owner = useState('');
    var owner = _owner[0], setOwner = _owner[1];
    var _repo = useState('');
    var repo = _repo[0], setRepo = _repo[1];
    var _submitting = useState(false);
    var submitting = _submitting[0], setSubmitting = _submitting[1];
    var _errors = useState({});
    var errors = _errors[0], setErrors = _errors[1];

    var nameRef = useRef(null);
    useEffect(function () {
      if (nameRef.current) nameRef.current.focus();
    }, []);

    // Determine visibility of owner/repo fields
    var showOwner = sourceType === 'github' || sourceType === 'grafana' || sourceType === 'npm';
    var showRepo = sourceType === 'github' || sourceType === 'grafana' || sourceType === 'npm' || sourceType === 'pypi' || sourceType === 'crates';

    var ownerLabel, repoLabel, ownerPlaceholder, repoPlaceholder;
    if (sourceType === 'npm') {
      ownerLabel = t('proj_npm_scope');
      repoLabel = t('proj_npm_package');
      ownerPlaceholder = '@types';
      repoPlaceholder = 'node';
    } else if (sourceType === 'pypi') {
      repoLabel = t('proj_pypi_package');
      repoPlaceholder = 'requests';
    } else if (sourceType === 'crates') {
      repoLabel = t('proj_crates_name');
      repoPlaceholder = 'serde';
    } else {
      ownerLabel = t('proj_github_owner');
      repoLabel = t('proj_github_repo');
      ownerPlaceholder = 'facebook';
      repoPlaceholder = 'react';
    }

    function handleSubmit() {
      var newErrors = {};
      if (!name.trim()) newErrors.name = t('validation_required');
      if (!sourceType) newErrors.sourceType = t('proj_source_type_required');

      if (sourceType === 'github' || sourceType === 'grafana') {
        if (!owner.trim()) newErrors.owner = t('validation_required');
        if (!repo.trim()) newErrors.repo = t('validation_required');
      } else if (sourceType === 'npm') {
        if (!repo.trim()) newErrors.repo = t('validation_required');
      } else if (sourceType === 'pypi' || sourceType === 'crates') {
        if (!repo.trim()) newErrors.repo = t('validation_required');
      }

      setErrors(newErrors);
      if (Object.keys(newErrors).length) return;

      setSubmitting(true);
      var body = {
        name: name.trim(),
        display_name: displayName.trim() || name.trim(),
        source_type: sourceType,
        source_url: sourceUrl.trim(),
        settings: {
          crawl_interval: parseInt(interval) || 60,
          github_owner: owner.trim(),
          github_repo: repo.trim()
        }
      };

      Api.post('/admin/projects', body).then(function (res) {
        setSubmitting(false);
        if (!res || !res.success) {
          Components.showToast((res && res.message) || t('error'), 'error');
          return;
        }
        Components.showToast(t('proj_created'), 'success');
        if (props.onCreated) props.onCreated(res.data);
        if (props.onClose) props.onClose();
      });
    }

    return html`
      <div class="grid gap-4">
        <div>
          <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('proj_name')}</label>
          <input ref=${nameRef} class="input" placeholder="nodejs, kubernetes"
                 value=${name} onInput=${function (e) { setName(e.target.value); }} />
          ${errors.name ? html`<span class="text-xs" style="color:var(--color-error)">${errors.name}</span>` : html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_name')}</span>`}
        </div>
        <div>
          <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('proj_display_name')}</label>
          <input class="input" placeholder="Node.js"
                 value=${displayName} onInput=${function (e) { setDisplayName(e.target.value); }} />
          <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_display_name')}</span>
        </div>
        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('source_type')}</label>
            <select class="input select" value=${sourceType}
                    onChange=${function (e) { setSourceType(e.target.value); }}>
              <option value="" disabled selected>--</option>
              <option value="github">GitHub</option>
              <option value="go">Go</option>
              <option value="hashicorp">HashiCorp</option>
              <option value="grafana">Grafana</option>
              <option value="npm">NPM</option>
              <option value="pypi">PyPI</option>
              <option value="crates">Crates</option>
            </select>
            ${errors.sourceType ? html`<span class="text-xs" style="color:var(--color-error)">${errors.sourceType}</span>` : html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_source_type')}</span>`}
          </div>
          <div>
            <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('proj_crawl_interval')}</label>
            <input type="number" min="1" class="input" value=${interval}
                   onInput=${function (e) { setInterval2(e.target.value); }} />
            <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_interval')}</span>
          </div>
        </div>
        <div>
          <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('proj_source_url')}</label>
          <input class="input" placeholder="https://..."
                 value=${sourceUrl} onInput=${function (e) { setSourceUrl(e.target.value); }} />
          <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_source_url')}</span>
        </div>
        ${showOwner || showRepo ? html`
          <div class="grid grid-cols-2 gap-3">
            ${showOwner ? html`
              <div>
                <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${ownerLabel}</label>
                <input class="input" placeholder=${ownerPlaceholder}
                       value=${owner} onInput=${function (e) { setOwner(e.target.value); }} />
                ${errors.owner ? html`<span class="text-xs" style="color:var(--color-error)">${errors.owner}</span>` : html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_github')}</span>`}
              </div>
            ` : null}
            ${showRepo ? html`
              <div>
                <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${repoLabel}</label>
                <input class="input" placeholder=${repoPlaceholder}
                       value=${repo} onInput=${function (e) { setRepo(e.target.value); }} />
                ${errors.repo ? html`<span class="text-xs" style="color:var(--color-error)">${errors.repo}</span>` : html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_proj_github')}</span>`}
              </div>
            ` : null}
          </div>
        ` : null}
        <div class="flex justify-end gap-3 mt-6">
          <button class="btn btn-secondary" onClick=${props.onClose}>${t('cancel')}</button>
          <button class="btn btn-primary" onClick=${handleSubmit} disabled=${submitting}>
            ${submitting ? '...' : t('save')}
          </button>
        </div>
      </div>`;
  }

  // ── New Project Modal (Preact-based) ──────────────────────────────
  function NewProjectModal(props) {
    var ref = useRef(null);
    var previousFocusRef = useRef(null);

    useEffect(function () {
      previousFocusRef.current = document.activeElement;
    }, []);

    function close() {
      if (previousFocusRef.current && previousFocusRef.current.focus) {
        previousFocusRef.current.focus();
      }
      if (props.onClose) props.onClose();
    }

    function handleOverlayClick(e) {
      if (e.target === ref.current) close();
    }

    function handleKeyDown(e) {
      if (e.key === 'Escape') { close(); return; }
    }

    return html`
      <div ref=${ref} class="modal-overlay" onClick=${handleOverlayClick} onKeyDown=${handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" style=${{ maxWidth: '32rem' }}>
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('proj_add')}</h3>
          <${NewProjectForm} onClose=${close} onCreated=${props.onCreated} />
        </div>
      </div>`;
  }

  // ── Main Component ────────────────────────────────────────────────
  function FilesComponent() {
    var _projects = useState([]);
    var projects = _projects[0], setProjects = _projects[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var _sortBy = useState('name');
    var sortBy = _sortBy[0], setSortBy = _sortBy[1];
    var _showModal = useState(false);
    var showModal = _showModal[0], setShowModal = _showModal[1];

    var mountedRef = useRef(true);

    async function loadProjects() {
      Cache.invalidate('GET:/admin/projects');
      try {
        var res = await Api.get('/admin/projects', { silent: true, cache: true, cacheTtl: 60000 });
        if (!mountedRef.current) return;
        if (!res || !res.success) {
          setError(t('error_load_failed'));
          setLoading(false);
          return;
        }
        setProjects(res.data || []);
        setError(null);
        setLoading(false);
      } catch (e) {
        if (mountedRef.current) { setError(t('error_load_failed')); setLoading(false); }
      }
    }

    useEffect(function () {
      mountedRef.current = true;
      loadProjects();
      return function () {
        mountedRef.current = false;
      };
    }, []);

    // Sort projects
    var sorted = useMemo(function () {
      var enabled = projects.filter(function (p) { return p.enabled !== false; });
      return enabled.slice().sort(function (a, b) {
        if (sortBy === 'file_count') return (b.file_count || 0) - (a.file_count || 0);
        if (sortBy === 'last_activity') {
          var ta = a.last_crawled_at || '', tb = b.last_crawled_at || '';
          return tb < ta ? -1 : tb > ta ? 1 : 0;
        }
        var na = (a.display_name || a.name || '').toLowerCase();
        var nb = (b.display_name || b.name || '').toLowerCase();
        return na < nb ? -1 : na > nb ? 1 : 0;
      });
    }, [projects, sortBy]);

    function handleProjectCreated(project) {
      setProjects(function (prev) { return prev.concat([project]); });
    }

    // ── Loading skeleton ───────────────────────────────────────────
    if (loading) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="flex flex-wrap items-center gap-3 mb-5">
            <select class="select" style="width:auto;font-size:0.8125rem" disabled>
              <option>${t('sort_by_name')}</option>
            </select>
            <button class="btn btn-primary" style="white-space:nowrap" disabled>${t('proj_add')}</button>
          </div>
          <div class="grid gap-4" style="grid-template-columns:repeat(auto-fill,minmax(260px,1fr))">
            ${[1,2,3].map(function () { return html`<div class="skeleton skeleton-card"></div>`; })}
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
            <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setError(null); loadProjects(); }}>
              ${t('error_retry')}
            </button>
          </div>
        </div>`;
    }

    // ── Main content ───────────────────────────────────────────────
    var gridContent;
    if (!sorted.length) {
      gridContent = html`
        <div style="grid-column:1/-1">
          <div class="empty-state" role="status" aria-live="polite">
            <div style="margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
            <p class="text-sm" style="color:var(--color-text-tertiary)">${t('proj_empty')}</p>
            <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_add_first_project_desc')}</p>
            <div style="margin-top:0.75rem">
              <button class="btn btn-primary btn-sm" data-action="empty-state-action"
                      onClick=${function () { setShowModal(true); }}>${t('cta_add_first_project')}</button>
            </div>
          </div>
          <p class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.5rem">${t('files_empty_help')}</p>
        </div>`;
    } else {
      gridContent = sorted.map(function (p) {
        return html`<${ProjectCard} key=${p.id} project=${p} />`;
      });
    }

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
        <div class="flex flex-wrap items-center gap-3 mb-5">
          <select class="select" style="width:auto;font-size:0.8125rem"
                  value=${sortBy} onChange=${function (e) { setSortBy(e.target.value); }}>
            <option value="name">${t('sort_by_name')}</option>
            <option value="file_count">${t('sort_by_file_count')}</option>
            <option value="last_activity">${t('sort_by_last_activity')}</option>
          </select>
          <button class="btn btn-primary" style="white-space:nowrap"
                  onClick=${function () { setShowModal(true); }}>${t('proj_add')}</button>
        </div>
        <div class="grid gap-4" style="grid-template-columns:repeat(auto-fill,minmax(260px,1fr))">
          ${gridContent}
        </div>
        ${showModal ? html`<${NewProjectModal}
          onClose=${function () { setShowModal(false); }}
          onCreated=${handleProjectCreated}
        />` : null}
      </div>`;
  }

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${FilesComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
