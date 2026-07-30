const Drawer = (function () {
'use strict';
var SK = 'drawer-open', TABS = ['files', 'isos', 'configs'], W = 320;
var _m = false, _at = 'files', _o = false;
var _d = { files: null, isos: null, configs: null }, _ep = {};
var _ov, _pn, _tb, _ct;

var IC = {
  close: '<svg aria-hidden="true" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
  cr: '<svg aria-hidden="true" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><polyline points="9 6 15 12 9 18"/></svg>',
  file: '<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>',
  disc: '<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/></svg>',
  cfg: '<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M14.7 6.3l7-2.8v14.5l-7 2.8-7.4-2.8-7 2.8V6.3l7-2.8z"/></svg>',
  copy: '<svg aria-hidden="true" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>',
  search: '<svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>'
};

function _t(k) { return typeof t === 'function' ? t(k) : k; }
function _fb(b) { return b != null && typeof Helpers !== 'undefined' ? Helpers.formatBytes(b) : ''; }
function _cn(n) { while (n.firstChild) n.removeChild(n.firstChild); }

function _row(label, sub, icon, onClick) {
  var r = document.createElement('div');
  r.style.cssText = 'display:flex;align-items:center;gap:var(--space-sm);padding:var(--space-xs) var(--space-md);cursor:default;color:var(--color-text);font-size:var(--font-size-sm);transition:background var(--transition-fast)';
  if (onClick) { r.style.cursor = 'pointer'; r.addEventListener('click', onClick); }
  r.addEventListener('mouseenter', function () { this.style.background = 'var(--color-surface-hover)'; });
  r.addEventListener('mouseleave', function () { this.style.background = 'none'; });
  var ic = document.createElement('span');
  ic.style.cssText = 'flex-shrink:0;display:flex;color:var(--color-text-tertiary)'; ic.innerHTML = icon || '';
  var tx = document.createElement('span');
  tx.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap'; tx.textContent = label;
  r.appendChild(ic); r.appendChild(tx);
  if (sub) {
    var sb = document.createElement('span');
    sb.style.cssText = 'flex-shrink:0;font-size:var(--font-size-xs);color:var(--color-text-tertiary)';
    sb.textContent = sub; r.appendChild(sb);
  }
  return r;
}

function _mount() {
  if (_m) return;
  _ov = document.createElement('div'); _ov.className = 'drawer-overlay';
  _ov.style.cssText = 'position:fixed;inset:0;z-index:var(--z-overlay);background:var(--color-overlay);opacity:0;pointer-events:none;transition:opacity var(--transition-base)';
  _ov.addEventListener('click', function () { close(); });

  _pn = document.createElement('div'); _pn.className = 'drawer-panel'; _pn.setAttribute('role', 'dialog');
  _pn.style.cssText = 'position:fixed;top:0;right:0;bottom:0;width:' + W + 'px;max-width:90vw;z-index:var(--z-modal);background:var(--color-bg);border-left:1px solid var(--color-border);transform:translateX(100%);transition:transform var(--transition-slow);display:flex;flex-direction:column;box-shadow:var(--shadow-xl)';

  var hd = document.createElement('div');
  hd.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:var(--space-sm) var(--space-md);border-bottom:1px solid var(--color-border)';
  var tl = document.createElement('span');
  tl.style.cssText = 'font-size:var(--font-size-base);font-weight:var(--font-weight-semibold);color:var(--color-text)'; tl.textContent = _t('drawer_title');
  var cb = document.createElement('button'); cb.innerHTML = IC.close; cb.setAttribute('aria-label', _t('common_close'));
  cb.style.cssText = 'background:none;border:none;cursor:pointer;color:var(--color-text-tertiary);padding:var(--space-2xs);display:flex;border-radius:var(--radius-sm)';
  cb.addEventListener('click', function () { close(); });
  hd.appendChild(tl); hd.appendChild(cb);


  _tb = document.createElement('div'); _tb.className = 'drawer-tabs';
  for (var i = 0; i < TABS.length; i++) {
    (function (tab) {
      var btn = document.createElement('button');
      btn.className = 'drawer-tab' + (tab === _at ? ' active' : '');
      btn.textContent = _t('drawer_tab_' + tab); btn.dataset.tab = tab;
      btn.addEventListener('click', function () { _st(tab); });
      _tb.appendChild(btn);
    })(TABS[i]);
  }

  _ct = document.createElement('div');
  _ct.style.cssText = 'flex:1;overflow-y:auto;padding:var(--space-xs) 0';

  _pn.appendChild(hd); _pn.appendChild(_tb); _pn.appendChild(_ct);
  document.body.appendChild(_ov); document.body.appendChild(_pn);
  _m = true;

  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && _o) { close(); e.preventDefault(); }
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') { toggle(); e.preventDefault(); }
  });
}

