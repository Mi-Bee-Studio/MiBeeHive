// Module: modules/file-center — File Center (default home page)
//
// Primary page users see: cross-project file listing with search, filters,
// pagination, a flat/tree view switcher, and a WebDAV status card.
// Data comes from T16 (GET /admin/files) and T17 (channels).
const FileCenter = (function () {
  'use strict';

  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  var PAGE_SIZE = 25;

  // ── Filter option lists ────────────────────────────────────────────────
  var OS_OPTIONS = ['linux', 'windows', 'darwin'];
  var ARCH_OPTIONS = ['amd64', 'arm64'];
  var SOURCE_OPTIONS = ['github', 'go', 'hashicorp', 'grafana', 'npm', 'pypi', 'iso'];
  var CATEGORY_OPTIONS = ['ops', 'monitoring', 'database', 'runtime', 'infra', 'container'];

  // ── Sortable table columns ─────────────────────────────────────────────
  var COLUMNS = [
    { key: 'filename', labelKey: 'file_center_filename', sortable: true },
    { key: 'version', labelKey: 'file_center_version', sortable: true },
    { key: 'os', labelKey: 'file_center_os', sortable: true },
    { key: 'arch', labelKey: 'file_center_arch', sortable: true },
    { key: 'size_bytes', labelKey: 'file_center_size', sortable: true },
    { key: 'source_type', labelKey: 'file_center_source', sortable: false },
    { key: 'actions', labelKey: 'file_center_actions', sortable: false },
  ];

  // ── Filter group (vertical sidebar) ────────────────────────────────────
  function FilterGroup(props) {
    var options = props.options || [];
    var active = props.active || '';
    var onSelect = props.onSelect;
    return html`
      <div class="mb-4">
        <div class="text-xs font-medium mb-2" style="color:var(--color-text-secondary)">${props.label}</div>
        <div class="flex flex-col gap-1">
          <button type="button" class=${'filter-btn' + (active === '' ? ' active' : '')}
                  onClick=${function () { onSelect(''); }}>${t('filter_all')}</button>
          ${options.map(function (o) {
            return html`
              <button type="button" key=${o} class=${'filter-btn' + (active === o ? ' active' : '')}
                      onClick=${function () { onSelect(o); }}>${o}</button>`;
          })}
        </div>
      </div>`;
  }

  // ── Main component ─────────────────────────────────────────────────────
  function FileCenterComponent(props) {
    var signal = props.signal;

    // Data state
    var _files = useState([]);
    var files = _files[0], setFiles = _files[1];
    var _total = useState(0);
    var total = _total[0], setTotal = _total[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var _view = useState('flat');
    var view = _view[0], setView = _view[1];

    // WebDAV + channels
    var _webdav = useState(null);
    var webdav = _webdav[0], setWebdav = _webdav[1];
    var _channels = useState([]);
    var channels = _channels[0], setChannels = _channels[1];
    var _selectedChannel = useState('');
    var selectedChannel = _selectedChannel[0], setSelectedChannel = _selectedChannel[1];

    // Search input (controlled) + ref for debounce closure
    var _searchInput = useState('');
    var searchInput = _searchInput[0], setSearchInput = _searchInput[1];
    var searchInputRef = useRef('');

    // Mutable query params read by loadFiles (kept in refs so the polling
    // closure always sees the latest values without re-subscribing).
    var filtersRef = useRef({ os: '', arch: '', category: '', source_type: '' });
    var sortRef = useRef({ field: 'filename', order: 'asc' });
    var offsetRef = useRef(0);
    var qRef = useRef('');
    var mountedRef = useRef(true);

    function buildQuery() {
      var f = filtersRef.current;
      var s = sortRef.current;
      var params = [];
      if (f.os) params.push('os=' + encodeURIComponent(f.os));
      if (f.arch) params.push('arch=' + encodeURIComponent(f.arch));
      if (f.category) params.push('category=' + encodeURIComponent(f.category));
      if (f.source_type) params.push('source_type=' + encodeURIComponent(f.source_type));
      if (qRef.current) params.push('q=' + encodeURIComponent(qRef.current));
      params.push('sort=' + encodeURIComponent(s.field));
      params.push('order=' + encodeURIComponent(s.order));
      params.push('limit=' + PAGE_SIZE);
      params.push('offset=' + offsetRef.current);
      return params.join('&');
    }

    async function loadFiles() {
      var url = '/admin/files?' + buildQuery();
      try {
        var res = await Api.getWithHeaders(url, { signal: signal, silent: true });
        if (!mountedRef.current) return;
        if (!res || !res.data || !res.data.success) {
          setError(t('file_center_error'));
          setLoading(false);
          return;
        }
        setFiles(res.data.data || []);
        setTotal(res.total || 0);
        setError(null);
        setLoading(false);
      } catch (e) {
        if (e && e.name === 'AbortError') return;
        if (mountedRef.current) { setError(t('file_center_error')); setLoading(false); }
      }
    }

    function loadWebdav() {
      Api.get('/admin/webdav/status', { signal: signal, silent: true }).then(function (res) {
        if (!mountedRef.current) return;
        if (res && res.success) setWebdav(res.data);
      });
    }

    function loadChannels() {
      Api.get('/admin/channels', { signal: signal, silent: true }).then(function (res) {
        if (!mountedRef.current) return;
        if (res && res.success) {
          var list = res.data || [];
          setChannels(list);
          if (list.length && !selectedChannel) setSelectedChannel(String(list[0].id));
        }
      });
    }

    // Initial load
    useEffect(function () {
      mountedRef.current = true;
      loadFiles();
      loadWebdav();
      loadChannels();
      return function () { mountedRef.current = false; };
    }, []);

    // Periodic refresh (5s) tied to the route's abort signal.
    Hooks.usePolling(loadFiles, 5000, { signal: signal, scope: '/', immediate: false });

    // Debounced search
    var debouncedSearch = useRef(null);
    if (!debouncedSearch.current) {
      debouncedSearch.current = Helpers.debounce(function () {
        qRef.current = searchInputRef.current.trim();
        offsetRef.current = 0;
        loadFiles();
      }, 300);
    }

    function handleSearchInput(e) {
      var v = e.target.value;
      searchInputRef.current = v;
      setSearchInput(v);
      debouncedSearch.current();
    }

    function setFilter(key, value) {
      filtersRef.current[key] = value;
      offsetRef.current = 0;
      loadFiles();
    }

    function clearFilters() {
      filtersRef.current = { os: '', arch: '', category: '', source_type: '' };
      offsetRef.current = 0;
      loadFiles();
    }

    function handleSort(field) {
      var s = sortRef.current;
      if (s.field === field) {
        s.order = s.order === 'asc' ? 'desc' : 'asc';
      } else {
        s.field = field;
        s.order = 'asc';
      }
      offsetRef.current = 0;
      loadFiles();
    }

    function goToPage(page) {
      offsetRef.current = (page - 1) * PAGE_SIZE;
      loadFiles();
    }

    function prevPage() {
      if (offsetRef.current > 0) { offsetRef.current -= PAGE_SIZE; loadFiles(); }
    }

    function nextPage() {
      if (offsetRef.current + PAGE_SIZE < total) { offsetRef.current += PAGE_SIZE; loadFiles(); }
    }

    function copyLink(file) {
      var url = window.location.origin + '/api/v1/files/' + file.public_token + '/download';
      Helpers.copyToClipboard(url).then(function () {
        Components.showToast(t('file_center_copied'), 'success');
      });
    }

    function copyWebdavUrl() {
      if (!webdav) return;
      var url = webdav.https_url || webdav.http_url || '';
      if (!url) return;
      Helpers.copyToClipboard(url).then(function () {
        Components.showToast(t('file_center_copied'), 'success');
      });
    }

    // ── Derived values ────────────────────────────────────────────────────
    var currentPage = Math.floor(offsetRef.current / PAGE_SIZE) + 1;
    var totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
    var from = total > 0 ? offsetRef.current + 1 : 0;
    var to = Math.min(offsetRef.current + PAGE_SIZE, total);
    var pageSizeSum = files.reduce(function (acc, f) { return acc + (f.size_bytes || 0); }, 0);

    // Page numbers (max 5 visible, current centered)
    var pageNumbers = useMemo(function () {
      var pages = [];
      var start = Math.max(1, currentPage - 2);
      var end = Math.min(totalPages, start + 4);
      start = Math.max(1, end - 4);
      for (var i = start; i <= end; i++) pages.push(i);
      return pages;
    }, [currentPage, totalPages]);

    // ── Render helpers ────────────────────────────────────────────────────
    function renderSortHeader(col) {
      var s = sortRef.current;
      var isActive = s.field === col.key;
      var arrow = isActive ? (s.order === 'asc' ? ' \u2191' : ' \u2193') : '';
      if (!col.sortable) {
        return html`<th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t(col.labelKey)}</th>`;
      }
      return html`
        <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">
          <button type="button" class="btn btn-ghost btn-sm" style="padding:0;font-weight:600"
                  onClick=${function () { handleSort(col.key); }}>
            ${t(col.labelKey)}${arrow}
          </button>
        </th>`;
    }

    function renderActions(file) {
      var dlUrl = '/api/v1/files/' + file.public_token + '/download';
      return html`
        <div class="flex items-center gap-2" onClick=${function (e) { e.stopPropagation(); }}>
          <a href=${dlUrl} target="_blank" rel="noopener" title=${t('file_center_download')}
             class="btn btn-icon" aria-label="${t('file_center_download')}"
             dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.download }} />
          <button type="button" class="btn btn-icon" title="${t('file_center_copy_link')}"
                  aria-label="${t('file_center_copy_link')}"
                  onClick=${function (e) { e.stopPropagation(); copyLink(file); }}
                  dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.link }} />
          <${Components.ActionMenu} items=${[
            { label: t('file_center_view_details'), onClick: function () { FileDetail.open(file); } },
            { label: t('file_center_copy_link'), onClick: function () { copyLink(file); } },
            { label: t('file_center_download'), onClick: function () { window.open(dlUrl, '_blank'); } },
          ]} />
        </div>`;
    }

    function renderTable() {
      if (loading) {
        return html`<div class="card p-4"><div dangerouslySetInnerHTML=${{ __html: Components.skeletonTable(5, 6) }} /></div>`;
      }
      if (error) {
        return html`
          <div class="card p-6">
            <div class="empty-state" role="status" aria-live="polite">
              <div style="margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
              <p class="text-sm" style="color:var(--color-text-tertiary)">${error}</p>
              <div style="margin-top:0.75rem">
                <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setError(null); loadFiles(); }}>
                  ${t('error_retry')}
                </button>
              </div>
            </div>
          </div>`;
      }
      if (!files.length) {
        return html`
          <div class="card p-6">
            <div class="empty-state" role="status" aria-live="polite">
              <div style="margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
              <p class="text-sm" style="color:var(--color-text-tertiary)">${t('file_center_no_results')}</p>
            </div>
          </div>`;
      }
      return html`
        <div class="card table-wrap">
          <table class="w-full text-sm">
            <thead>
              <tr>
                ${COLUMNS.map(function (col) { return renderSortHeader(col); })}
              </tr>
            </thead>
            <tbody>
              ${files.map(function (f) {
                return html`
                  <tr key=${f.id} data-id=${f.id} style="cursor:pointer" onClick=${function () { FileDetail.open(f); }}>
                    <td style="padding:0.625rem 0.75rem">
                      <div class="flex items-center gap-2" style="min-width:0">
                        <span style="color:var(--color-text-quaternary);flex-shrink:0"
                              dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.file }} />
                        <span class="truncate" style="color:var(--color-text)" title=${f.filename}>${f.filename}</span>
                      </div>
                    </td>
                    <td style="padding:0.625rem 0.75rem;color:var(--color-text-secondary)">${f.version || '-'}</td>
                    <td style="padding:0.625rem 0.75rem;color:var(--color-text-secondary)">${f.os || '-'}</td>
                    <td style="padding:0.625rem 0.75rem;color:var(--color-text-secondary)">${f.arch || '-'}</td>
                    <td style="padding:0.625rem 0.75rem;color:var(--color-text-secondary)">${Helpers.formatBytes(f.size_bytes)}</td>
                    <td style="padding:0.625rem 0.75rem">
                      <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(f.source_type) }} />
                    </td>
                    <td style="padding:0.625rem 0.75rem">${renderActions(f)}</td>
                  </tr>`;
              })}
            </tbody>
          </table>
        </div>`;
    }

    function renderPagination() {
      if (loading || error || !files.length) return null;
      return html`
        <div class="flex flex-wrap items-center justify-between gap-3 py-3">
          <span class="text-xs" style="color:var(--color-text-tertiary)">
            ${t('file_center_showing', { from: from, to: to, total: total })}
          </span>
          <div class="flex items-center gap-1">
            <button type="button" class="btn btn-ghost btn-sm" disabled=${offsetRef.current <= 0}
                    onClick=${prevPage}>${t('file_center_prev')}</button>
            ${pageNumbers.map(function (p) {
              return html`
                <button type="button" class=${'btn btn-sm' + (p === currentPage ? ' btn-primary' : ' btn-ghost')}
                        onClick=${function () { goToPage(p); }}>${p}</button>`;
            })}
            <button type="button" class="btn btn-ghost btn-sm" disabled=${offsetRef.current + PAGE_SIZE >= total}
                    onClick=${nextPage}>${t('file_center_next')}</button>
          </div>
        </div>`;
    }

    function renderWebdavCard() {
      var enabled = webdav && webdav.enabled;
      var url = webdav ? (webdav.https_url || webdav.http_url || '') : '';
      return html`
        <div class="card p-4 mt-2">
          <div class="flex items-center justify-between mb-3">
            <div class="text-sm font-semibold" style="color:var(--color-text)">${t('file_center_webdav_card_title')}</div>
            <span class=${'badge ' + (enabled ? 'badge-success' : 'badge-default')}>
              ${enabled ? t('file_center_webdav_active') : t('file_center_webdav_inactive')}
            </span>
          </div>
          <div class="grid gap-2 text-xs" style="color:var(--color-text-secondary)">
            <div class="flex items-center justify-between gap-2">
              <span>${t('file_center_webdav_protocol')}</span>
              <span style="color:var(--color-text)">${webdav && webdav.https_url ? 'HTTPS' : 'HTTP'}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>${t('file_center_webdav_url')}</span>
              <span class="truncate" style="color:var(--color-text);max-width:60%">${url || '-'}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>${t('file_center_webdav_storage')}</span>
              <span class="truncate" style="color:var(--color-text);max-width:60%">${webdav ? webdav.storage_path : '-'}</span>
            </div>
            <div class="flex items-center justify-between gap-2">
              <span>${t('file_center_channel')}</span>
              <select class="select" style="width:auto;font-size:0.75rem;padding:0.25rem 0.5rem"
                      value=${selectedChannel}
                      onChange=${function (e) { setSelectedChannel(e.target.value); }}>
                ${channels.length === 0 ? html`<option value="">${t('file_center_no_channels')}</option>` : null}
                ${channels.map(function (c) {
                  return html`<option key=${c.id} value=${String(c.id)}>${c.name || c.slug}</option>`;
                })}
              </select>
            </div>
          </div>
          <button type="button" class="btn btn-secondary btn-sm mt-3" disabled=${!url}
                  onClick=${copyWebdavUrl}>${t('file_center_webdav_card_copy_url')}</button>
        </div>`;
    }

    function renderTreePlaceholder() {
      return html`
        <div class="card p-6">
          <div class="empty-state" role="status" aria-live="polite">
            <div style="margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.folder }} />
            <p class="text-sm" style="color:var(--color-text-tertiary)">${t('file_center_tree_placeholder')}</p>
          </div>
        </div>`;
    }

    // ── Main layout ───────────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h1 class="text-lg font-semibold" style="color:var(--color-text)">${t('file_center_title')}</h1>
          <div class="text-xs" style="color:var(--color-text-tertiary)">
            ${t('file_center_summary', { count: total, size: Helpers.formatBytes(pageSizeSum) })}
          </div>
        </div>
        <div class="flex gap-4">
          <aside class="w-44 shrink-0 hidden md:block">
            <div class="card p-3">
              <div class="text-xs font-semibold mb-3" style="color:var(--color-text-secondary)">${t('file_center_filters')}</div>
              <${FilterGroup} label=${t('file_center_filter_os')} options=${OS_OPTIONS}
                active=${filtersRef.current.os} onSelect=${function (v) { setFilter('os', v); }} />
              <${FilterGroup} label=${t('file_center_filter_arch')} options=${ARCH_OPTIONS}
                active=${filtersRef.current.arch} onSelect=${function (v) { setFilter('arch', v); }} />
              <${FilterGroup} label=${t('file_center_filter_source')} options=${SOURCE_OPTIONS}
                active=${filtersRef.current.source_type} onSelect=${function (v) { setFilter('source_type', v); }} />
              <${FilterGroup} label=${t('file_center_filter_category')} options=${CATEGORY_OPTIONS}
                active=${filtersRef.current.category} onSelect=${function (v) { setFilter('category', v); }} />
              <button type="button" class="btn btn-ghost btn-sm w-full" onClick=${clearFilters}>
                ${t('file_center_filter_clear')}
              </button>
            </div>
          </aside>
          <div class="flex-1 min-w-0">
            <div class="flex flex-wrap items-center gap-3 mb-3">
              <div class="relative flex-1 min-w-[200px]">
                <input class="input input-search" type="search"
                       placeholder=${t('file_center_search_placeholder')}
                       value=${searchInput} onInput=${handleSearchInput}
                       aria-label="${t('file_center_search_placeholder')}" />
              </div>
              <div class="flex items-center gap-1" role="tablist" aria-label="${t('file_center_view_switch')}">
                <button type="button" role="tab" aria-selected=${view === 'flat'}
                        class=${'filter-btn' + (view === 'flat' ? ' active' : '')}
                        onClick=${function () { setView('flat'); }}>${t('file_center_flat')}</button>
                <button type="button" role="tab" aria-selected=${view === 'tree'}
                        class=${'filter-btn' + (view === 'tree' ? ' active' : '')}
                        onClick=${function () { setView('tree'); }}>${t('file_center_tree')}</button>
              </div>
            </div>
            ${view === 'tree' ? renderTreePlaceholder() : renderTable()}
            ${renderPagination()}
            ${renderWebdavCard()}
          </div>
        </div>
      </div>`;
  }

  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${FileCenterComponent} signal=${signal} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();