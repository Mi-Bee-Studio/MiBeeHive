// Module: core/search — Global search with categorized dropdown results
const GlobalSearch = (function () {
  'use strict';
  var w, inp, dd, _open = false, MIN = 2;
  var _popup, _popupInp, _popupDd, _popupBackdrop, _popupOpen = false;
  var CATS = [['projects','/files/projects/',true],['files','/files'],['configs','/deploy/configs'],['isos','/deploy/iso'],['containers','/containers/',true]];
  var CAT_LABELS = {projects:'project',files:'file',configs:'config',isos:'iso',containers:'container'};
  var _search = Helpers.debounce(function (q) {
    Api.get('/admin/search?q=' + encodeURIComponent(q)).then(function (r) {
      if (inp.value.trim().length < MIN) return;
      _show((r && r.success && r.data) ? r.data : {});
    });
  }, 300);
  function _esc(s) { return Helpers.escapeHtml(s || ''); }
  function _tr(s, n) { return (!s || s.length <= n) ? s : s.substring(0, n - 3) + '...'; }
  function init() {
    var slot = document.getElementById('global-search-slot');
    if (!slot || w) return;
    w = document.createElement('div');
    w.className = 'gs';
    w.innerHTML = '<div class="gs-wrap"><span class="gs-icon">' + Helpers.ICONS.search + '</span>' +
      '<input type="text" class="input input-search gs-input" placeholder="' + t('search') + '..." autocomplete="off"></div>' +
      '<div class="gs-dd" style="display:none"></div>';
    inp = w.querySelector('.gs-input');
    dd = w.querySelector('.gs-dd');
    inp.addEventListener('input', function () {
      var q = inp.value.trim();
      if (q.length < MIN) { _close(); return; }
      dd.innerHTML = '<div class="gs-empty">' + t('common_loading') + '</div>'; _openFn(); _search(q);
    });
    inp.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') { _close(); inp.blur(); inp.value = ''; }
    });
    document.addEventListener('click', function (e) { if (w && !w.contains(e.target)) _close(); });
    slot.appendChild(w);
  }
  function _openFn() { dd.style.display = ''; _open = true; }
  function _close() { dd.style.display = 'none'; _open = false; }
  function _show(data) {
    var h = '', total = 0;
    for (var i = 0; i < CATS.length; i++) {
      var key = CATS[i][0], route = CATS[i][1], hasDetail = CATS[i][2], items = data[key];
      if (!items || !items.length) continue;
      total += items.length;
      h += '<div class="gs-group"><div class="gs-group-label">' + _esc(t('search_type_' + (CAT_LABELS[key] || key))) + '</div>';
      for (var j = 0; j < items.length; j++) {
        var it = items[j];
        var href = route;
        if (hasDetail) href += (it.id || '');
        h += '<a href="#' + href + '" class="gs-item" data-gs-nav>' +
          '<span class="gs-item-name">' + _esc(it.name) + '</span>' +
          (it.detail ? '<span class="gs-item-detail">' + _esc(_tr(it.detail, 60)) + '</span>' : '') +
          '</a>';
      }
      h += '</div>';
    }
    if (!total) h = '<div class="gs-empty">' + t('no_results') + '</div>';
    dd.innerHTML = h; _openFn();
    dd.querySelectorAll('[data-gs-nav]').forEach(function (el) {
      el.addEventListener('click', function () { _close(); inp.value = ''; });
    });
  }
  function openPopup() {
    if (_popupOpen) return;
    if (!_popup) {
      _popupBackdrop = document.createElement('div');
      _popupBackdrop.className = 'sidebar-search-popup-backdrop';
      _popupBackdrop.addEventListener('click', function () { closePopup(); });

      _popup = document.createElement('div');
      _popup.className = 'sidebar-search-popup';

      var iconWrap = document.createElement('div');
      iconWrap.style.cssText = 'position:relative;display:flex;align-items:center';
      iconWrap.innerHTML = '<span style="position:absolute;left:0.625rem;color:var(--color-text-tertiary);pointer-events:none;display:flex">' + Helpers.ICONS.search + '</span>';
      _popupInp = document.createElement('input');
      _popupInp.type = 'text';
      _popupInp.className = 'sidebar-search-popup-input';
      _popupInp.placeholder = t('search') + '...';
      _popupInp.autocomplete = 'off';
      iconWrap.appendChild(_popupInp);
      _popup.appendChild(iconWrap);

      _popupDd = document.createElement('div');
      _popupDd.className = 'sidebar-search-popup-results';
      _popupDd.style.display = 'none';
      _popup.appendChild(_popupDd);

      _popupInp.addEventListener('input', function () {
        var q = _popupInp.value.trim();
        if (q.length < MIN) { _popupDd.style.display = 'none'; return; }
        _popupDd.innerHTML = '<div class="gs-empty">' + t('common_loading') + '</div>';
        _popupDd.style.display = '';
        _searchPopup(q);
      });

      _popupInp.addEventListener('keydown', function (e) {
        if (e.key === 'Escape') { closePopup(); }
      });

      document.addEventListener('keydown', function (e) {
        if (e.key === 'Escape' && _popupOpen) { closePopup(); e.preventDefault(); }
      });
    }

    document.body.appendChild(_popupBackdrop);
    document.body.appendChild(_popup);
    _popupOpen = true;
    _popupInp.value = '';
    _popupDd.style.display = 'none';
    setTimeout(function () { _popupInp.focus(); }, 100);
  }

  function closePopup() {
    if (!_popupOpen) return;
    if (_popupBackdrop && _popupBackdrop.parentNode) _popupBackdrop.parentNode.removeChild(_popupBackdrop);
    if (_popup && _popup.parentNode) _popup.parentNode.removeChild(_popup);
    _popupOpen = false;
  }

  var _searchPopup = Helpers.debounce(function (q) {
    Api.get('/admin/search?q=' + encodeURIComponent(q)).then(function (r) {
      if (!_popupOpen || _popupInp.value.trim().length < MIN) return;
      _showPopup((r && r.success && r.data) ? r.data : {});
    });
  }, 300);

  function _showPopup(data) {
    var h = '', total = 0;
    for (var i = 0; i < CATS.length; i++) {
      var key = CATS[i][0], route = CATS[i][1], hasDetail = CATS[i][2], items = data[key];
      if (!items || !items.length) continue;
      total += items.length;
      h += '<div class="gs-group"><div class="gs-group-label">' + _esc(t('search_type_' + (CAT_LABELS[key] || key))) + '</div>';
      for (var j = 0; j < items.length; j++) {
        var it = items[j];
        var href = route;
        if (hasDetail) href += (it.id || '');
        h += '<a href="#' + href + '" class="gs-item" data-gs-nav>' +
          '<span class="gs-item-name">' + _esc(it.name) + '</span>' +
          (it.detail ? '<span class="gs-item-detail">' + _esc(_tr(it.detail, 60)) + '</span>' : '') +
          '</a>';
      }
      h += '</div>';
    }
    if (!total) h = '<div class="gs-empty">' + t('no_results') + '</div>';
    _popupDd.innerHTML = h;
    _popupDd.style.display = '';
    _popupDd.querySelectorAll('[data-gs-nav]').forEach(function (el) {
      el.addEventListener('click', function () { closePopup(); });
    });
  }

  function destroy() {
    if (w && w.parentNode) w.parentNode.removeChild(w);
    w = null; inp = null; dd = null; _open = false;
    closePopup();
  }
  return { init: init, destroy: destroy, openPopup: openPopup, closePopup: closePopup, isPopupOpen: function() { return _popupOpen; } };
})();