function _st(tab) {
  _at = tab;
  var ts = _tb.querySelectorAll('.drawer-tab');
  for (var i = 0; i < ts.length; i++) ts[i].classList.toggle('active', ts[i].dataset.tab === tab);
  _ld(tab); _rc();
}

function _ld(tab) {
  if (_d[tab] !== null) return;
  _d[tab] = 'loading'; _rc();
  var url = tab === 'files' ? '/admin/projects' : tab === 'isos' ? '/admin/os-install/catalog' : '/admin/os-install/configs';
  Api.get(url).then(function (r) {
    _d[tab] = (r && r.success && r.data) ? r.data : []; _rc();
  }).catch(function () { _d[tab] = []; _rc(); if (typeof Components !== "undefined" && Components.showToast) Components.showToast(t('error'),'error'); });
}

function _rc() { _cn(_ct); if (_at === 'files') _rF(); else if (_at === 'isos') _rI(); else _rC(); }

function _rF() {
  var d = _d.files;
  if (d === 'loading') { _ct.appendChild(_row(_t('common_loading'))); return; }
  if (!d || !d.length) { _ct.appendChild(_row(_t('drawer_no_files'))); return; }
  var any = false;
  for (var i = 0; i < d.length; i++) {
    var p = d[i], nm = p.display_name || p.name || '';
    any = true;
    var ex = !!_ep[p.id], fc = p.file_count != null ? p.file_count : (p.files ? p.files.length : 0);
    var cs = _t('drawer_file_count'); cs = cs.replace('{count}', fc);
    var h = document.createElement('div');
    h.style.cssText = 'display:flex;align-items:center;gap:var(--space-sm);padding:var(--space-xs) var(--space-md);cursor:pointer;color:var(--color-text);font-size:var(--font-size-sm);font-weight:var(--font-weight-medium);transition:background var(--transition-fast)';
    h.addEventListener('mouseenter', function () { this.style.background = 'var(--color-surface-hover)'; });
    h.addEventListener('mouseleave', function () { this.style.background = 'none'; });
    var ch = document.createElement('span');
    ch.style.cssText = 'flex-shrink:0;display:flex;color:var(--color-text-tertiary);transition:transform var(--transition-fast);transform:rotate(' + (ex ? '90deg' : '0') + ')'; ch.innerHTML = IC.cr;
    var lb = document.createElement('span');
    lb.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap'; lb.textContent = nm;
    var ct = document.createElement('span');
    ct.style.cssText = 'flex-shrink:0;font-size:var(--font-size-xs);color:var(--color-text-tertiary)'; ct.textContent = cs;
    h.appendChild(ch); h.appendChild(lb); h.appendChild(ct);
    (function (id) { h.addEventListener('click', function () { _ep[id] = !_ep[id]; _rc(); }); })(p.id);
    _ct.appendChild(h);
    if (ex && p.files && p.files.length) {
      for (var j = 0; j < p.files.length; j++) {
        var f = p.files[j];
        _ct.appendChild(_row(f.filename, _fb(f.size), IC.file));
      }
    }
  }
  if (!any) _ct.appendChild(_row(_t('no_results')));
}

