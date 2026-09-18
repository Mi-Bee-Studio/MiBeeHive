// Module: modules/scripts — Scheduled scripts (#70)
//
// Cron-driven shell script tasks: list / create / edit / enable / delete,
// in-UI script body editor, manual "run now", and run history with
// stdout/stderr. Backed by /api/v1/admin/scripts* endpoints.
const Scripts = (function () {
  'use strict';

  var html = PreactBridge.html;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  function formatTime(v) {
    if (!v) return t('scripts.never') || '—';
    try {
      var d = new Date(v);
      if (isNaN(d.getTime())) return String(v);
      return Helpers.formatTime(d);
    } catch (e) {
      return String(v);
    }
  }

  function statusBadge(sc) {
    if (!sc.enabled) {
      return html`<span class="badge" style="background:var(--color-bg-tertiary);color:var(--color-text-tertiary)">${t('scripts.disabled')}</span>`;
    }
    if (sc.last_status === 'failed') {
      return html`<span class="badge" style="background:var(--color-error);color:var(--color-text-inverse)">${t('scripts.status_failed')}</span>`;
    }
    if (sc.last_status === 'success') {
      return html`<span class="badge" style="background:var(--color-success);color:var(--color-text-inverse)">${t('scripts.status_success')}</span>`;
    }
    return html`<span class="badge" style="background:var(--color-bg-tertiary);color:var(--color-text-secondary)">${t('scripts.status_scheduled')}</span>`;
  }

  // ── Script table row ────────────────────────────────────────────────────────
  function ScriptRow(props) {
    var sc = props.script;
    var onRun = props.onRun;
    var onEdit = props.onEdit;
    var onContent = props.onContent;
    var onHistory = props.onHistory;
    var onToggle = props.onToggle;
    var onDelete = props.onDelete;

    return html`
      <tr>
        <td class="font-medium">${sc.name}</td>
        <td><code class="text-xs">${sc.schedule}</code></td>
        <td class="text-xs" style="color:var(--color-text-secondary)">${sc.script_path}</td>
        <td class="text-xs">${sc.timeout_seconds}s</td>
        <td>${statusBadge(sc)}</td>
        <td class="text-xs">${formatTime(sc.last_run_at)}</td>
        <td class="text-xs">${sc.enabled ? formatTime(sc.next_run_at) : '—'}</td>
        <td>
          <div class="flex gap-1 flex-wrap">
            <button type="button" class="btn btn-secondary btn-sm" onClick=${function () { onRun(sc); }} title=${t('scripts.run_now')}>
              ▶
            </button>
            <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { onToggle(sc); }} title=${t('scripts.toggle')}>
              ${sc.enabled ? '⏸' : '⏵'}
            </button>
            <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { onContent(sc); }} title=${t('scripts.content')}>
              ✎
            </button>
            <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { onHistory(sc); }} title=${t('scripts.history')}>
              ☰
            </button>
            <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { onEdit(sc); }} title=${t('scripts.edit')}>
              ⚙
            </button>
            <button type="button" class="btn btn-ghost btn-sm" style="color:var(--color-error)" onClick=${function () { onDelete(sc); }} title=${t('scripts.delete')}>
              ✕
            </button>
          </div>
        </td>
      </tr>
    `;
  }

  // ── Run history entry ───────────────────────────────────────────────────────
  function RunItem(props) {
    var run = props.run;
    var _open = useState(false);
    var open = _open[0], setOpen = _open[1];

    var finished = !!run.finished_at;
    var ok = finished && run.exit_code === 0;
    var color = !finished
      ? 'var(--color-warning)'
      : ok ? 'var(--color-success)' : 'var(--color-error)';

    return html`
      <div class="card" style="padding:0.75rem 1rem;margin-bottom:0.5rem">
        <div class="flex items-center justify-between cursor-pointer" onClick=${function () { setOpen(!open); }}>
          <div class="flex items-center gap-2 text-xs">
            <span style="color:${color}">●</span>
            <span class="font-medium">#${run.id}</span>
            <span style="color:var(--color-text-tertiary)">${run.trigger === 'manual' ? t('scripts.trigger_manual') : t('scripts.trigger_cron')}</span>
            <span style="color:var(--color-text-tertiary)">${formatTime(run.started_at)}</span>
            ${finished
              ? html`<span style="color:${color}">exit ${run.exit_code}</span>`
              : html`<span style="color:${color}">${t('scripts.running')}</span>`}
          </div>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${open ? '▲' : '▼'}</span>
        </div>
        ${open ? html`
          <div class="mt-2">
            ${run.error ? html`
              <pre class="text-xs" style="background:var(--color-bg-secondary);padding:0.5rem;border-radius:var(--radius-sm);color:var(--color-error);white-space:pre-wrap">${Helpers.escapeHtml(run.error)}</pre>` : null}
            <div class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.25rem">stdout</div>
            <pre class="text-xs" style="background:var(--color-bg-secondary);padding:0.5rem;border-radius:var(--radius-sm);white-space:pre-wrap;max-height:200px;overflow:auto">${Helpers.escapeHtml(run.stdout) || '—'}</pre>
            <div class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.25rem">stderr</div>
            <pre class="text-xs" style="background:var(--color-bg-secondary);padding:0.5rem;border-radius:var(--radius-sm);white-space:pre-wrap;max-height:200px;overflow:auto">${Helpers.escapeHtml(run.stderr) || '—'}</pre>
          </div>` : null}
      </div>
    `;
  }

  // ── Main component ──────────────────────────────────────────────────────────
  function ScriptsComponent(props) {
    var signal = props.signal;

    var _scripts = useState(null);
    var scripts = _scripts[0], setScripts = _scripts[1];
    var _dir = useState('');
    var dir = _dir[0], setDir = _dir[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];

    var mountedRef = useRef(true);

    function load() {
      Api.get('/admin/scripts', { signal: signal, silent: true }).then(function (res) {
        if (!mountedRef.current) return;
        if (res && res.success) {
          setScripts((res.data && res.data.scripts) || []);
          setDir((res.data && res.data.scripts_dir) || '');
          setLoading(false);
        } else {
          Components.showToast((res && res.message) || 'load failed', 'error');
          setLoading(false);
        }
      }).catch(function (e) {
        if (e && e.name === 'AbortError') return;
        if (mountedRef.current) setLoading(false);
      });
    }

    useEffect(function () {
      mountedRef.current = true;
      load();
      return function () { mountedRef.current = false; };
    }, []);

    // ── Actions ───────────────────────────────────────────────────────────────
    function handleRun(sc) {
      Api.post('/admin/scripts/' + sc.id + '/run', {}, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('scripts.run_started') || 'Run started', 'success');
          setTimeout(function () { if (mountedRef.current) load(); }, 1500);
        } else {
          Components.showToast((res && res.message) || 'run failed', 'error');
        }
      });
    }

    function handleToggle(sc) {
      Api.put('/admin/scripts/' + sc.id, { enabled: !sc.enabled }, { signal: signal }).then(function (res) {
        if (res && res.success) {
          load();
        } else {
          Components.showToast((res && res.message) || 'update failed', 'error');
        }
      });
    }

    function handleDelete(sc) {
      Components.showConfirmModal(t('scripts.delete_confirm') + ': ' + sc.name + '?').then(function (confirmed) {
        if (!confirmed) return;
        Api.delete('/admin/scripts/' + sc.id, { signal: signal }).then(function (res) {
          if (res && res.success) {
            Components.showToast(t('scripts.deleted') || 'Deleted', 'success');
            load();
          } else {
            Components.showToast((res && res.message) || 'delete failed', 'error');
          }
        });
      });
    }

    function openEditModal(sc) {
      var isEdit = !!sc;
      var initial = sc || { name: '', schedule: '* * * * *', script_path: '', timeout_seconds: 300, enabled: true };
      var body = `
        <form id="script-form" class="space-y-4">
          <div>
            <label class="block text-sm font-medium mb-1">${t('scripts.name')} *</label>
            <input type="text" name="name" class="input w-full" value="${Helpers.escapeHtml(initial.name)}" required maxlength="64" />
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">${t('scripts.schedule')} *</label>
            <input type="text" name="schedule" class="input w-full" value="${Helpers.escapeHtml(initial.schedule)}" required placeholder="*/10 * * * *" />
            <p class="text-xs mt-1" style="color:var(--color-text-tertiary)">${t('scripts.schedule_hint')}</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">${t('scripts.script_path')} *</label>
            <input type="text" name="script_path" class="input w-full" value="${Helpers.escapeHtml(initial.script_path)}" required placeholder="backup.sh" />
            <p class="text-xs mt-1" style="color:var(--color-text-tertiary)">${t('scripts.script_path_hint')}: ${Helpers.escapeHtml(dir)}/</p>
          </div>
          <div>
            <label class="block text-sm font-medium mb-1">${t('scripts.timeout')}</label>
            <input type="number" name="timeout_seconds" class="input w-full" value="${initial.timeout_seconds}" min="1" max="86400" />
          </div>
          <div class="flex items-center gap-2">
            <input type="checkbox" name="enabled" id="script-enabled" ${initial.enabled ? 'checked' : ''} class="checkbox" />
            <label for="script-enabled" class="text-sm">${t('scripts.enabled')}</label>
          </div>
          <div class="flex justify-end gap-2 mt-4">
            <button type="button" class="btn btn-ghost" data-close="1">${t('cancel')}</button>
            <button type="submit" class="btn btn-primary">${t('save')}</button>
          </div>
        </form>
      `;
      var modal = Components.createModal({
        title: isEdit ? t('scripts.edit') : t('scripts.new'),
        bodyHtml: body,
        onMount: function (overlay) {
          var form = overlay.querySelector('#script-form');
          overlay.querySelector('[data-close]').addEventListener('click', modal.close);
          form.addEventListener('submit', function (e) {
            e.preventDefault();
            var data = {
              name: form.name.value.trim(),
              schedule: form.schedule.value.trim(),
              script_path: form.script_path.value.trim(),
              timeout_seconds: parseInt(form.timeout_seconds.value, 10) || 300,
              enabled: form.enabled.checked,
            };
            var req = isEdit
              ? Api.put('/admin/scripts/' + sc.id, data, { signal: signal })
              : Api.post('/admin/scripts', data, { signal: signal });
            req.then(function (res) {
              if (res && res.success) {
                Components.showToast(t('scripts.saved') || 'Saved', 'success');
                modal.close();
                load();
              } else {
                Components.showToast((res && res.message) || 'save failed', 'error');
              }
            });
          });
        },
      });
    }

    function openContentModal(sc) {
      var body = `
        <div class="space-y-2">
          <div class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(sc.script_path)} — ${Helpers.escapeHtml(dir)}/</div>
          <textarea id="script-content" class="input w-full font-mono text-xs" rows="14"
            placeholder="#!/bin/sh&#10;echo hello" spellcheck="false" style="min-height:240px"></textarea>
          <p class="text-xs" style="color:var(--color-text-tertiary)">${t('scripts.content_hint')}</p>
          <div class="flex justify-end gap-2">
            <button type="button" class="btn btn-ghost" data-close="1">${t('cancel')}</button>
            <button type="button" class="btn btn-primary" data-save="1">${t('save')}</button>
          </div>
        </div>
      `;
      var modal = Components.createModal({
        title: t('scripts.content') + ': ' + Helpers.escapeHtml(sc.name),
        bodyHtml: body,
        onMount: function (overlay) {
          var ta = overlay.querySelector('#script-content');
          overlay.querySelector('[data-close]').addEventListener('click', modal.close);
          fetch('/api/v1/admin/scripts/' + sc.id + '/content', {
            headers: { Authorization: 'Bearer ' + Auth.getToken() },
          }).then(function (r) {
            if (r.ok) return r.text();
            return ''; // 404 = not uploaded yet; empty editor starts one
          }).then(function (text) {
            // Prefill only if the editor is untouched — a late response must
            // never clobber content the user already typed or saved.
            if (ta.value === '') ta.value = text;
          });
          overlay.querySelector('[data-save]').addEventListener('click', function () {
            fetch('/api/v1/admin/scripts/' + sc.id + '/content', {
              method: 'PUT',
              headers: { Authorization: 'Bearer ' + Auth.getToken(), 'Content-Type': 'text/plain' },
              body: ta.value,
            }).then(function (r) {
              if (r.ok) {
                Components.showToast(t('scripts.content_saved') || 'Saved', 'success');
                modal.close();
              } else {
                r.json().then(function (j) {
                  Components.showToast((j && j.message) || 'save failed', 'error');
                }).catch(function () {
                  Components.showToast('save failed', 'error');
                });
              }
            });
          });
        },
      });
    }

    function openHistoryModal(sc) {
      var body = '<div id="script-runs" class="text-xs" style="color:var(--color-text-tertiary)">…</div>';
      var modal = Components.createModal({
        title: t('scripts.history') + ': ' + Helpers.escapeHtml(sc.name),
        bodyHtml: body,
        size: '720px',
        onMount: function (overlay) {
          var container = overlay.querySelector('#script-runs');
          function renderRuns(runs) {
            render(html`
              <div>
                ${runs.length === 0
                  ? html`<p class="text-sm" style="color:var(--color-text-tertiary)">${t('scripts.no_runs')}</p>`
                  : runs.map(function (run) { return html`<${RunItem} key=${run.id} run=${run} />`; })}
              </div>
            `, container);
          }
          Api.get('/admin/scripts/' + sc.id + '/runs?limit=20', { signal: signal, silent: true })
            .then(function (res) { renderRuns((res && res.data) || []); })
            .catch(function () { renderRuns([]); });
        },
      });
    }

    // ── Render ────────────────────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h1 class="text-lg font-semibold">${t('scripts.title')}</h1>
          <button type="button" class="btn btn-primary" onClick=${function () { openEditModal(null); }}>
            ${t('scripts.new')}
          </button>
        </div>
        <p class="text-xs mb-4" style="color:var(--color-text-tertiary)">
          ${t('scripts.dir_hint')}: <code>${dir}</code>
        </p>

        ${loading ? html`
          <div class="card p-6"><p class="text-sm" style="color:var(--color-text-tertiary)">${t('common_loading')}</p></div>`
        : (scripts && scripts.length === 0) ? html`
          <div class="card p-6">
            <div class="empty-state">
              <p class="text-sm" style="color:var(--color-text-tertiary)">${t('scripts.empty')}</p>
            </div>
          </div>`
        : html`
          <div class="card" style="overflow-x:auto">
            <table class="table w-full">
              <thead>
                <tr class="text-left">
                  <th>${t('scripts.name')}</th>
                  <th>${t('scripts.schedule')}</th>
                  <th>${t('scripts.script_path')}</th>
                  <th>${t('scripts.timeout')}</th>
                  <th>${t('scripts.status')}</th>
                  <th>${t('scripts.last_run')}</th>
                  <th>${t('scripts.next_run')}</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                ${scripts.map(function (sc) {
                  return html`<${ScriptRow} key=${sc.id} script=${sc}
                                         onRun=${handleRun} onEdit=${openEditModal}
                                         onContent=${openContentModal} onHistory=${openHistoryModal}
                                         onToggle=${handleToggle} onDelete=${handleDelete} />`;
                })}
              </tbody>
            </table>
          </div>`}
      </div>
    `;
  }

  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${ScriptsComponent} signal=${signal} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
