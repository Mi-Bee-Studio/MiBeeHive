var Tasks = (function () {
  'use strict';
  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  var TYPE_ORDER = ['crawl', 'download', 'iso_check', 'backup'];
  var TYPE_BADGE = { crawl: 'badge-blue', download: 'badge-cyan', iso_check: 'badge-purple', backup: 'badge-orange' };
  var TYPE_LABELS = { crawl: 'Crawl', download: 'Download', iso_check: 'ISO Check', backup: 'Backup' };
  var STATUS_DOT = { running: 'status-dot-success', scheduled: 'status-dot-warning', idle: 'status-dot-neutral', error: 'status-dot-error' };
  var STATUS_BADGE = { running: 'badge-success', scheduled: 'badge-warning', idle: 'badge-default', error: 'badge-error' };

  var MIG_MODULE_BADGE = { oss: 'badge-blue', os_install: 'badge-cyan', iso: 'badge-purple' };
  var MIG_MODULE_LABELS = { oss: 'OSS', os_install: 'OS Install', iso: 'ISO' };
  var MIG_STATUS_BADGE = { pending: 'badge-warning', running: 'badge-blue', completed: 'badge-success', failed: 'badge-error' };
  var MIG_STATUS_DOT = { pending: 'status-dot-warning', running: 'status-dot-success', completed: 'status-dot-success', failed: 'status-dot-error' };
  var MIG_STATUS_LABELS = { pending: 'Pending', running: 'Running', completed: 'Completed', failed: 'Failed' };
  function _migFilters() {
    return [
      { key: 'all', label: t('common_all') || 'All' },
      { key: 'active', label: 'Active' },
      { key: 'completed', label: t('mig_status_completed') || 'Completed' },
      { key: 'failed', label: t('mig_status_failed') || 'Failed' }
    ];
  }

  function TaskCard(props) {
    var k = props.task;
    var tb = TYPE_BADGE[k.type] || 'badge-default';
    var typeLabel = t('tasks_type_' + k.type) || TYPE_LABELS[k.type] || k.type;
    var sd = STATUS_DOT[k.status] || STATUS_DOT.idle;
    var sb = STATUS_BADGE[k.status] || STATUS_BADGE.idle;
    var statusLabel = t('tasks_status_' + k.status) || k.status;

    var progressBar = null;
    if (k.status === 'running' && k.progress > 0) {
      var pct = Math.min(k.progress, 100);
      progressBar = html`
        <div class="dl-progress" role="status">
          <div class="dl-progress-bar">
            <div class="dl-progress-fill" style="width:${pct}%"></div>
          </div>
          <span class="dl-progress-text">${Math.round(pct)}%</span>
        </div>`;
    }

    return html`
      <div class="task-card" key=${k.id} data-id="${k.id}" data-status="${k.status}" data-type="${k.type}"
           style="padding:.75rem 1rem;border-bottom:1px solid var(--color-border-subtle)">
        <div class="flex items-center justify-between gap-2 mb-1">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-sm font-medium truncate" style="color:var(--color-text)">${Helpers.escapeHtml(k.name)}</span>
            <span class="badge ${tb}">${Helpers.escapeHtml(typeLabel)}</span>
          </div>
          <span class="inline-flex items-center">
            <span class="status-dot ${sd}"></span>
            <span class="badge ${sb}">${Helpers.escapeHtml(statusLabel)}</span>
          </span>
        </div>
        <div class="flex items-center gap-4 flex-wrap">
          ${k.schedule ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(k.schedule)}</span>` : null}
          <span class="text-xs" style="color:var(--color-text-tertiary)">${t('tasks_last_run')}: ${k.last_run_at ? Helpers.formatTime(k.last_run_at) : t('never')}</span>
          ${k.next_run_at ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('tasks_next_run')}: ${Helpers.formatTime(k.next_run_at)}</span>` : null}
          ${k.last_result ? html`<span class="text-xs" style="color:${k.last_result === 'success' ? 'var(--color-success)' : 'var(--color-error)'}">${Helpers.escapeHtml(k.last_result)}</span>` : null}
        </div>
        ${progressBar}
      </div>`;
  }

  function TaskSection(props) {
    var type = props.type;
    var tasks = props.tasks;
    var tb = TYPE_BADGE[type] || 'badge-default';
    var typeLabel = t('tasks_type_' + type) || TYPE_LABELS[type] || type;

    return html`
      <div class="task-section" key=${type} data-type="${type}">
        <div class="flex items-center gap-2 px-4 py-2" style="background:var(--color-bg-tertiary);border-bottom:1px solid var(--color-border)">
          <span class="text-xs font-semibold uppercase tracking-wide" style="color:var(--color-text-secondary)">${Helpers.escapeHtml(typeLabel)} (${tasks.length})</span>
        </div>
        ${tasks.map(function (t) {
          return html`<${TaskCard} key=${t.id} task=${t} />`;
        })}
      </div>`;
  }

  function MigrationCard(props) {
    var m = props.migration;
    var onCancel = props.onCancel;
    var mb = MIG_MODULE_BADGE[m.module] || 'badge-default';
    var modLabel = t('mig_module_' + m.module) || MIG_MODULE_LABELS[m.module] || m.module;
    var sb = MIG_STATUS_BADGE[m.status] || 'badge-default';
    var sd = MIG_STATUS_DOT[m.status] || 'status-dot-neutral';
    var stLabel = t('mig_status_' + m.status) || MIG_STATUS_LABELS[m.status] || m.status;
    var isActive = m.status === 'pending' || m.status === 'running';

    var pct = 0;
    if (m.total_files > 0) {
      pct = Math.min(Math.round((m.migrated_files / m.total_files) * 100), 100);
    } else if (m.total_bytes > 0) {
      pct = Math.min(Math.round((m.migrated_bytes / m.total_bytes) * 100), 100);
    }

    var progressBar = null;
    if (m.status === 'running' && pct > 0) {
      progressBar = html`
        <div class="dl-progress" role="status" style="margin-top:0.5rem">
          <div class="dl-progress-bar">
            <div class="dl-progress-fill" style="width:${pct}%"></div>
          </div>
          <span class="dl-progress-text">${pct}%</span>
        </div>`;
    }

    var fileLabel = t('mig_files') || 'files';
    var sizeInfo = (m.migrated_bytes !== undefined && m.total_bytes !== undefined)
      ? Helpers.formatBytes(m.migrated_bytes) + ' / ' + Helpers.formatBytes(m.total_bytes)
      : null;
    var fileInfo = (m.migrated_files !== undefined && m.total_files !== undefined)
      ? m.migrated_files + ' / ' + m.total_files + ' ' + fileLabel
      : null;

    return html`
      <div class="task-card" data-id="${m.id}" data-status="${m.status}"
           style="padding:.75rem 1rem;border-bottom:1px solid var(--color-border-subtle);${isActive ? 'background:var(--color-bg-secondary)' : ''}">
        <div class="flex items-center justify-between gap-2 mb-1">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-sm font-medium truncate" style="color:var(--color-text)">${Helpers.escapeHtml(m.old_path || '')}</span>
            <span class="badge ${mb}">${Helpers.escapeHtml(modLabel)}</span>
          </div>
          <div class="flex items-center gap-2">
            ${isActive ? html`
              <button class="btn btn-ghost btn-sm" onClick=${function() { onCancel(m.id); }}
                      style="font-size:0.75rem;padding:0.125rem 0.5rem;color:var(--color-error)">${t('common_cancel') || 'Cancel'}</button>
            ` : null}
            <span class="inline-flex items-center">
              <span class="status-dot ${sd}"></span>
              <span class="badge ${sb}">${Helpers.escapeHtml(stLabel)}</span>
            </span>
          </div>
        </div>
        <div class="flex items-center gap-4 flex-wrap">
          ${fileInfo ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${fileInfo}</span>` : null}
          ${sizeInfo ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${sizeInfo}</span>` : null}
          ${m.started_at ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('mig_started') || 'Started'}: ${Helpers.formatTime(m.started_at)}</span>` : null}
          ${m.completed_at ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('mig_completed') || 'Done'}: ${Helpers.formatTime(m.completed_at)}</span>` : null}
        </div>
        ${m.error_message ? html`
          <div class="text-xs" style="color:var(--color-error);margin-top:0.375rem">${Helpers.escapeHtml(m.error_message)}</div>
        ` : null}
        ${progressBar}
      </div>`;
  }

  function MigrationSection(props) {
    var migrations = props.migrations;
    var onCancel = props.onCancel;

    var _filter = useState('all');
    var migFilter = _filter[0], setMigFilter = _filter[1];

    var filtered = migrations;
    if (migFilter === 'active') {
      filtered = migrations.filter(function(m) { return m.status === 'pending' || m.status === 'running'; });
    } else if (migFilter === 'completed') {
      filtered = migrations.filter(function(m) { return m.status === 'completed'; });
    } else if (migFilter === 'failed') {
      filtered = migrations.filter(function(m) { return m.status === 'failed'; });
    }

    return html`
      <div class="card" style="margin-top:1rem" data-section="storage-migrations">
        <div class="flex items-center justify-between px-4 py-3" style="border-bottom:1px solid var(--color-border)">
          <span class="text-xs font-semibold uppercase tracking-wide" style="color:var(--color-text-secondary)">${t('mig_title') || 'Storage Migrations'} (${migrations.length})</span>
          <div class="filter-bar" style="gap:0.25rem">
            ${_migFilters().map(function(f) {
              return html`
                <button key=${f.key}
                        class=${'filter-btn' + (f.key === migFilter ? ' active' : '')}
                        style="font-size:0.6875rem;padding:0.125rem 0.5rem"
                        onClick=${function() { setMigFilter(f.key); }}>${f.label}</button>
              `;
            })}
          </div>
        </div>
        ${filtered.length === 0 ? html`
          <div class="empty-state" style="padding:1.5rem">
            <p class="text-sm" style="color:var(--color-text-tertiary)">${t('mig_none') || 'No storage migrations'}</p>
          </div>
        ` : filtered.map(function(m) {
          return html`<${MigrationCard} key=${m.id} migration=${m} onCancel=${onCancel} />`;
        })}
      </div>`;
  }
  function TasksComponent() {
    var _tasksByType = useState({});
    var tasksByType = _tasksByType[0], setTasksByType = _tasksByType[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _hasData = useState(false);
    var hasData = _hasData[0], setHasData = _hasData[1];
    var _migrations = useState([]);
    var migrations = _migrations[0], setMigrations = _migrations[1];

    var mountedRef = useRef(true);

    async function fetchTasks() {
      try {
        var r = await Api.get('/admin/tasks', { silent: true });
        if (!r || !r.success) { setHasData(true); setLoading(false); return; }
        var d = r.data || [];
        // Group by type preserving order
        var grouped = {};
        TYPE_ORDER.forEach(function (o) { grouped[o] = []; });
        d.forEach(function (t) {
          var tp = t.type || 'other';
          if (!grouped[tp]) grouped[tp] = [];
          grouped[tp].push(t);
        });
        // Remove empty groups from the ordered list
        var result = {};
        TYPE_ORDER.forEach(function (o) { if (grouped[o].length) result[o] = grouped[o]; });
        // Add any types not in TYPE_ORDER
        Object.keys(grouped).forEach(function (k) {
          if (TYPE_ORDER.indexOf(k) === -1 && grouped[k].length) result[k] = grouped[k];
        });
        setTasksByType(result);
        setHasData(true);
        setLoading(false);
      } catch (e) {
        setHasData(true);
        setLoading(false);
      }
    }

    async function fetchMigrations() {
      try {
        var r = await Api.get('/admin/storage/migrations', { silent: true });
        if (r && r.success && mountedRef.current) {
          setMigrations(r.data || []);
        }
      } catch (e) { /* silent */ }
    }

    function cancelMigration(id) {
      Api.post('/admin/storage/migrations/' + id + '/cancel').then(function(r) {
        if (r && r.success) {
          if (window.showToast) window.showToast(t('mig_cancelled') || 'Migration cancelled', 'success');
          fetchMigrations();
        }
      });
    }


    // Initial load + auto-refresh
    useEffect(function () {
      mountedRef.current = true;
      fetchTasks();
      var tid = setInterval(function () {
        if (mountedRef.current) fetchTasks();
      }, 15000);
      return function () {
        mountedRef.current = false;
        clearInterval(tid);
      };
    }, []);
    // Migration polling (5s)
    useEffect(function () {
      fetchMigrations();
      var mid = setInterval(function () {
        if (mountedRef.current) fetchMigrations();
      }, 5000);
      return function () { clearInterval(mid); };
    }, []);

    // ── Loading skeleton ───────────────────────────────────────────────────
    if (loading) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('tasks') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('tasks_title')}</h1>
            <div class="card">
              ${[1,2,3,4].map(function () { return html`<div class="skeleton" style="height:3rem;margin:.5rem"></div>`; })}
            </div>
          </div>
        </div>`;
    }

    // ── Empty state ────────────────────────────────────────────────────────
    var typeKeys = Object.keys(tasksByType);
    if (!typeKeys.length) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('tasks') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('tasks_title')}</h1>
            <div class="card">
              <div class="empty-state">
                <p class="text-sm" style="color:var(--color-text-tertiary)">${t('cta_no_tasks')}</p>
                <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_no_tasks_desc')}</p>
              </div>
            </div>
          </div>
        </div>`;
    }

    // ── Sections ───────────────────────────────────────────────────────────
    // Render in TYPE_ORDER, then remaining keys
    var orderedKeys = TYPE_ORDER.filter(function (o) { return tasksByType[o]; });
    var remainingKeys = typeKeys.filter(function (k) { return TYPE_ORDER.indexOf(k) === -1; });
    var allKeys = orderedKeys.concat(remainingKeys);

    return html`
      <div>
        <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('tasks') }} />
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('tasks_title')}</h1>
          <div class="card">
            ${allKeys.map(function (tp) {
              return html`<${TaskSection} key=${tp} type=${tp} tasks=${tasksByType[tp]} />`;
            })}
          </div>
          <${MigrationSection} migrations=${migrations} onCancel=${cancelMigration} />
        </div>
      </div>`;
  }

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${TasksComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
