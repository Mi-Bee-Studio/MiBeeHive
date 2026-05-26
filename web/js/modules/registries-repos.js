const RegistriesRepos = (function () {
  'use strict';
  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  var _esc = Helpers.escapeHtml;
  var _enc = encodeURIComponent;
  var _fb = Helpers.formatBytes;
  var _ft = Helpers.formatTime;
  var _arrowR = '\u25B6';
  var _arrowD = '\u25BC';

  function _safeName(n) { return n.replace(/[\/\.]/g, '_'); }

  // ── Tag Detail ─────────────────────────────────────────────────────────
  function TagDetail(props) {
    var d = props.data;
    if (!d) return null;
    var tag = d.tag || {};
    var manifest = d.manifest || {};
    var platform = manifest.platform || {};
    var layers = manifest.layers || [];

    return html`
      <div class="card" style="padding:0.75rem;font-size:0.8125rem">
        <div class="flex flex-col gap-1 mb-2">
          <span class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('reg_digest')}</span>
          <span class="text-xs" style="color:var(--color-text);font-family:monospace;word-break:break-all">${_esc(tag.digest || '')}</span>
        </div>
        <div class="flex gap-4 mb-2 flex-wrap">
          <span class="text-xs" style="color:var(--color-text-secondary)">${t('reg_size')}: ${_fb(tag.size || 0)}</span>
          <span class="text-xs" style="color:var(--color-text-secondary)">${t('reg_platform')}: ${_esc((platform.os || tag.platform || '') + '/' + (platform.architecture || ''))}</span>
          <span class="text-xs" style="color:var(--color-text-secondary)">${_ft(tag.created_at)}</span>
        </div>
        ${layers.length ? html`
          <div class="text-xs font-medium mb-1" style="color:var(--color-text-secondary)">${t('reg_layers')} (${layers.length})</div>
          ${layers.map(function (l, i) {
            var digest = l.digest || '';
            var short = digest.length > 24 ? digest.substring(0, 24) + '..' : digest;
            return html`
              <div key=${i} class="flex items-start gap-2" style="padding:0.25rem 0;border-bottom:1px solid var(--color-border)">
                <span class="text-xs" style="color:var(--color-text-tertiary);font-family:monospace;min-width:180px" title=${_esc(digest)}>${_esc(short)}</span>
                <span class="text-xs" style="color:var(--color-text-secondary);min-width:60px">${_fb(l.size || 0)}</span>
                <span class="text-xs truncate" style="color:var(--color-text-tertiary);flex:1">${_esc(l.command || l.media_type || '')}</span>
              </div>`;
          })}
        ` : null}
      </div>`;
  }

  // ── Main Component ─────────────────────────────────────────────────────
  function Component() {
    var regsState = useState({ list: [], loading: true });
    var regs = regsState[0], setRegs = regsState[1];

    var selState = useState(null);
    var selectedId = selState[0], setSelectedId = selState[1];

    var repoState = useState({ items: [], marker: '' });
    var repos = repoState[0], setRepos = repoState[1];

    var expandState = useState(null);
    var expandedRepo = expandState[0], setExpanded = expandState[1];

    var tagState = useState({ items: [], marker: '' });
    var tags = tagState[0], setTags = tagState[1];

    var detailState = useState({ tag: null, data: null });
    var detail = detailState[0], setDetail = detailState[1];

    var manualState = useState('');
    var manualRepo = manualState[0], setManual = manualState[1];

    function _reg() {
      return regs.list.find(function (r) { return r.id === selectedId; }) || null;
    }

    function _path(suffix) {
      var r = _reg();
      return r ? '/admin/registries/' + r.id + suffix : '';
    }

    useEffect(function () {
      Api.get('/admin/registries').then(function (r) {
        setRegs({ list: r && r.success ? (r.data || []) : [], loading: false });
      });
    }, []);

    function selectRegistry(id) {
      setSelectedId(id);
      setRepos({ items: [], marker: '' });
      setExpanded(null);
      setTags({ items: [], marker: '' });
      setDetail({ tag: null, data: null });
      if (id) loadRepos(id, '');
    }

    function loadRepos(regId, marker) {
      var url = '/admin/registries/' + regId + '/catalog?n=20';
      if (marker) url += '&last=' + _enc(marker);
      Api.get(url).then(function (r) {
        if (!r || !r.success) return;
        var items = r.data || [];
        var m = items.length === 20 ? items[items.length - 1] : '';
        if (marker) {
          setRepos({ items: repos.items.concat(items), marker: m });
        } else {
          setRepos({ items: items, marker: m });
        }
      });
    }

    function loadMoreRepos() {
      if (selectedId) loadRepos(selectedId, repos.marker);
    }

    function toggleRepo(name) {
      if (expandedRepo === name) {
        setExpanded(null);
        return;
      }
      setExpanded(name);
      setTags({ items: [], marker: '' });
      setDetail({ tag: null, data: null });
      loadTags(name, '');
    }

    function loadTags(repoName, marker) {
      var url = _path('/tags?repo=' + _enc(repoName) + '&n=20');
      if (marker) url += '&last=' + _enc(marker);
      Api.get(url).then(function (r) {
        if (!r || !r.success) return;
        var items = r.data || [];
        var m = items.length === 20 ? items[items.length - 1] : '';
        if (marker) {
          setTags({ items: tags.items.concat(items), marker: m });
        } else {
          setTags({ items: items, marker: m });
        }
      });
    }

    function loadMoreTags(repoName) {
      loadTags(repoName, tags.marker);
    }

    function toggleTag(repoName, tagName) {
      if (detail.tag === tagName) {
        setDetail({ tag: null, data: null });
        return;
      }
      setDetail({ tag: tagName, data: null });
      Api.get(_path('/tags/' + _enc(tagName) + '?repo=' + _enc(repoName))).then(function (r) {
        if (r && r.success) setDetail({ tag: tagName, data: r.data });
      });
    }

    function deleteTag(repoName, tagName) {
      Components.showConfirmModal(t('reg_delete_tag') + ' "' + tagName + '"?', function () {
        Api.delete(_path('/tags/' + _enc(tagName) + '?repo=' + _enc(repoName))).then(function (r) {
          if (r && r.success) {
            Components.showToast(t('reg_deleted'), 'success');
            var newItems = tags.items.filter(function (t) { return t !== tagName; });
            setTags(Object.assign({}, tags, { items: newItems }));
            if (detail.tag === tagName) setDetail({ tag: null, data: null });
          } else {
            Components.showToast(t('reg_failed'), 'error');
          }
        });
      });
    }

    function addManualRepo() {
      var name = manualRepo.trim();
      if (!name) return;
      if (repos.items.indexOf(name) < 0) {
        setRepos({ items: [name].concat(repos.items), marker: repos.marker });
      }
      setExpanded(name);
      setTags({ items: [], marker: '' });
      setDetail({ tag: null, data: null });
      setManual('');
      loadTags(name, '');
    }

    if (regs.loading) return html`<div class="flex-center" style="padding:2rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;

    return html`
      <div class="flex flex-col gap-3">
        <div class="flex items-center gap-2 flex-wrap">
          ${!regs.list.length ? html`<p class="text-sm" style="color:var(--color-text-tertiary)">${t('reg_no_registries')}</p>` : html`
            <select class="input" style="width:100%;max-width:320px" value=${selectedId || ''}
              onChange=${function (e) { selectRegistry(parseInt(e.target.value, 10) || null); }}>
              <option value="">${t('reg_select_registry')}</option>
              ${regs.list.map(function (r) {
                return html`<option key=${r.id} value=${r.id}>${_esc(r.name)} (${_esc(r.type)})</option>`;
              })}
            </select>
          `}
          <input class="input" placeholder=${t('reg_manual_repo')} style="max-width:280px;font-size:0.8125rem"
            value=${manualRepo} onInput=${function (e) { setManual(e.target.value); }}
            onKeyDown=${function (e) { if (e.key === 'Enter') addManualRepo(); }} />
          <button class="btn btn-primary btn-sm" onClick=${addManualRepo} aria-label=${t('reg_add')}>${t('reg_add')}</button>
        </div>

        ${selectedId ? html`<div>
          ${!repos.items.length ? html`<p class="text-sm" style="color:var(--color-text-tertiary)">${t('reg_no_repos')}</p>` : null}
          ${repos.items.map(function (name) {
            var isExpanded = expandedRepo === name;
            return html`
              <div key=${name} class="queue-item" style="cursor:pointer;flex-direction:column;align-items:stretch"
                onClick=${function () { toggleRepo(name); }}>
                <div class="flex items-center gap-2" style="width:100%">
                  <span class="flex-1 text-sm font-medium truncate" style="color:var(--color-text)">${_esc(name)}</span>
                  <span class="text-xs" style="color:var(--color-text-tertiary)">${isExpanded ? _arrowD : _arrowR}</span>
                </div>
                ${isExpanded ? html`
                  <div style="margin-top:0.5rem;width:100%" onClick=${function (e) { e.stopPropagation(); }}>
                    ${!tags.items.length ? html`<p class="text-xs" style="color:var(--color-text-tertiary)">${t('reg_no_tags')}</p>` : null}
                    ${tags.items.map(function (tagName) {
                      var isDetailed = detail.tag === tagName;
                      return html`
                        <div key=${tagName} class="queue-item" style="flex-direction:column;align-items:stretch;cursor:pointer"
                          onClick=${function () { toggleTag(name, tagName); }}>
                          <div class="flex items-center gap-2" style="width:100%">
                            <span class="flex-1 text-sm" style="color:var(--color-text)">${_esc(tagName)}</span>
                            <button class="btn btn-sm btn-danger-outline" style="padding:0.125rem 0.5rem"
                              onClick=${function (e) { e.stopPropagation(); deleteTag(name, tagName); }}
                              aria-label=${t('reg_delete_tag') + ' ' + tagName}>${t('reg_delete_tag')}</button>
                            <span class="text-xs" style="color:var(--color-text-tertiary)">${isDetailed ? _arrowD : _arrowR}</span>
                          </div>
                          ${isDetailed ? html`
                            <div style="margin-top:0.5rem;width:100%">
                              ${detail.data ? html`<${TagDetail} data=${detail.data} />` : html`<div style="text-align:center;padding:0.5rem"><div class="spinner" style="width:1rem;height:1rem;border-width:2px;margin:0 auto"></div></div>`}
                            </div>` : null}
                        </div>`;
                    })}
                    ${tags.marker ? html`
                      <div style="text-align:center;padding:0.25rem">
                        <button class="btn btn-secondary btn-sm" onClick=${function () { loadMoreTags(name); }}>${t('reg_load_more')}</button>
                      </div>` : null}
                  </div>` : null}
              </div>`;
          })}
          ${repos.marker ? html`
            <div style="text-align:center;padding:0.5rem">
              <button class="btn btn-secondary btn-sm" onClick=${loadMoreRepos}>${t('reg_load_more')}</button>
            </div>` : null}
        </div>` : null}
      </div>`;
  }

  return Component;
})();
