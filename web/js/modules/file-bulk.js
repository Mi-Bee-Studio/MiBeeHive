// Module: modules/file-bulk — Bulk operations for the File Center
//
// A self-contained module that augments the file table with multi-select
// (checkbox column, Shift+click range select, Select All) and a bulk action
// bar (Zip Download, Copy Links, Publish to View, Delete). It is not a routed
// page; it exposes the standard { render, destroy } contract plus the pure
// SelectionController / BulkActions helpers so it can be embedded by
// file-center or driven directly by tests.
//
// NOTE: The backend ZIP endpoint (POST /admin/files/zip) does not exist yet.
// downloadZip() is a streaming stub that posts { file_ids } and saves the
// returned blob; the backend will be wired up later.
const FileBulk = (function () {
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

  // ── Selection state (pure, testable) ─────────────────────────────
  // Tracks selected file ids in a Set plus the last toggled row index so
  // Shift+click can select a contiguous range.
  function SelectionController() {
    this.selected = new Set();
    this.lastIndex = -1;
  }
  SelectionController.prototype.toggle = function (id, index) {
    var key = String(id);
    if (this.selected.has(key)) this.selected.delete(key); else this.selected.add(key);
    this.lastIndex = typeof index === 'number' ? index : this.lastIndex;
  };
  SelectionController.prototype.rangeSelect = function (id, index, files) {
    var from = this.lastIndex >= 0 ? this.lastIndex : 0;
    var lo = Math.min(from, index), hi = Math.max(from, index);
    for (var i = lo; i <= hi; i++) {
      if (files[i]) this.selected.add(String(files[i].id));
    }
    this.lastIndex = index;
  };
  SelectionController.prototype.selectAll = function (files) {
    var self = this;
    (files || []).forEach(function (f) { self.selected.add(String(f.id)); });
  };
  SelectionController.prototype.deselectAll = function () { this.selected.clear(); };
  SelectionController.prototype.clear = function () { this.selected.clear(); this.lastIndex = -1; };
  SelectionController.prototype.count = function () { return this.selected.size; };
  SelectionController.prototype.isSelected = function (id) { return this.selected.has(String(id)); };
  SelectionController.prototype.ids = function () { return Array.from(this.selected); };

  // ── Bulk actions (testable; use window.Api / Helpers / Auth) ─────
  var BulkActions = {
    // Streaming ZIP download. Uses raw fetch (not Api.post) so the response
    // can be consumed as a blob without buffering the whole payload in memory.
    downloadZip: function (ids, opts) {
      opts = opts || {};
      var token = (typeof Auth !== 'undefined' && Auth.getToken) ? Auth.getToken() : null;
      var headers = { 'Content-Type': 'application/json' };
      if (token) headers['Authorization'] = 'Bearer ' + token;
      var fetchOpts = {
        method: 'POST',
        headers: headers,
        body: JSON.stringify({ file_ids: ids })
      };
      if (opts.signal) fetchOpts.signal = opts.signal;
      return fetch('/api/v1/admin/files/zip', fetchOpts).then(function (res) {
        if (!res.ok) throw new Error('HTTP ' + res.status);
        return res.blob();
      }).then(function (blob) {
        var url = URL.createObjectURL(blob);
        var a = document.createElement('a');
        a.href = url;
        a.download = 'files.zip';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      });
    },

    // Copy each selected file's public download URL to the clipboard.
    copyLinks: function (files) {
      var urls = (files || []).map(function (f) {
        return window.location.origin + '/api/v1/files/' + f.public_token + '/download';
      });
      return Helpers.copyToClipboard(urls.join('\n')).then(function () {
        return urls.length;
      });
    },

    // Publish file references into a virtual view.
    publishToView: function (viewId, ids) {
      return Api.post('/admin/views/' + viewId + '/nodes', {
        nodes: (ids || []).map(function (id) { return { type: 'file', file_id: id }; })
      });
    },

    // Delete files sequentially.
    deleteFiles: function (ids) {
      var chain = Promise.resolve();
      (ids || []).forEach(function (id) {
        chain = chain.then(function () {
          return Api.delete('/admin/files/' + id, { silent: true });
        });
      });
      return chain;
    }
  };

  // ── Delete confirmation (testable) ───────────────────────────────
  function confirmDelete(ids, onConfirm) {
    Components.showConfirmModal(t('bulk.delete_confirm_message', { count: ids.length }), function () {
      if (typeof onConfirm === 'function') onConfirm();
    });
  }

  // ── Preact component ─────────────────────────────────────────────
  function FileBulkComponent(props) {
    var signal = props.signal;

    var _files = useState([]);
    var files = _files[0], setFiles = _files[1];
    var _total = useState(0);
    var total = _total[0], setTotal = _total[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];

    var controllerRef = useRef(null);
    if (!controllerRef.current) {
      controllerRef.current = new SelectionController();
      (props.initialSelected || []).forEach(function (id) {
        controllerRef.current.selected.add(String(id));
      });
    }
    var controller = controllerRef.current;

    var _sel = useState(controller.ids());
    var selected = _sel[0], setSelected = _sel[1];

    var mountedRef = useRef(true);

    function refreshSelection() { setSelected(controller.ids()); }

    function loadFiles() {
      Api.getWithHeaders('/admin/files?limit=' + PAGE_SIZE, { signal: signal, silent: true }).then(function (res) {
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
      }).catch(function (e) {
        if (e && e.name === 'AbortError') return;
        if (mountedRef.current) { setError(t('file_center_error')); setLoading(false); }
      });
    }

    useEffect(function () {
      mountedRef.current = true;
      loadFiles();
      return function () { mountedRef.current = false; };
    }, []);

    // ── Selection handlers ─────────────────────────────────────────
    function handleRowClick(e, file, index) {
      if (e.target.closest('input[type="checkbox"]')) return;
      if (e.shiftKey) controller.rangeSelect(file.id, index, files);
      else controller.toggle(file.id, index);
      refreshSelection();
    }

    function handleCheckbox(e, file, index) {
      e.stopPropagation();
      if (e.shiftKey) controller.rangeSelect(file.id, index, files);
      else controller.toggle(file.id, index);
      refreshSelection();
    }

    function handleSelectAll(e) {
      if (e.target.checked) controller.selectAll(files);
      else controller.deselectAll();
      refreshSelection();
    }

    var allSelected = files.length > 0 && files.every(function (f) { return controller.isSelected(f.id); });

    // ── Bulk action handlers ───────────────────────────────────────
    function handleZip() {
      var ids = controller.ids();
      if (!ids.length) { Components.showToast(t('bulk.no_selection'), 'warning'); return; }
      Components.showToast(t('bulk.zip_progress'), 'info');
      BulkActions.downloadZip(ids, { signal: signal }).then(function () {
        Components.showToast(t('bulk.zip_download'), 'success');
      }).catch(function (err) {
        if (err && err.name === 'AbortError') return;
        Components.showToast(t('error') + ': ' + (err.message || err), 'error');
      });
    }

    function handleCopy() {
      var selectedFiles = files.filter(function (f) { return controller.isSelected(f.id); });
      if (!selectedFiles.length) { Components.showToast(t('bulk.no_selection'), 'warning'); return; }
      BulkActions.copyLinks(selectedFiles).then(function (count) {
        Components.showToast(t('bulk.copy_success', { count: count }), 'success');
      });
    }

    function handlePublish() {
      var ids = controller.ids();
      if (!ids.length) { Components.showToast(t('bulk.no_selection'), 'warning'); return; }
      openPublishModal(ids);
    }

    function handleDelete() {
      var ids = controller.ids();
      if (!ids.length) { Components.showToast(t('bulk.no_selection'), 'warning'); return; }
      confirmDelete(ids, function () {
        BulkActions.deleteFiles(ids).then(function () {
          Components.showToast(t('bulk.delete_success', { count: ids.length }), 'success');
          controller.clear();
          refreshSelection();
          loadFiles();
        }).catch(function (err) {
          Components.showToast(t('error') + ': ' + (err.message || err), 'error');
        });
      });
    }

    function openPublishModal(ids) {
      Api.get('/admin/views', { signal: signal, silent: true }).then(function (res) {
        var views = (res && res.success && res.data) ? res.data : [];
        if (!views.length) {
          Components.showToast(t('bulk.no_views'), 'warning');
          return;
        }
        var options = views.map(function (v) {
          return '<option value="' + v.id + '">' + Helpers.escapeHtml(v.name || v.slug || v.id) + '</option>';
        }).join('');
        var body = '<div class="p-1">' +
          '<label class="text-xs" style="color:var(--color-text-secondary)">' + t('bulk.publish_to_view') + '</label>' +
          '<select class="select mt-2" id="bulk-view-select">' + options + '</select>' +
          '<div class="flex justify-end gap-3 mt-4">' +
            '<button class="btn btn-secondary" data-action="cancel">' + t('cancel') + '</button>' +
            '<button class="btn btn-primary" data-action="confirm">' + t('confirm') + '</button>' +
          '</div></div>';
        var modal = Components.createModal({
          title: t('bulk.publish_to_view'),
          bodyHtml: body,
          onMount: function (overlay) {
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', function () { modal.close(); });
            overlay.querySelector('[data-action="confirm"]').addEventListener('click', function () {
              var sel = overlay.querySelector('#bulk-view-select');
              var viewId = sel ? sel.value : '';
              if (!viewId) { Components.showToast(t('bulk.no_selection'), 'warning'); return; }
              BulkActions.publishToView(viewId, ids).then(function (res2) {
                if (res2 && res2.success) Components.showToast(t('bulk.publish_success'), 'success');
                else Components.showToast(t('error'), 'error');
                modal.close();
              });
            });
          }
        });
      });
    }

    // ── Render helpers ─────────────────────────────────────────────
    function renderBulkBar() {
      if (!selected.length) return null;
      return html`
        <div class="bulk-bar anim-slide-down" role="toolbar"
             aria-label="${t('bulk.selected_count', { count: selected.length })}">
          <span class="bulk-count">${t('bulk.selected_count', { count: selected.length })}</span>
          <div class="bulk-actions">
            <button type="button" class="btn btn-secondary btn-sm" onClick=${handleZip}>${t('bulk.zip_download')}</button>
            <button type="button" class="btn btn-secondary btn-sm" onClick=${handleCopy}>${t('bulk.copy_link')}</button>
            <button type="button" class="btn btn-secondary btn-sm" onClick=${handlePublish}>${t('bulk.publish_to_view')}</button>
            <button type="button" class="btn btn-danger btn-sm" onClick=${handleDelete}>${t('bulk.delete')}</button>
          </div>
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
                <th class="checkbox-col">
                  <input type="checkbox" aria-label="${t('bulk.select_all')}"
                         checked=${allSelected}
                         onChange=${handleSelectAll} />
                </th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_filename')}</th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_version')}</th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_os')}</th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_arch')}</th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_size')}</th>
                <th class="text-left" style="padding:0.625rem 0.75rem;font-weight:600">${t('file_center_source')}</th>
              </tr>
            </thead>
            <tbody>
              ${files.map(function (f, i) {
                var isSel = controller.isSelected(f.id);
                return html`
                  <tr key=${f.id} data-id=${f.id}
                      class=${isSel ? 'selected-row' : ''}
                      style="cursor:pointer"
                      onClick=${function (e) { handleRowClick(e, f, i); }}>
                    <td class="checkbox-col">
                      <input type="checkbox" aria-label=${f.filename}
                             checked=${isSel}
                             onChange=${function (e) { handleCheckbox(e, f, i); }} />
                    </td>
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
                  </tr>`;
              })}
            </tbody>
          </table>
        </div>`;
    }

    // ── Main layout ────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h1 class="text-lg font-semibold" style="color:var(--color-text)">${t('file_center_title')}</h1>
          <div class="text-xs" style="color:var(--color-text-tertiary)">
            ${t('file_center_summary', { count: total, size: Helpers.formatBytes(files.reduce(function (a, f) { return a + (f.size_bytes || 0); }, 0)) })}
          </div>
        </div>
        ${renderBulkBar()}
        ${renderTable()}
      </div>`;
  }

  // ── Module contract ──────────────────────────────────────────────
  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${FileBulkComponent} signal=${signal} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  var api = {
    render: renderFn,
    destroy: destroyFn,
    SelectionController: SelectionController,
    BulkActions: BulkActions,
    confirmDelete: confirmDelete,
    FileBulkComponent: FileBulkComponent
  };

  // Expose globally (also makes the module importable in vitest).
  window.FileBulk = api;
  return api;
})();