const FilesProjects = (function () {
  'use strict';
  var html = PreactBridge.html;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  function _esc(s) { return Helpers.escapeHtml(s); }

  function _uniqueValues(files, key) {
    var seen = {};
    var result = [];
    files.forEach(function (f) {
      if (f[key] && !seen[f[key]]) {
        seen[f[key]] = true;
        result.push(f[key]);
      }
    });
    return result.sort();
  }

  function _applyFilters(files, version, os, arch) {
    return files.filter(function (f) {
      if (version && f.version !== version) return false;
      if (os && f.os !== os) return false;
      if (arch && f.arch !== arch) return false;
      return true;
    });
  }

  function _applySort(files, sortCol, sortDir) {
    if (!sortCol || !sortDir) return files;
    var sorted = files.slice();
    var dir = sortDir === 'asc' ? 1 : -1;
    sorted.sort(function (a, b) {
      if (sortCol === 'size_bytes') {
        return ((a.size_bytes || 0) - (b.size_bytes || 0)) * dir;
      }
      var va = (a[sortCol] || '').toLowerCase();
      var vb = (b[sortCol] || '').toLowerCase();
      if (va < vb) return -1 * dir;
      if (va > vb) return 1 * dir;
      return 0;
    });
    return sorted;
  }

  function _versionGroupKey(version) {
    if (!version) return 'unknown';
    var v = version.replace(/-.+$/, '');
    var parts = v.split('.');
    if (parts.length >= 2) return parts[0] + '.' + parts[1] + '.x';
    return version;
  }

  function _groupByVersion(files) {
    var groups = {};
    var order = [];
    files.forEach(function (f) {
      var key = _versionGroupKey(f.version || '');
      if (!groups[key]) {
        groups[key] = [];
        order.push(key);
      }
      groups[key].push(f);
    });
    return { groups: groups, order: order };
  }

  // ── Sort Header ────────────────────────────────────────────────────────
  function SortHeader(_ref) {
    var col = _ref.col, label = _ref.label, sortCol = _ref.sortCol, sortDir = _ref.sortDir, onSort = _ref.onSort;
    var isActive = sortCol === col && sortDir;
    var cls = 'th-sortable' + (isActive ? ' sort-' + sortDir + ' sort-active' : '');
    var icon = sortCol === col && sortDir === 'desc' ? ' \u25BC' : ' \u25B2';
    return html`<th class=${cls} onClick=${function () { onSort(col); }}>${label}<span class="sort-icon">${icon}</span></th>`;
  }

  // ── Version Group Header ───────────────────────────────────────────────
  function VersionGroupHeader(_ref) {
    var groupKey = _ref.groupKey, count = _ref.count, collapsed = _ref.collapsed, onToggle = _ref.onToggle, isLatest = _ref.isLatest;
    var arrow = collapsed ? '\u25B6' : '\u25BC';
    var cls = 'version-group-header' + (isLatest ? ' version-group-latest' : '');
    return html`
      <tr class=${cls} onClick=${onToggle} style="cursor:pointer">
        <td colspan="7" style="padding:0.5rem 0.75rem;font-weight:600;font-size:0.8125rem;color:var(--color-text-secondary);border-bottom:1px solid var(--color-border)">
          <span style="margin-right:0.5rem;font-size:0.7rem">${arrow}</span>
          <span>${groupKey}</span>
          <span style="margin-left:0.5rem;font-weight:400;color:var(--color-text-tertiary)">(${count} ${count === 1 ? 'file' : 'files'})</span>
          ${isLatest ? html`<span class="badge badge-sm badge-success" style="margin-left:0.5rem">latest</span>` : null}
        </td>
      </tr>`;
  }

  // ── File Row ───────────────────────────────────────────────────────────
  function FileRow(_ref) {
    var f = _ref.file, onDownload = _ref.onDownload, isLatest = _ref.isLatest;
    var canDownload = f.status === 'complete' || f.status === 'downloaded' || f.status === 'imported';
    return html`
      <tr class="version-group-file-row">
        <td style="color:var(--color-text-secondary)">
          ${_esc(f.version)}
          ${isLatest ? html`<span class="badge badge-sm badge-success" style="margin-left:0.25rem">latest</span>` : null}
        </td>
        <td style="color:var(--color-text);max-width:16rem" class="truncate" title=${f.filename}>${_esc(f.filename)}</td>
        <td>${_esc(f.os || '-')}</td>
        <td>${_esc(f.arch || '-')}</td>
        <td>${Helpers.formatBytes(f.size_bytes)}</td>
        <td dangerouslySetInnerHTML=${{ __html: Helpers.statusBadge(f.status) }}></td>
        <td>
          ${canDownload
            ? html`<button class="btn btn-secondary btn-sm"
                onClick=${function () { onDownload(f.id, f.filename); }}>
                <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.download }}></span>${t('download')}
              </button>`
            : html`<span class="text-xs" style="color:var(--color-text-tertiary)">-</span>`}
        </td>
      </tr>`;
  }

  // ── File Table ─────────────────────────────────────────────────────────
  function FileTable(_ref) {
    var files = _ref.files, sortCol = _ref.sortCol, sortDir = _ref.sortDir, onSort = _ref.onSort, onDownload = _ref.onDownload, collapsedGroups = _ref.collapsedGroups, onToggleGroup = _ref.onToggleGroup, latestVersion = _ref.latestVersion;
    if (!files || files.length === 0) {
      return html`
        <div class="empty-state">
          <div style="color:var(--color-text-quaternary)" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }}></div>
          <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-top:0.75rem">${t('no_files')}</p>
          <div style="margin-top:0.75rem"><a href="#/files/crawl" class="btn btn-primary btn-sm">${t('cta_trigger_crawl')}</a></div>
          <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_trigger_crawl_desc')}</p>
        </div>`;
    }

    var grouped = _groupByVersion(files);
    var rows = [];
    grouped.order.forEach(function (groupKey) {
      var groupFiles = grouped.groups[groupKey];
      var isCollapsed = collapsedGroups[groupKey];
      var isLatest = groupKey === _versionGroupKey(latestVersion || '');
      rows.push(html`<${VersionGroupHeader} key=${'gh-' + groupKey} groupKey=${groupKey} count=${groupFiles.length} collapsed=${isCollapsed} onToggle=${function () { onToggleGroup(groupKey); }} isLatest=${isLatest} />`);
      if (!isCollapsed) {
        groupFiles.forEach(function (f) {
          var isLatestFile = f.version === latestVersion;
          rows.push(html`<${FileRow} key=${f.id} file=${f} onDownload=${onDownload} isLatest=${isLatestFile} />`);
        });
      }
    });

    return html`
      <div class="table-responsive">
        <table>
          <thead><tr>
            <${SortHeader} col="version" label=${t('version')} sortCol=${sortCol} sortDir=${sortDir} onSort=${onSort} />
            <${SortHeader} col="filename" label=${t('file_name')} sortCol=${sortCol} sortDir=${sortDir} onSort=${onSort} />
            <th>${t('filter_os')}</th>
            <th>${t('filter_arch')}</th>
            <${SortHeader} col="size_bytes" label=${t('size')} sortCol=${sortCol} sortDir=${sortDir} onSort=${onSort} />
            <th>${t('crawl_status')}</th>
            <th>${t('download')}</th>
          </tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`;
  }

  // ── Filter Select ──────────────────────────────────────────────────────
  function FilterSelect(_ref) {
    var allLabel = _ref.allLabel, options = _ref.options, value = _ref.value, onChange = _ref.onChange;
    return html`
      <select class="input select" style="width:auto"
        value=${value} onChange=${function (e) { onChange(e.target.value); }}>
        <option value="">${allLabel}</option>
        ${options.map(function (opt) {
          return html`<option key=${opt} value=${opt}>${_esc(opt)}</option>`;
        })}
      </select>`;
  }

  // ── Pagination Container ───────────────────────────────────────────────
  function PaginationContainer(_ref) {
    var containerId = _ref.containerId;
    var ref = useRef(null);
    useEffect(function () {
      if (ref.current) {
        ref.current.id = containerId;
      }
    }, []);
    return html`<div ref=${ref} id=${containerId}></div>`;
  }

  // ── Main Project Detail Component ──────────────────────────────────────
  function ProjectDetailComponent(_ref) {
    var projectId = _ref.projectId;

    var _data = useState(null);
    var project = _data[0], setProject = _data[1];
    var _filesState = useState([]);
    var files = _filesState[0], setFiles = _filesState[1];
    var _totalState = useState(0);
    var total = _totalState[0], setTotal = _totalState[1];
    var _offsetState = useState(0);
    var offset = _offsetState[0], setOffset = _offsetState[1];
    var _loadingState = useState(true);
    var loading = _loadingState[0], setLoading = _loadingState[1];
    var _errorState = useState(null);
    var error = _errorState[0], setError = _errorState[1];
    var _filterVersionState = useState('');
    var filterVersion = _filterVersionState[0], setFilterVersion = _filterVersionState[1];
    var _filterOsState = useState('');
    var filterOs = _filterOsState[0], setFilterOs = _filterOsState[1];
    var _filterArchState = useState('');
    var filterArch = _filterArchState[0], setFilterArch = _filterArchState[1];
    var _sortColState = useState('');
    var sortCol = _sortColState[0], setSortCol = _sortColState[1];
    var _sortDirState = useState('');
    var sortDir = _sortDirState[0], setSortDir = _sortDirState[1];
    var _allFilesRef = useRef([]);
    var _collapsedState = useState({});
    var collapsedGroups = _collapsedState[0], setCollapsedGroups = _collapsedState[1];

    var pageSize = 50;

    // Load data
    var loadData = useCallback(function () {
      setLoading(true);
      setError(null);
      setOffset(0);
      setFiles([]);
      setTotal(0);
      setSortCol('');
      setSortDir('');
      setFilterVersion('');
      setFilterOs('');
      setFilterArch('');

      var projectReq = Api.get('/admin/projects/' + projectId);
      var filesReq = Api.getWithHeaders('/projects/' + projectId + '/files?limit=' + pageSize + '&offset=0', { cache: true, cacheTtl: 30000 });

      Promise.all([projectReq, filesReq]).then(function (results) {
        var projRes = results[0];
        var filesRes = results[1];

        // Distinguish "project does not exist" (e.g. after deletion, or a stale
        // URL → backend 404 "project not found") from a genuine load/network
        // failure, so the user is not shown a misleading "check your network"
        // message. (#14)
        var projectMissing = projRes && !projRes.success &&
          /not found|不存在/i.test(projRes.message || '');
        if (projectMissing) {
          setError({ notFound: true });
          setLoading(false);
          return;
        }
        if (!projRes || !projRes.success || !projRes.data) {
          setError({ message: t('error_load_failed') });
          setLoading(false);
          return;
        }

        setProject(projRes.data);
        var newFiles = [];
        var newTotal = 0;
        if (filesRes && filesRes.data && filesRes.data.success) {
          newFiles = filesRes.data.data || [];
          newTotal = filesRes.total || 0;
        }
        _allFilesRef.current = newFiles;
        setFiles(newFiles);
        setTotal(newTotal);
        setOffset(newFiles.length);
        setLoading(false);
      });
    }, [projectId]);

    useEffect(function () {
      loadData();
    }, [loadData]);

    // Render pagination when files change
    useEffect(function () {
      if (!loading && !error && project) {
        Components.renderPagination('fp-pagination', {
          offset: offset,
          limit: pageSize,
          total: total,
          onLoadMore: _loadMoreFiles
        });
      }
    }, [files, total, offset, loading, error, project]);

    function _loadMoreFiles(currentOffset) {
      if (!project) return;
      Api.getWithHeaders('/projects/' + project.id + '/files?limit=' + pageSize + '&offset=' + currentOffset).then(function (res) {
        if (!res || !res.data || !res.data.success) {
          Components.renderPagination('fp-pagination', {
            offset: offset, limit: pageSize, total: total, onLoadMore: _loadMoreFiles
          });
          return;
        }
        var newFiles = res.data.data || [];
        var allFiles = _allFilesRef.current.concat(newFiles);
        _allFilesRef.current = allFiles;
        var newOffset = offset + newFiles.length;
        if (res.total) setTotal(res.total);
        setFiles(allFiles);
        setOffset(newOffset);
      });
    }

    // Sort handler
    function handleSort(col) {
      var newCol = sortCol;
      var newDir = sortDir;
      if (sortCol === col) {
        if (sortDir === 'asc') { newDir = 'desc'; }
        else if (sortDir === 'desc') { newCol = ''; newDir = ''; }
      } else {
        newCol = col;
        newDir = 'asc';
      }
      setSortCol(newCol);
      setSortDir(newDir);
    }

    // Download handler
    function handleDownload(fileId, filename) {
      var token = Auth.getToken();
      if (!token) {
        Router.push('/login');
        return;
      }
      Components.showToast(filename ? t('download_starting') + ': ' + filename : t('download_starting'), 'success');
      window.location.href = '/api/v1/files/' + fileId + '/download?token=' + encodeURIComponent(token);
    }

    // Delete handler
    function handleDelete() {
      if (!project) return;
      var name = project.display_name || project.name;
      var pid = project.id;
      Components.showConfirmModal(t('proj_delete_confirm', { name: name }), function (modalBtn) {
        if (modalBtn) { modalBtn.disabled = true; modalBtn.textContent = '...'; }
        Api.delete('/admin/projects/' + pid).then(function (res) {
          if (!res || !res.success) {
            Components.showToast((res && res.message) || t('error'), 'error');
            // Modal is already closed by ConfirmModal.handleConfirm; nothing
            // to re-enable here.
            return;
          }
          Components.showToast(t('proj_deleted'), 'success');
          // Invalidate cached project list so the deleted project is gone.
          if (window.App && App.cache) App.cache.invalidatePattern('GET:/admin/projects');
          // Navigate back to the project list. Use location.hash directly so
          // it fires even though the confirm modal has already unmounted.
          Router.push('/files');
        }).catch(function (err) {
          Components.showToast(t('error') + ': ' + (err && err.message ? err.message : err), 'error');
        });
      });
    }

    // Retry handler
    function handleRetry() {
      loadData();
    }

    function handleToggleGroup(groupKey) {
      setCollapsedGroups(function (prev) {
        var next = Object.assign({}, prev);
        next[groupKey] = !prev[groupKey];
        return next;
      });
    }

    // Computed filter values
    var versions = useMemo(function () { return _uniqueValues(_allFilesRef.current, 'version'); }, [files]);
    var oses = useMemo(function () { return _uniqueValues(_allFilesRef.current, 'os'); }, [files]);
    var arches = useMemo(function () { return _uniqueValues(_allFilesRef.current, 'arch'); }, [files]);

    // Filtered + sorted files
    var displayFiles = useMemo(function () {
      var filtered = _applyFilters(files, filterVersion, filterOs, filterArch);
      return _applySort(filtered, sortCol, sortDir);
    }, [files, filterVersion, filterOs, filterArch, sortCol, sortDir]);

    var latestVersion = useMemo(function () {
      if (!displayFiles || displayFiles.length === 0) return '';
      var versions = displayFiles.map(function (f) { return f.version || ''; }).filter(Boolean);
      if (versions.length === 0) return '';
      versions.sort(function (a, b) { return a < b ? 1 : -1; });
      return versions[0];
    }, [displayFiles]);

    // ── Loading state ──
    if (loading) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: Components.skeletonHeading('30%') }}></div>
          <div class="mt-4" dangerouslySetInnerHTML=${{ __html: Components.skeletonTable(5, 7) }}></div>
        </div>`;
    }

    if (error) {
      var isNotFound = error && error.notFound;
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div class="anim-fade-in empty-state">
            <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }}></div>
            <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${isNotFound ? t('error_project_not_found') : (error.message || t('error_load_failed'))}</p>
            ${isNotFound
              ? html`<a class="btn btn-primary btn-sm" href="#/files">${t('back_to_files')}</a>`
              : html`<button class="btn btn-primary btn-sm retry-btn" onClick=${handleRetry}>${t('error_retry')}</button>`}
          </div>
        </div>`;
    }

    if (!project) return null;

    var name = _esc(project.display_name || project.name);
    var subName = _esc(project.name);

    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">

        <nav class="breadcrumb" aria-label="Breadcrumb">
          <a href="#/files" class="breadcrumb-item">${t('nav_files')}</a>
          <span class="breadcrumb-sep">/</span>
          <span class="breadcrumb-item active" style="min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" aria-current="page">${name}</span>
        </nav>

        <div class="flex flex-wrap items-center gap-3 mb-2">
          <h1 class="text-xl font-bold tracking-tight" style="color:var(--color-text);min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${name}</h1>
          <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(project.source_type) }}></span>
          ${project.latest_version
            ? html`<span class="badge badge-default">v${_esc(project.latest_version)}</span>`
            : null}
          <div class="flex-1"></div>
          <button class="btn btn-danger-outline btn-sm"
            onClick=${handleDelete}>
            <svg aria-hidden="true" class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0"/></svg>
            ${t('proj_delete')}
          </button>
        </div>

        <div class="flex flex-wrap gap-x-4 gap-y-1 text-xs mb-6" style="color:var(--color-text-tertiary)">
          <span>${subName}</span>
          ${total > 0 ? html`<span>${total} ${t('files')}</span>` : null}
          ${project.source_url ? html`<span>${_esc(project.source_url)}</span>` : null}
          ${project.last_crawled_at
            ? html`<span>${t('last_crawled')}: ${Helpers.formatTime(project.last_crawled_at)}</span>`
            : null}
        </div>

        ${_allFilesRef.current.length > 0
          ? html`<div class="flex flex-wrap gap-3 mb-5">
              <${FilterSelect} allLabel=${t('filter_version') + ': ' + t('filter_all')} options=${versions} value=${filterVersion} onChange=${setFilterVersion} />
              <${FilterSelect} allLabel=${t('filter_os') + ': ' + t('filter_all')} options=${oses} value=${filterOs} onChange=${setFilterOs} />
              <${FilterSelect} allLabel=${t('filter_arch') + ': ' + t('filter_all')} options=${arches} value=${filterArch} onChange=${setFilterArch} />
            </div>`
          : null}

        <div id="fp-table-container">
          <${FileTable} files=${displayFiles} sortCol=${sortCol} sortDir=${sortDir} onSort=${handleSort} onDownload=${handleDownload} collapsedGroups=${collapsedGroups} onToggleGroup=${handleToggleGroup} latestVersion=${latestVersion} />
        </div>

        <${PaginationContainer} containerId="fp-pagination" />

      </div>`;
  }

  // ── Public API ─────────────────────────────────────────────────────────
  function render(projectId) {
    var app = document.getElementById('main-content');
    if (!app) return;
    PreactBridge.render(html`<${ProjectDetailComponent} projectId=${projectId} />`, app);
  }

  return {
    render: render,
  };
})();