function _rI() {
  var d = _d.isos;
  if (d === 'loading') { _ct.appendChild(_row(_t('common_loading'))); return; }
  if (!d || !d.length) { _ct.appendChild(_row(_t('drawer_isos_empty'))); return; }
  var any = false;
  for (var i = 0; i < d.length; i++) {
    var iso = d[i], nm = iso.name || iso.filename || '';
    any = true;
    _ct.appendChild(_row(nm, _fb(iso.size) || (iso.status || ''), IC.disc));
  }
  if (!any) _ct.appendChild(_row(_t('no_results')));
}

function _rC() {
  var d = _d.configs;
  if (d === 'loading') { _ct.appendChild(_row(_t('common_loading'))); return; }
  if (!d || !d.length) { _ct.appendChild(_row(_t('drawer_configs_empty'))); return; }
  var any = false;
  for (var i = 0; i < d.length; i++) {
    var c = d[i], nm = c.name || '';
    any = true;
    var it = document.createElement('div');
    it.style.cssText = 'padding:var(--space-xs) var(--space-md);border-bottom:1px solid var(--color-border-subtle)';
    var rw = document.createElement('div');
    rw.style.cssText = 'display:flex;align-items:center;gap:var(--space-sm)';
    var ic = document.createElement('span');
    ic.style.cssText = 'flex-shrink:0;display:flex;color:var(--color-text-tertiary)'; ic.innerHTML = IC.cfg;
    var tx = document.createElement('span');
    tx.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--color-text);font-size:var(--font-size-sm)'; tx.textContent = nm;
    rw.appendChild(ic); rw.appendChild(tx); it.appendChild(rw);
    if (c.os_type && c.name) {
      var url = location.origin + '/pxe/' + c.os_type + '/' + encodeURIComponent(c.name);
      var ur = document.createElement('div');
      ur.style.cssText = 'display:flex;align-items:center;gap:var(--space-xs);margin-top:var(--space-2xs);padding-left:1.375rem';
      var ut = document.createElement('span');
      ut.style.cssText = 'flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-family:monospace'; ut.textContent = url;
      var cp = document.createElement('button'); cp.innerHTML = IC.copy; cp.title = _t('common_copyUrl');
      cp.style.cssText = 'flex-shrink:0;background:none;border:none;cursor:pointer;color:var(--color-text-tertiary);padding:2px;display:flex';
      (function (u) {
        cp.addEventListener('click', function (e) {
          e.stopPropagation();
          if (typeof Helpers !== 'undefined') Helpers.copyToClipboard(u);
          else if (navigator.clipboard) navigator.clipboard.writeText(u);
          if (typeof Components !== "undefined" && Components.showToast) Components.showToast(_t('common_copied'), 'success');
        });
      })(url);
      ur.appendChild(ut); ur.appendChild(cp); it.appendChild(ur);
    }
    _ct.appendChild(it);
  }
  if (!any) _ct.appendChild(_row(_t('no_results')));
}

function toggle() { _o ? close() : open(); }

function open() {
  _mount(); _o = true; localStorage.setItem(SK, 'true');
  _ov.style.opacity = '1'; _ov.style.pointerEvents = 'auto'; _pn.style.transform = 'translateX(0)';
  _ld(_at); _rc();
}

function close() {
  if (!_m) return; _o = false; localStorage.setItem(SK, 'false');
  _ov.style.opacity = '0'; _ov.style.pointerEvents = 'none'; _pn.style.transform = 'translateX(100%)';
}

function isOpen() { return _o; }

if (localStorage.getItem(SK) === 'true' && document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', function () { open(); });
}

return { toggle: toggle, open: open, close: close, isOpen: isOpen };
})();
