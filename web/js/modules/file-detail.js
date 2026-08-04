// Module: modules/file-detail — File Detail Drawer
//
// Slide-in panel showing a single file's metadata, checksum, source info,
// and dual action buttons (Download + Copy Link). It is not a routed page;
// file-center opens it by calling FileDetail.open(file). The admin-only
// internal endpoint (/admin/files/{id}/internal) is fetched to reveal the
// physical local_path, which is never exposed to non-admin callers.
const FileDetail = (function () {
  'use strict';

  var _mounted = false, _open = false;
  var _ov, _pn, _hd, _ct;
  var _file = null;

  var IC = {
    close: '<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
    download: '<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>',
    link: '<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m13.35-.622l1.757-1.757a4.5 4.5 0 00-6.364-6.364l-4.5 4.5a4.5 4.5 0 001.242 7.244"/></svg>',
    copy: '<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>'
  };

  function _t(k) { return typeof t === 'function' ? t(k) : k; }
  function _cn(n) { while (n.firstChild) n.removeChild(n.firstChild); }

  function _mount() {
    if (_mounted) return;
    _ov = document.createElement('div'); _ov.className = 'drawer-overlay';
    _ov.style.cssText = 'position:fixed;inset:0;z-index:var(--z-overlay);background:var(--color-overlay);opacity:0;pointer-events:none;transition:opacity var(--transition-base)';
    _ov.addEventListener('click', close);

    _pn = document.createElement('div'); _pn.className = 'drawer-panel'; _pn.setAttribute('role', 'dialog');
    _pn.style.cssText = 'position:fixed;top:0;right:0;bottom:0;width:400px;max-width:92vw;z-index:var(--z-modal);background:var(--color-bg);border-left:1px solid var(--color-border);transform:translateX(100%);transition:transform var(--transition-slow);display:flex;flex-direction:column;box-shadow:var(--shadow-xl)';

    var hd = document.createElement('div');
    hd.style.cssText = 'display:flex;align-items:center;gap:var(--space-sm);padding:var(--space-sm) var(--space-md);border-bottom:1px solid var(--color-border)';
    var back = document.createElement('button');
    back.innerHTML = IC.close; back.setAttribute('aria-label', _t('common_close'));
    back.style.cssText = 'flex-shrink:0;background:none;border:none;cursor:pointer;color:var(--color-text-tertiary);padding:var(--space-2xs);display:flex;border-radius:var(--radius-sm)';
    back.addEventListener('click', close);
    _hd = document.createElement('span');
    _hd.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:var(--font-size-base);font-weight:var(--font-weight-semibold);color:var(--color-text)';
    hd.appendChild(back); hd.appendChild(_hd);

    _ct = document.createElement('div');
    _ct.style.cssText = 'flex:1;overflow-y:auto;padding:var(--space-md)';

    _pn.appendChild(hd); _pn.appendChild(_ct);
    document.body.appendChild(_ov); document.body.appendChild(_pn);
    _mounted = true;

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape' && _open) { close(); e.preventDefault(); }
    });
  }

  function _labelRow(label, value) {
    var row = document.createElement('div');
    row.style.cssText = 'display:flex;align-items:flex-start;justify-content:space-between;gap:var(--space-md);padding:var(--space-xs) 0;border-bottom:1px solid var(--color-border-subtle)';
    var lb = document.createElement('span');
    lb.style.cssText = 'flex-shrink:0;font-size:var(--font-size-sm);color:var(--color-text-tertiary)';
    lb.textContent = label;
    var vl = document.createElement('span');
    vl.style.cssText = 'flex:1;min-width:0;text-align:right;font-size:var(--font-size-sm);color:var(--color-text);word-break:break-all';
    vl.textContent = value;
    row.appendChild(lb); row.appendChild(vl);
    return row;
  }

  function _sectionTitle(text) {
    var el = document.createElement('div');
    el.style.cssText = 'font-size:var(--font-size-xs);font-weight:var(--font-weight-semibold);color:var(--color-text-tertiary);text-transform:uppercase;letter-spacing:0.05em;margin:var(--space-md) 0 var(--space-xs)';
    el.textContent = text;
    return el;
  }

  function _copyRow(label, value, toastKey) {
    var row = document.createElement('div');
    row.style.cssText = 'display:flex;align-items:center;gap:var(--space-sm);padding:var(--space-xs) 0;border-bottom:1px solid var(--color-border-subtle)';
    var lb = document.createElement('span');
    lb.style.cssText = 'flex-shrink:0;font-size:var(--font-size-sm);color:var(--color-text-tertiary)';
    lb.textContent = label;
    var vl = document.createElement('span');
    vl.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:var(--font-size-sm);color:var(--color-text);font-family:monospace';
    vl.textContent = value;
    vl.title = value;
    var cp = document.createElement('button');
    cp.innerHTML = IC.copy; cp.title = _t('common_copyUrl');
    cp.style.cssText = 'flex-shrink:0;background:none;border:none;cursor:pointer;color:var(--color-text-tertiary);padding:2px;display:flex';
    cp.addEventListener('click', function () {
      Helpers.copyToClipboard(value).then(function () {
        if (typeof Components !== 'undefined' && Components.showToast) Components.showToast(_t(toastKey), 'success');
      });
    });
    row.appendChild(lb); row.appendChild(vl); row.appendChild(cp);
    return row;
  }

  function _downloadUrl(file) {
    if (file && file.download_url) return file.download_url;
    if (file && file.public_token) return window.location.origin + '/api/v1/files/' + file.public_token + '/download';
    return '';
  }

  function _render() {
    _cn(_ct);
    if (!_file) return;
    var f = _file;
    _hd.textContent = f.filename || _t('detail_drawer_title');

    // Status badge
    var statusRow = document.createElement('div');
    statusRow.style.cssText = 'display:flex;align-items:center;justify-content:space-between;gap:var(--space-md);padding:var(--space-xs) 0;border-bottom:1px solid var(--color-border-subtle)';
    var stLb = document.createElement('span');
    stLb.style.cssText = 'font-size:var(--font-size-sm);color:var(--color-text-tertiary)';
    stLb.textContent = _t('detail_drawer_status');
    var stVal = document.createElement('span');
    stVal.innerHTML = Helpers.statusBadge(f.status);
    statusRow.appendChild(stLb); statusRow.appendChild(stVal);
    _ct.appendChild(statusRow);

    _ct.appendChild(_labelRow(_t('detail_drawer_version'), f.version || '-'));
    _ct.appendChild(_labelRow(_t('detail_drawer_os'), f.os || '-'));
    _ct.appendChild(_labelRow(_t('detail_drawer_arch'), f.arch || '-'));
    _ct.appendChild(_labelRow(_t('detail_drawer_size'), Helpers.formatBytes(f.size_bytes)));
    _ct.appendChild(_labelRow(_t('detail_drawer_source'), f.source_type || '-'));
    _ct.appendChild(_labelRow(_t('detail_drawer_category'), f.category || '-'));

    // Download URL + checksum (copyable)
    var url = _downloadUrl(f);
    if (url) _ct.appendChild(_copyRow(_t('detail_drawer_download_url'), url, 'detail_drawer_link_copied'));
    if (f.checksum) _ct.appendChild(_copyRow(_t('detail_drawer_checksum'), f.checksum, 'detail_drawer_checksum_copied'));
    if (f.created_at) _ct.appendChild(_labelRow(_t('detail_drawer_created'), Helpers.formatTime(f.created_at)));


    // Dual action buttons
    var actions = document.createElement('div');
    actions.style.cssText = 'display:flex;gap:var(--space-sm);margin-top:var(--space-lg)';
    var dl = document.createElement('a');
    dl.href = url; dl.target = '_blank'; dl.rel = 'noopener';
    dl.className = 'btn btn-primary';
    dl.style.cssText = 'flex:1;display:inline-flex;align-items:center;justify-content:center;gap:var(--space-xs)';
    dl.innerHTML = IC.download + '<span>' + _t('detail_drawer_download') + '</span>';
    var cp = document.createElement('button');
    cp.type = 'button'; cp.className = 'btn btn-secondary';
    cp.style.cssText = 'flex:1;display:inline-flex;align-items:center;justify-content:center;gap:var(--space-xs)';
    cp.innerHTML = IC.link + '<span>' + _t('detail_drawer_copy_link') + '</span>';
    cp.addEventListener('click', function () {
      Helpers.copyToClipboard(url).then(function () {
        if (typeof Components !== 'undefined' && Components.showToast) Components.showToast(_t('detail_drawer_link_copied'), 'success');
      });
    });
    actions.appendChild(dl); actions.appendChild(cp);
    _ct.appendChild(actions);
  }

  function open(file) {
    _mount();
    _file = file || null;
    _open = true;
    _ov.style.opacity = '1'; _ov.style.pointerEvents = 'auto'; _pn.style.transform = 'translateX(0)';
    _render();
  }

  function close() {
    if (!_mounted) return;
    _open = false;
    _ov.style.opacity = '0'; _ov.style.pointerEvents = 'none'; _pn.style.transform = 'translateX(100%)';
  }

  function isOpen() { return _open; }

  return {
    render: function () { /* not a routed page; opened from file-center */ },
    destroy: function () { close(); },
    open: open,
    close: close,
    isOpen: isOpen
  };
})();