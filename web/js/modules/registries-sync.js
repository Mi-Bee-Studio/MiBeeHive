const RegistriesSync = (function () {
  'use strict';
  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;

  var _statusBadge = { completed: 'badge-success', failed: 'badge-error', running: 'badge-blue', pending: 'badge-default', cancelled: 'badge-warning' };

  function _formatDur(start, end) {
    var ms = new Date(end) - new Date(start);
    if (isNaN(ms) || ms < 0) return '-';
    if (ms < 6e4) return Math.round(ms / 1e3) + 's';
    if (ms < 36e5) return Math.round(ms / 6e4) + 'm';
    return (ms / 36e5).toFixed(1) + 'h';
  }

  function Badge(props) {
    return html`<span class="badge ${_statusBadge[props.status] || 'badge-default'}" style="font-size:0.6875rem">${Helpers.escapeHtml(t('reg_status_' + props.status) || props.status)}</span>`;
  }

  // ── Sync Form ──────────────────────────────────────────────────────────
  function SyncForm(props) {
    var regs = props.registries;
    var formState = useState({ srcReg: '', tgtReg: '', srcRepo: '', tgtRepo: '', srcTag: 'latest', tgtTag: 'latest', platform: 'linux/arm64' });
    var form = formState[0], setForm = formState[1];
    var busyState = useState(false);
    var busy = busyState[0], setBusy = busyState[1];

    function up(k, v) {
      var next = Object.assign({}, form, { [k]: v });
      if (k === 'srcRepo' && !form.tgtRepo) next.tgtRepo = v;
      if (k === 'srcTag' && !form.tgtTag) next.tgtTag = v;
      setForm(next);
    }

    function handleSubmit() {
      if (!form.srcReg) { Components.showToast(t('reg_source_registry_required'), 'error'); return; }
      if (!form.tgtReg) { Components.showToast(t('reg_target_registry_required'), 'error'); return; }
      if (!form.srcRepo.trim()) { Components.showToast(t('reg_source_repo_required'), 'error'); return; }
      if (!form.srcTag.trim()) { Components.showToast(t('error_sync_source_required'), 'error'); return; }
      setBusy(true);
      Api.post('/admin/sync', {
        source_registry_id: +form.srcReg, target_registry_id: +form.tgtReg,
        source_repo: form.srcRepo.trim(), source_tag: form.srcTag.trim(),
        target_repo: form.tgtRepo.trim() || form.srcRepo.trim(),
        target_tag: form.tgtTag.trim() || form.srcTag.trim(),
        platform: form.platform
      }).then(function (r) {
        setBusy(false);
        if (r && r.success) { Components.showToast(t('sync_created'), 'success'); props.onCreated(); }
        else Components.showToast((r && r.message) || t('error_sync_failed'), 'error');
      });
    }

    var regOpts = html`<option value="">${t('reg_select_option')}</option>`.concat(
      regs.map(function (r) { return html`<option key=${r.id} value=${r.id}>${Helpers.escapeHtml(r.name)} (${Helpers.escapeHtml(r.type)})</option>`; })
    );
    var lbl = 'display:block;margin-bottom:0.25rem;font-size:0.75rem;font-weight:var(--font-weight-medium);color:var(--color-text-tertiary)';
    var inp = { width: '100%', marginTop: '2px' };

    return html`
      <div class="card" style="padding:1.25rem">
        <h3 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('reg_create_sync')}</h3>
        <div class="grid gap-3" style="grid-template-columns:1fr 1fr">
          <div><label style=${lbl}>${t('reg_source')}</label><select class="input" style=${inp} value=${form.srcReg} onChange=${function (e) { up('srcReg', e.target.value); }}>${regOpts}</select></div>
          <div><label style=${lbl}>${t('reg_target')}</label><select class="input" style=${inp} value=${form.tgtReg} onChange=${function (e) { up('tgtReg', e.target.value); }}>${regOpts}</select></div>
          <div><label style=${lbl}>${t('reg_source')} ${t('reg_repo')}</label><input class="input" style=${inp} placeholder="library/nginx" value=${form.srcRepo} onInput=${function (e) { up('srcRepo', e.target.value); }} /></div>
          <div><label style=${lbl}>${t('reg_target')} ${t('reg_repo')}</label><input class="input" style=${inp} placeholder="myorg/nginx" value=${form.tgtRepo} onInput=${function (e) { up('tgtRepo', e.target.value); }} /></div>
          <div><label style=${lbl}>${t('reg_source')} ${t('reg_tag')}</label><input class="input" style=${inp} placeholder="latest" value=${form.srcTag} onInput=${function (e) { up('srcTag', e.target.value); }} /></div>
          <div><label style=${lbl}>${t('reg_target')} ${t('reg_tag')}</label><input class="input" style=${inp} placeholder="latest" value=${form.tgtTag} onInput=${function (e) { up('tgtTag', e.target.value); }} /></div>
          <div><label style=${lbl}>${t('reg_platform')}</label><select class="input" style=${inp} value=${form.platform} onChange=${function (e) { up('platform', e.target.value); }}>
            <option value="linux/arm64">linux/arm64</option><option value="linux/amd64">linux/amd64</option><option value="linux/arm/v7">linux/arm/v7</option>
          </select></div>
          <div style="display:flex;align-items:flex-end"><button class="btn btn-primary" style="width:100%" disabled=${busy} onClick=${handleSubmit}>${busy ? '...' : t('reg_create_sync')}</button></div>
        </div>
      </div>`;
  }

  // ── Active Task Card ───────────────────────────────────────────────────
  function TaskCard(props) {
    var task = props.task;
    var pct = task.total_bytes > 0 ? Math.round(task.progress_bytes / task.total_bytes * 100) : 0;
    var srcRef = Helpers.escapeHtml(task.source_repo) + ':' + Helpers.escapeHtml(task.source_tag);
    var tgtRef = Helpers.escapeHtml(task.target_repo) + ':' + Helpers.escapeHtml(task.target_tag);

    function handleCancel() {
      Components.showConfirmModal(t('reg_cancel_sync') + '?', function () {
        Api.post('/admin/sync/' + task.id + '/cancel').then(function (r) {
          if (r && r.success) { Components.showToast(t('sync_cancelled'), 'success'); props.onRefresh(); }
          else Components.showToast((r && r.message) || t('error_sync_failed'), 'error');
        });
      });
    }

    return html`
      <div class="queue-item" data-id=${task.id} data-status=${task.status}>
        <div class="flex-1 min-w-0 flex flex-col gap-1">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium truncate" style="color:var(--color-text)">${srcRef}</span>
            <span class="text-xs" style="color:var(--color-text-tertiary)">--></span>
            <span class="text-sm font-medium truncate" style="color:var(--color-text)">${tgtRef}</span>
          </div>
          <div class="dl-progress" role="status">
            <div class="dl-progress-bar"><div class="dl-progress-fill" style=${{ width: pct + '%' }}></div></div>
            <span class="dl-progress-text">${pct}%</span>
          </div>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.formatBytes(task.progress_bytes)}${task.total_bytes > 0 ? ' / ' + Helpers.formatBytes(task.total_bytes) : ''}</span>
          ${task.error_message ? html`<span class="text-xs" style="color:var(--color-error)">${Helpers.escapeHtml(task.error_message)}</span>` : null}
        </div>
        <div class="flex items-center gap-1 shrink-0">
          <${Badge} status=${task.status} />
          ${task.status === 'running' || task.status === 'pending' ? html`
            <button class="btn btn-secondary btn-sm" onClick=${handleCancel} aria-label=${t('reg_cancel_sync')}>${t('common_cancel')}</button>` : null}
        </div>
      </div>`;
  }

  // ── Main Sync Component ────────────────────────────────────────────────
  function Component(props) {
    var regsState = useState([]);
    var regs = regsState[0], setRegs = regsState[1];
    var activeState = useState([]);
    var activeTasks = activeState[0], setActive = activeState[1];
    var histState = useState([]);
    var history = histState[0], setHistory = histState[1];
    var mountedRef = useRef(true);

    function loadRegs() {
      Api.get('/admin/registries').then(function (r) {
        if (mountedRef.current && r && r.success) setRegs(r.data || []);
      });
    }

    function loadActive() {
      Api.get('/admin/sync?status=running', { silent: true }).then(function (r) {
        if (!mountedRef.current) return;
        var tasks = r && r.success ? (r.data || []) : [];
        if (!tasks.length) {
          Api.get('/admin/sync?status=pending', { silent: true }).then(function (pr) {
            if (mountedRef.current) setActive(pr && pr.success ? (pr.data || []) : []);
          });
        } else {
          setActive(tasks);
        }
      });
    }

    function loadHistory() {
      Api.get('/admin/sync').then(function (r) {
        if (!mountedRef.current) return;
        var all = r && r.success ? (r.data || []) : [];
        setHistory(all.filter(function (t) { return t.status !== 'running' && t.status !== 'pending'; }));
      });
    }

    useEffect(function () {
      mountedRef.current = true;
      loadRegs();
      loadActive();
      loadHistory();
      var tid = setInterval(function () { if (mountedRef.current) loadActive(); }, 5000);
      return function () { mountedRef.current = false; clearInterval(tid); };
    }, []);

    var emptyActive = html`<div class="empty-state">
      <p class="text-sm" style="color:var(--color-text-tertiary)">${t('reg_no_active_syncs')}</p>
      <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_sync_help')}</p>
    </div>`;

    var emptyHistory = html`<div class="empty-state">
      <p class="text-sm" style="color:var(--color-text-tertiary)">${t('reg_no_sync_history')}</p>
    </div>`;

    return html`
      <div>
        <${SyncForm} registries=${regs} onCreated=${function () { loadActive(); loadHistory(); }} />
        <div class="divider" style="margin:1.25rem 0"></div>
        <h3 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('reg_active_tasks')}</h3>
        ${!activeTasks.length ? emptyActive : activeTasks.map(function (task) {
          return html`<${TaskCard} key=${task.id} task=${task} onRefresh=${loadActive} />`;
        })}
        <div class="divider" style="margin:1.25rem 0"></div>
        <h3 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('reg_sync_history')}</h3>
        ${!history.length ? emptyHistory : html`
          <div class="table-wrap overflow-x-auto"><table><thead><tr>
            <th>${t('reg_source')}</th><th>${t('reg_target')}</th><th>${t('common_status')}</th>
            <th>${t('reg_duration')}</th><th>${t('common_size')}</th><th>${t('reg_time')}</th>
          </tr></thead><tbody>
          ${history.map(function (task) {
            return html`<tr key=${task.id}>
              <td><span class="text-sm" style="color:var(--color-text)">${Helpers.escapeHtml(task.source_repo)}:${Helpers.escapeHtml(task.source_tag)}</span></td>
              <td><span class="text-sm" style="color:var(--color-text)">${Helpers.escapeHtml(task.target_repo)}:${Helpers.escapeHtml(task.target_tag)}</span></td>
              <td><${Badge} status=${task.status} /></td>
              <td><span class="text-xs" style="color:var(--color-text-tertiary)">${_formatDur(task.created_at, task.updated_at)}</span></td>
              <td><span class="text-xs" style="color:var(--color-text-tertiary)">${task.progress_bytes > 0 ? Helpers.formatBytes(task.progress_bytes) : '-'}</span></td>
              <td><span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.formatDate(task.created_at)}</span></td>
            </tr>`;
          })}
          </tbody></table></div>
        `}
      </div>`;
  }

  return Component;
})();
