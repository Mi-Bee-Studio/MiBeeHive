const FilesQueue = (function () {
  'use strict';
  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  var STATUS_KEYS = {
    downloading: { labelKey: 'queue_downloading', cls: 'queue-status-downloading', iconKey: 'statusDownloading' },
    complete: { labelKey: 'queue_complete', cls: 'queue-status-pending', iconKey: 'statusSuccess' },
    downloaded: { labelKey: 'queue_complete', cls: 'queue-status-pending', iconKey: 'statusSuccess' },
    error: { labelKey: 'queue_failed', cls: 'queue-status-error', iconKey: 'statusError' },
    failed_permanent: { labelKey: 'queue_failed', cls: 'queue-status-permanent', iconKey: 'statusError' },
  };
  var DEFAULT_STATUS = { labelKey: 'queue_pending', cls: 'queue-status-pending', iconKey: 'statusPending' };

  function getStatusInfo(status) {
    return STATUS_KEYS[status] || DEFAULT_STATUS;
  }

  // ── Nav tabs (shared HTML) ────────────────────────────────────────
  function _nav() {
    return '<div class="module-tabs">' +
      '<a href="#/files" class="module-tab">' + t('project') + '</a>' +
      '<a href="#/files/queue" class="module-tab active">' + t('queue_status') + '</a>' +
      '<a href="#/files/crawl" class="module-tab">' + t('crawl_status_title') + '</a></div>';
  }

  // ── Stat Card ─────────────────────────────────────────────────────
  function StatCard(props) {
    return html`
      <div class="card stat-card-accent ${props.accent}" style="padding:1rem">
        <div class="text-xs font-medium uppercase tracking-wide" style="color:var(--color-text-tertiary)">${props.label}</div>
        <div class="text-2xl font-bold" style="color:var(--color-text)">${props.value}</div>
      </div>`;
  }

  // ── Queue Item ────────────────────────────────────────────────────
  function QueueItem(props) {
    var f = props.item;
    var si = getStatusInfo(f.status);

    var progressBar = null;
    if (f.status === 'downloading') {
      progressBar = html`
        <div class="dl-progress" role="status" data-dl-id="${f.id}">
          <div class="dl-progress-bar">
            <div class="dl-progress-fill" id="fq-b-${f.id}" style="width:0%"></div>
          </div>
          <span class="dl-progress-text" id="fq-t-${f.id}">0%</span>
          <span class="dl-progress-text" id="fq-i-${f.id}" style="min-width:auto;color:var(--color-text-tertiary);font-weight:var(--font-weight-normal);display:none"></span>
        </div>`;
    }

    var retryBtn = null;
    if (f.status === 'error' || f.status === 'failed_permanent') {
      retryBtn = html`
        <button class="btn btn-ghost btn-sm" onClick=${function () { props.onRetry(f.id); }}
                style="min-height:auto;padding:0.25rem 0.5rem;font-size:var(--font-size-xs)">
          <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.download }} />${t('error_retry')}
        </button>`;
    }

    return html`
      <div class="queue-item" data-id="${f.id}" data-status="${f.status}">
        <div class="flex flex-col gap-1 min-w-0 flex-1">
          <span class="text-sm font-medium truncate" style="color:var(--color-text)" title="${Helpers.escapeHtml(f.filename)}">${Helpers.escapeHtml(f.filename)}</span>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.formatBytes(f.size_bytes)}</span>
          ${progressBar}
        </div>
        <div class="flex items-center gap-2">
          ${retryBtn}
          <span class="queue-status ${si.cls}">
            <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS[si.iconKey] }} />${t(si.labelKey)}
          </span>
        </div>
      </div>`;
  }

  // ── Main Component ────────────────────────────────────────────────
  function QueueComponent() {
    var _stats = useState(null);
    var stats = _stats[0], setStats = _stats[1];
    var _files = useState([]);
    var files = _files[0], setFiles = _files[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _filterStatus = useState('');
    var filterStatus = _filterStatus[0], setFilterStatus = _filterStatus[1];

    var mountedRef = useRef(true);

    async function loadData() {
      try {
        var results = await Promise.all([
          Api.get('/files/queue/stats', { silent: true }),
          Api.get('/files/queue', { silent: true })
        ]);
        if (!mountedRef.current) return;

        var statsRes = results[0], filesRes = results[1];
        if (statsRes && statsRes.success) {
          setStats(statsRes.data);
        }
        if (filesRes && filesRes.success) {
          setFiles(filesRes.data || []);
        }
        setLoading(false);
      } catch (e) {
        if (mountedRef.current) setLoading(false);
      }
    }

    async function pollProgress() {
      try {
        var r = await Api.get('/files/queue/progress', { silent: true });
        if (!r || !r.success || !mountedRef.current) return;
        var p = r.data || {};
        var ids = Object.keys(p);
        for (var i = 0; i < ids.length; i++) {
          var id = ids[i], v = p[id];
          var b = document.getElementById('fq-b-' + id);
          var tx = document.getElementById('fq-t-' + id);
          if (b) b.style.width = v.percent + '%';
          if (tx) tx.textContent = v.percent + '%';
          var infoEl = document.getElementById('fq-i-' + id);
          if (infoEl) {
            var infoParts = [];
            if (v.speed > 0) infoParts.push(Helpers.formatBytes(v.speed) + '/s');
            if (v.eta > 0) {
              var m = Math.floor(v.eta / 60), s = v.eta % 60;
              infoParts.push(m > 0 ? m + 'm ' + s + 's' : s + 's');
            }
            if (infoParts.length) { infoEl.textContent = infoParts.join(' \u00b7 '); infoEl.style.display = ''; }
            else { infoEl.style.display = 'none'; }
          }
        }
      } catch (e) { /* ignore */ }
    }

    function handleRetry(id) {
      Api.post('/admin/files/' + id + '/retry', {}).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('retry_started') || 'Retry started', 'success');
          loadData();
        } else {
          Components.showToast(res && res.error ? res.error : (t('error') || 'Error'), 'error');
        }
      });
    }

    // Initial load + polling
    useEffect(function () {
      mountedRef.current = true;
      loadData();
      var dataTimer = setInterval(function () {
        if (mountedRef.current) loadData();
      }, 10000);
      var progressTimer = setInterval(function () {
        if (mountedRef.current) pollProgress();
      }, 3000);
      return function () {
        mountedRef.current = false;
        clearInterval(dataTimer);
        clearInterval(progressTimer);
      };
    }, []);

    // FilterBar init
    var filterContainerRef = useRef(null);
    useEffect(function () {
      if (!filterContainerRef.current) return;
      Components.FilterBar.init(filterContainerRef.current, {
        id: 'queue-filter',
        filters: [
          { key: '', label: t('common_all') },
          { key: 'downloading', label: t('queue_downloading') },
          { key: 'complete', label: t('queue_complete') },
          { key: 'error', label: t('queue_failed') }
        ],
        onChange: function (activeKey) {
          setFilterStatus(activeKey);
        }
      });
      return function () {
        Components.FilterBar.destroy('queue-filter');
      };
    }, []);

    // Apply filter + sort
    var filteredFiles = useMemo(function () {
      var result = files;
      if (filterStatus) {
        var statusMap = {
          'downloading': ['downloading'],
          'complete': ['complete', 'downloaded'],
          'error': ['error', 'failed_permanent']
        };
        var matchStatuses = statusMap[filterStatus] || [filterStatus];
        result = files.filter(function (f) { return matchStatuses.indexOf(f.status) !== -1; });
      }
      return result;
    }, [files, filterStatus]);

    // ── Loading skeleton ───────────────────────────────────────────
    if (loading) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('queue_status')}</h1>
            <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
              ${[1,2,3,4].map(function () { return html`<div class="card skeleton" style="padding:1rem;height:5rem"></div>`; })}
            </div>
            <div class="card">
              ${[1,2,3].map(function () { return html`<div class="skeleton" style="height:3rem;margin:0.5rem"></div>`; })}
            </div>
          </div>
        </div>`;
    }

    // ── Stats row ──────────────────────────────────────────────────
    var s = stats || {};
    var statsRow = html`
      <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
        <${StatCard} label=${t('queue_pending')} value=${s.pending || 0} accent="stat-accent-blue" />
        <${StatCard} label=${t('queue_downloading')} value=${s.downloading || 0} accent="stat-accent-emerald" />
        <${StatCard} label=${t('queue_complete')} value=${s.complete || 0} accent="stat-accent-emerald" />
        <${StatCard} label=${t('queue_failed')} value=${(s.error || 0) + (s.failed_permanent || 0)} accent="stat-accent-amber" />
      </div>`;

    // ── List content ───────────────────────────────────────────────
    var listContent;
    if (!filteredFiles.length) {
      listContent = html`
        <div class="empty-state">
          <p class="text-sm" style="color:var(--color-text-tertiary)">${t('queue_empty')}</p>
          <p class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.5rem">${t('queue_empty_help')}</p>
        </div>`;
    } else {
      listContent = filteredFiles.map(function (f) {
        return html`<${QueueItem} key=${f.id} item=${f} onRetry=${handleRetry} />`;
      });
    }

    return html`
      <div>
        <div dangerouslySetInnerHTML=${{ __html: _nav() }} />
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('queue_status')}</h1>
          ${statsRow}
          <div id="fq-filter-container" ref=${filterContainerRef} class="mb-4"></div>
          <div class="card">
            ${listContent}
          </div>
        </div>
      </div>`;
  }

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${QueueComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
