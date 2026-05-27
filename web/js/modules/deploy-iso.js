const DeployISO = (function () {
  'use strict';
  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  // ── Constants ────────────────────────────────────────────────────────
  var STATUS_CLS = { available: 'badge-success', downloaded: 'badge-blue', error: 'badge-error', no_match: 'badge-default' };
  var ACCORDION_ID = 'iso-catalog-accordion';
  var POLL_INTERVAL = 5000;

  // ── Sub-navigation ───────────────────────────────────────────────────
  function _subNav(active) {
    return '<nav class="module-tabs">' +
      '<a href="#/deploy" class="module-tab' + (active === 'configs' ? ' active' : '') + '">' + t('nav_deploy_configs') + '</a>' +
      '<a href="#/deploy/iso" class="module-tab' + (active === 'iso' ? ' active' : '') + '">' + t('nav_deploy_iso') + '</a></nav>';
  }

  function _getLabels() {
    return { available: t('catalog_available'), downloaded: t('catalog_downloaded'), error: t('catalog_error'), no_match: t('catalog_no_match') };
  }

  function parseISOFilename(name) {
    var n = name.toLowerCase();
    var hasAmd = n.indexOf('x86_64') >= 0;
    var hasArm = n.indexOf('arm64') >= 0 || n.indexOf('aarch64') >= 0;
    var arch = hasArm ? 'arm64' : (hasAmd ? 'amd64' : 'amd64');
    var distros = [['ubuntu','ubuntu'],['debian','debian'],['rocky','rocky'],['alma','almalinux'],['fedora','fedora'],['opensuse','opensuse'],['centos','centos']];
    for (var i = 0; i < distros.length; i++) {
      if (n.indexOf(distros[i][0]) >= 0) return { distro: distros[i][1], arch: arch };
    }
    return { distro: 'other', arch: 'unknown' };
  }

  // ── Arch badge helper ────────────────────────────────────────────────
  function ArchBadge(props) {
    var arch = props.arch || 'unknown';
    return html`<span class="badge badge-default" style="font-size:0.625rem;margin-left:0.375rem;vertical-align:middle">${arch}</span>`;
  }

  // ── Catalog Entry Row (flat, with arch badge + ActionMenu) ───────────
  function CatalogEntryRow(props) {
    var e = props.entry;
    var lb = _getLabels();
    var E = Helpers.escapeHtml;
    var progress = props.progress;
    var lc = e.last_checked ? new Date(e.last_checked).toLocaleString() : t('never');
    var barId = 'iso-prog-' + e.id;

    var speedText = progress && progress.speed > 0 ? Helpers.formatBytes(progress.speed) + '/s' : undefined;
    var etaText;
    if (progress && progress.eta > 0) {
      var m = Math.floor(progress.eta / 60), s = progress.eta % 60;
      etaText = m > 0 ? m + 'm ' + s + 's' : s + 's';
    }

    // Build overflow menu items for secondary actions
    var menuItems = [];
    menuItems.push({ label: t('catalog_check'), onClick: function () { props.onCheck(e.id); } });
    menuItems.push({ label: t('catalog_edit'), onClick: function () { props.onEdit(e); } });
    menuItems.push({ label: t('action_delete'), onClick: function () { props.onDelete(e); }, danger: true });

    // Determine primary action button based on download state
    var ds = e.download_status || '';
    var primaryBtn = null;
    var secondaryBtn = null;
    if (ds === 'downloading') {
      primaryBtn = html`<button class="btn btn-ghost btn-sm" style="color:var(--color-error)"
        onClick=${function () { props.onCancel(e.id); }}>${t('catalog_cancel_download')}</button>`;
    } else if (ds === 'error') {
      primaryBtn = html`<button class="btn btn-ghost btn-sm" style="color:var(--color-warning)"
        onClick=${function () { props.onRetry(e.id); }}>${t('catalog_retry')}</button>`;
    } else {
      primaryBtn = html`<button class="btn btn-ghost btn-sm"
        onClick=${function () { props.onDownload(e.id); }}>${t('catalog_download')}</button>`;
    }

    // Status badge: combine catalog status with download status badge
    var statusBadge;
    if (props.queueStatus && props.queueStatus !== 'downloaded') {
      var qCls = { pending: 'badge-warning', available: 'badge-warning', downloading: 'badge-blue', downloaded: 'badge-success', error: 'badge-error' };
      var qLabel = props.queueStatus === 'available' ? t('catalog_available') : (t('iso_queue_' + props.queueStatus) || E(props.queueStatus));
      statusBadge = html`<span class="badge ${qCls[props.queueStatus] || 'badge-default'}" style="font-size:0.6875rem">${qLabel}</span>`;
    } else {
      statusBadge = html`<span class="badge ${STATUS_CLS[e.status] || 'badge-default'}" style="font-size:0.6875rem">${lb[e.status] || E(e.status)}</span>`;
    }

    return html`
      <tr data-cid=${e.id}>
        <td>
          <div class="font-medium" style="color:var(--color-text)">${E(e.name)}<${ArchBadge} arch=${e.arch} /></div>
          ${e.variant ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${E(e.variant)}</span>` : null}
          ${ds === 'error' && e.last_error ? html`
            <span class="text-xs" style="color:var(--color-error);display:block;margin-top:0.125rem" data-role="last-error">${E(e.last_error)}</span>` : null}
          ${ds === 'downloading' ? html`
            <div style="margin-top:0.25rem">
              <div class="dl-progress" id=${barId} style="background:var(--color-bg-tertiary);border-radius:var(--radius-sm);overflow:hidden;height:0.375rem">
                <div class="dl-progress-bar" style="width:${progress ? progress.percent : 0}%;height:100%;background:var(--color-primary);transition:width 0.3s"></div>
              </div>
              ${speedText ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${speedText}${etaText ? ' - ' + etaText : ''}</span>` : null}
            </div>` : null}
        </td>
        <td>${statusBadge}</td>
        <td><span class="text-xs" style="color:var(--color-text-tertiary)">${lc}</span></td>
        <td>
          <label class="toggle-switch di-auto">
            <input type="checkbox" role="switch" checked=${e.auto_update}
              aria-checked="${e.auto_update ? 'true' : 'false'}"
              onChange=${function (ev) { props.onToggleAuto(e.id, ev.target.checked, ev); }} />
            <span class="toggle-slider"></span>
          </label>
        </td>
        <td style="text-align:right">
          <div class="flex justify-end gap-1">
            ${primaryBtn}
            ${secondaryBtn}
            <${Components.ActionMenu} items=${menuItems} />
          </div>
        </td>
      </tr>`;
  }

  // ── Standalone File Row (with arch badge + ActionMenu) ───────────────
  function StandaloneFileRow(props) {
    var iso = props.iso, E = Helpers.escapeHtml;
    var sz = iso.size_bytes > 0 ? Helpers.formatBytes(iso.size_bytes) : '';
    var mt = iso.mod_time ? new Date(iso.mod_time).toLocaleString() : '';
    var parsed = parseISOFilename(iso.name);
    return html`
      <tr data-standalone=${iso.name}>
        <td><div class="font-medium" style="color:var(--color-text)">${E(iso.name)}<${ArchBadge} arch=${parsed.arch} /></div><span class="badge badge-default" style="font-size:0.625rem;margin-top:0.125rem">${t('catalog_file_badge')}</span></td>
        <td><span class="badge badge-blue" style="font-size:0.6875rem">${t('catalog_downloaded')}</span><span class="text-xs" style="color:var(--color-text-tertiary);margin-left:0.375rem">${sz}</span></td>
        <td><span class="text-xs" style="color:var(--color-text-tertiary)">${mt}</span></td>
        <td></td>
        <td style="text-align:right">
          <div class="flex justify-end">
            <${Components.ActionMenu} items=${[{ label: t('action_delete'), onClick: function() { props.onDelete(iso.name); }, danger: true }]} />
          </div>
        </td>
      </tr>`;
  }

  // ── Download Summary Bar ─────────────────────────────────────────────
  function DownloadSummaryBar(props) {
    var stats = props.stats || { pending: 0, downloading: 0, downloaded: 0, error: 0, total: 0 };
    // Only show when there's meaningful activity
    var hasActivity = stats.downloading > 0 || stats.pending > 0 || (stats.available && stats.available > 0) || stats.error > 0;
    if (!hasActivity) return null;

    return html`
      <div class="download-summary-bar" style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:0.5rem;padding:0.625rem 1rem;margin-bottom:0.75rem;border:1px solid var(--color-border);border-radius:var(--radius-md);background:var(--color-bg-secondary)">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1" style="font-size:0.75rem;color:var(--color-text-tertiary)">
          <span style="font-weight:var(--font-weight-semibold);color:var(--color-text)">${t('iso_download_summary')}</span>
          ${stats.downloading > 0 ? html`<span>${t('iso_queue_downloading')}: <span style="color:var(--color-primary)">${stats.downloading}</span></span>` : null}
          ${(stats.available || 0) > 0 ? html`<span>${t('catalog_available')}: <span style="color:var(--color-warning)">${stats.available}</span></span>` : null}
          ${stats.pending > 0 ? html`<span>${t('iso_queue_pending')}: <span style="color:var(--color-warning)">${stats.pending}</span></span>` : null}
          ${stats.error > 0 ? html`<span>${t('iso_queue_error')}: <span style="color:var(--color-error)">${stats.error}</span></span>` : null}
          ${stats.downloaded > 0 ? html`<span>${t('iso_queue_downloaded')}: <span style="color:var(--color-success)">${stats.downloaded}</span></span>` : null}
          <span>${t('iso_queue_total')}: <span style="color:var(--color-text-secondary)">${stats.total}</span></span>
        </div>
        ${props.onDownloadAll && (stats.pending > 0 || (stats.available || 0) > 0 || stats.error > 0) ? html`
          <button class="btn btn-ghost btn-sm" onClick=${props.onDownloadAll}>${t('iso_queue_download_all')}</button>
        ` : null}
      </div>`;
  }

  // ── Catalog Section (distro accordion, flat rows) ────────────────────
  function CatalogSection(props) {
    var _open = useState(false);
    var open = _open[0], setOpen = _open[1];
    var distro = props.distro;
    var entries = props.entries;
    var totalCount = entries.length;
    var progressMap = props.progressMap;

    // Build queue status lookup from queueItems
    var queueLookup = props.queueLookup || {};

    return html`
      <div style="border:1px solid var(--color-border);border-radius:var(--radius-md);margin-bottom:0.5rem;overflow:hidden">
        <button style="width:100%;display:flex;align-items:center;justify-content:space-between;padding:0.75rem 1rem;background:var(--color-bg-secondary);border:none;cursor:pointer;color:var(--color-text);font-size:0.875rem;font-weight:var(--font-weight-semibold)"
          onClick=${function () { setOpen(!open); }}>
          <div class="flex items-center gap-2">
            <span>${distro.charAt(0).toUpperCase() + distro.slice(1)}</span>
            <span class="text-xs" style="color:var(--color-text-tertiary)">${totalCount}</span>
          </div>
          <svg style="transform:rotate(${open ? '180' : '0'}deg);transition:transform 0.2s;width:1rem;height:1rem;color:var(--color-text-quaternary)" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M6 9l6 6 6-6"/></svg>
        </button>
        ${open ? html`
          <div class="table-wrap overflow-x-auto" style="padding:0">
            <table style="margin-bottom:0">
              <thead>
                <tr>
                  <th>${t('catalog_name')}</th>
                  <th>${t('catalog_status')}</th>
                  <th>${t('catalog_last_checked')}</th>
                  <th>${t('catalog_auto_update')}</th>
                  <th style="text-align:right">${t('actions')}</th>
                </tr>
              </thead>
              <tbody>
                ${entries.map(function (e) {
                  if (e._isStandalone) {
                    return html`<${StandaloneFileRow} key=${e.id} iso=${e} onDelete=${props.onDeleteStandalone} />`;
                  }
                  var fn = e.current_url ? e.current_url.split('/').pop() : '';
                  var queueEntry = queueLookup[e.id];
                  var queueStatus = queueEntry ? (queueEntry.download_status || 'pending') : null;
                  return html`<${CatalogEntryRow} key=${e.id} entry=${e} progress=${progressMap[fn]} queueStatus=${queueStatus}
                    onCheck=${props.onCheck} onDownload=${props.onDownload} onRetry=${props.onRetry}
                    onCancel=${props.onCancel} onEdit=${props.onEdit} onDelete=${props.onDelete}
                    onToggleAuto=${props.onToggleAuto} />`;
                })}
              </tbody>
            </table>
          </div>` : null}
      </div>`;
  }

  function DownloadISOModal(props) {
    var _fn=useState(''),fn=_fn[0],setFn=_fn[1];
    var _u=useState(''),u=_u[0],setU=_u[1];
    var _s=useState(false),s=_s[0],setS=_s[1];
    function submit(){
      var fname=fn.trim(),url=u.trim();
      if(!fname){showToast(t('validation_required'),'error');return;}
      if(!/\.iso$/i.test(fname)){showToast(t('validation_iso_filename'),'error');return;}
      if(!url){showToast(t('validation_required'),'error');return;}
      if(!Helpers.validateURL(url)){showToast(t('validation_iso_url'),'error');return;}
      setS(true);
      props.onSubmit(fname,url,function(){setS(false);setFn('');setU('');props.onClose();});
    }
    var fz='font-size:0.8125rem';
    return html`
      <div ref=${props.overlayRef} class="modal-overlay" onClick=${function(e){if(e.target===e.currentTarget)props.onClose();}} onKeyDown=${function(e){if(e.key==='Escape')props.onClose();}}>
        <div class="modal-content" role="dialog" aria-modal="true" style="max-width:28rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('osinstall_iso_trigger')}</h3>
          <div class="grid gap-3">
            <div><label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('osinstall_iso_url')}</label><input class="input" placeholder="https://example.com/path/to/file.iso" style=${fz} value=${u} onInput=${function(e){setU(e.target.value);}} /></div>
            <div><label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('osinstall_iso_filename')}</label><input class="input" placeholder="ubuntu-24.04-live-server-amd64.iso" style=${fz} value=${fn} onInput=${function(e){setFn(e.target.value);}} /></div>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('cancel')}</button>
            <button class="btn btn-primary" onClick=${submit} disabled=${s}>${s?'...':t('osinstall_iso_trigger')}</button>
          </div>
        </div>
      </div>`;
  }

  // ── Catalog Entry Modal ──────────────────────────────────────────────
  function CatalogEntryModal(props) {
    var entry = props.entry;
    var isEdit = !!entry;
    var E = Helpers.escapeHtml;

    var _name = useState(isEdit ? entry.name : '');
    var name = _name[0], setName = _name[1];
    var _distro = useState(isEdit ? entry.distro : '');
    var distro = _distro[0], setDistro = _distro[1];
    var _variant = useState(isEdit ? (entry.variant || '') : '');
    var variant = _variant[0], setVariant = _variant[1];
    var _arch = useState(isEdit ? entry.arch : 'amd64');
    var arch = _arch[0], setArch = _arch[1];
    var _checkUrl = useState(isEdit ? entry.check_url : '');
    var checkUrl = _checkUrl[0], setCheckUrl = _checkUrl[1];
    var _pattern = useState(isEdit ? entry.filename_pattern : '');
    var pattern = _pattern[0], setPattern = _pattern[1];
    var _autoUpdate = useState(isEdit && entry.auto_update ? true : false);
    var autoUpdate = _autoUpdate[0], setAutoUpdate = _autoUpdate[1];
    var _interval = useState(isEdit ? entry.check_interval_hours : 24);
    var interval = _interval[0], setInterval2 = _interval[1];
    var _submitting = useState(false);
    var submitting = _submitting[0], setSubmitting = _submitting[1];
    var _profiles = useState([]);
    var profiles = _profiles[0], setProfiles = _profiles[1];
    var _baseUrl = useState(isEdit ? (entry.base_url || '') : '');
    var baseUrl = _baseUrl[0], setBaseUrl = _baseUrl[1];
    var _versionDirPattern = useState(isEdit ? (entry.version_dir_pattern || '') : '');
    var versionDirPattern = _versionDirPattern[0], setVersionDirPattern = _versionDirPattern[1];
    var _isoPathTemplate = useState(isEdit ? (entry.iso_path_template || '') : '');
    var isoPathTemplate = _isoPathTemplate[0], setIsoPathTemplate = _isoPathTemplate[1];
    useEffect(function() {
      Api.get('/admin/os-install/catalog/profiles').then(function(r) {
        if (r && r.success) setProfiles(r.data || []);
      });
    }, []);

    var lbl = 'color:var(--color-text-secondary)';
    var fz = 'font-size:0.8125rem';

    function handleSubmit() {
      var nm = name.trim(), di = distro.trim(), cu = checkUrl.trim(), pt = pattern.trim();
      if (!nm) { showToast(t('catalog_name') + ': ' + t('validation_required'), 'error'); return; }
      if (!di) { showToast(t('catalog_distro') + ': ' + t('validation_required'), 'error'); return; }
      if (!cu && !baseUrl.trim()) { showToast(t('catalog_check_url') + ': ' + t('validation_required'), 'error'); return; }
      if (!pt) { showToast(t('catalog_filename_pattern') + ': ' + t('validation_required'), 'error'); return; }
      setSubmitting(true);
      var body = { name: nm, distro: di, variant: variant.trim(), arch: arch, check_url: cu, filename_pattern: pt, base_url: baseUrl.trim(), version_dir_pattern: versionDirPattern.trim(), iso_path_template: isoPathTemplate.trim(), auto_update: autoUpdate, check_interval_hours: parseInt(interval) || 24 };
      var req = isEdit ? Api.put('/admin/os-install/catalog/' + entry.id, body) : Api.post('/admin/os-install/catalog', body);
      req.then(function (r) {
        setSubmitting(false);
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        showToast(isEdit ? t('catalog_updated') : t('catalog_created'), 'success');
        props.onClose();
        props.onSaved();
      });
    }

    return html`
      <div ref=${props.overlayRef} class="modal-overlay" onClick=${props.handleOverlayClick} onKeyDown=${props.handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" style="max-width:32rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${isEdit ? t('catalog_edit') : t('catalog_add_entry')}</h3>
          <div class="grid gap-3">
            <div>
              <label class="text-xs font-medium" style="${lbl}">${t('catalog_distro_template')}</label>
              <select class="input select" style="${fz}" onChange=${function(ev) {
                var p = profiles.find(function(x) { return x.id == ev.target.value; });
                if (p) {
                  setDistro(p.distro || ''); setVariant(p.variant || ''); setArch(p.arch || 'amd64');
                  setBaseUrl(p.base_url || ''); setVersionDirPattern(p.version_dir_pattern || '');
                  setIsoPathTemplate(p.iso_path_template || ''); setPattern(p.filename_pattern || '');
                }
                ev.target.selectedIndex = 0;
              }}>
                <option value="">${t('catalog_select_template')}</option>
                ${profiles.map(function(p) { return html`<option value=${p.id}>${p.name}</option>`; })}
              </select>
            </div>
            <div>
              <label class="text-xs font-medium" style="${lbl}">${t('catalog_name')}</label>
              <input class="input" value=${name} placeholder="Ubuntu Server 24.04 LTS (amd64)" style="${fz}"
                onInput=${function (e) { setName(e.target.value); }} />
            </div>
            <div class="grid grid-cols-3 gap-3">
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_distro')}</label>
                <input class="input" value=${distro} placeholder="ubuntu" style="${fz}"
                  onInput=${function (e) { setDistro(e.target.value); }} />
              </div>
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_variant')}</label>
                <input class="input" value=${variant} placeholder="server" style="${fz}"
                  onInput=${function (e) { setVariant(e.target.value); }} />
              </div>
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_arch')}</label>
                <select class="input select" style="${fz}" value=${arch}
                  onChange=${function (e) { setArch(e.target.value); }}>
                  <option value="amd64">amd64</option>
                  <option value="arm64">arm64</option>
                </select>
              </div>
            </div>
            <div>
              <label class="text-xs font-medium" style="${lbl}">${t('catalog_check_url')}${baseUrl.trim() ? html`<span class="text-xs" style="color:var(--color-text-tertiary);margin-left:0.25rem">(${t('catalog_check_url_optional')})</span>` : ''}</label>
              <input class="input" value=${checkUrl} placeholder="https://releases.ubuntu.com/24.04/" style="${fz}"
                onInput=${function (e) { setCheckUrl(e.target.value); }} />
            </div>
            <div>
              <label class="text-xs font-medium" style="${lbl}">${t('catalog_filename_pattern')}</label>
              <input class="input" value=${pattern} placeholder="ubuntu-24\\.04\\.\\d+-live-server-amd64\\.iso" style="${fz}"
                onInput=${function (e) { setPattern(e.target.value); }} />
            </div>
            <div>
              <label class="text-xs font-medium" style="${lbl}">${t('catalog_base_url')}</label>
              <input class="input" value=${baseUrl} placeholder="https://releases.ubuntu.com/" style="${fz}"
                onInput=${function (e) { setBaseUrl(e.target.value); }} />
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_version_dir_pattern')}</label>
                <input class="input" value=${versionDirPattern} placeholder="\\d+\\.\\d+(\\.\\d+)?" style="${fz}"
                  onInput=${function (e) { setVersionDirPattern(e.target.value); }} />
              </div>
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_iso_path_template')}</label>
                <input class="input" value=${isoPathTemplate} placeholder="{version}/{arch}/" style="${fz}"
                  onInput=${function (e) { setIsoPathTemplate(e.target.value); }} />
              </div>
            </div>
            <div class="grid grid-cols-2 gap-3">
              <div class="flex items-center gap-3">
                <label class="toggle-switch">
                  <input type="checkbox" role="switch" checked=${autoUpdate}
                    aria-checked="${autoUpdate ? 'true' : 'false'}"
                    onChange=${function () { setAutoUpdate(!autoUpdate); }} />
                  <span class="toggle-slider"></span>
                </label>
                <span class="text-sm" style="${lbl}">${t('catalog_auto_update')}</span>
              </div>
              <div>
                <label class="text-xs font-medium" style="${lbl}">${t('catalog_check_interval')}</label>
                <input type="number" min="1" class="input" value=${interval} style="${fz}"
                  onInput=${function (e) { setInterval2(e.target.value); }} />
              </div>
            </div>
          </div>
          <div class="flex justify-end gap-3 mt-6">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('cancel')}</button>
            <button class="btn btn-primary" onClick=${handleSubmit} disabled=${submitting}>
              ${submitting ? '...' : t('save')}
            </button>
          </div>
        </div>
      </div>`;
  }

  // ── Main ISO Page Component ──────────────────────────────────────────
  function ISOPage() {
    var _catalog = useState([]);
    var catalog = _catalog[0], setCatalog = _catalog[1];
    var _isos = useState([]);
    var isos = _isos[0], setIsos = _isos[1];
    var _progress = useState([]);
    var progress = _progress[0], setProgress = _progress[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _catError = useState(null);
    var catError = _catError[0], setCatError = _catError[1];
    var _catFilter = useState('');
    var catFilter = _catFilter[0], setCatFilter = _catFilter[1];
    var _showCatModal = useState(false);
    var showCatModal = _showCatModal[0], setShowCatModal = _showCatModal[1];
    var _editEntry = useState(null);
    var editEntry = _editEntry[0], setEditEntry = _editEntry[1];

    var _showDlModal = useState(false);
    var showDlModal = _showDlModal[0], setShowDlModal = _showDlModal[1];

    // Button states
    var _checkAllBtn = useState(false);
    var checkAllBtn = _checkAllBtn[0], setCheckAllBtn = _checkAllBtn[1];

    // Queue state (still fetched for summary stats + inline status)
    var _queueItems = useState([]);
    var queueItems = _queueItems[0], setQueueItems = _queueItems[1];
    var _queueStats = useState({ pending: 0, downloading: 0, downloaded: 0, error: 0, total: 0 });
    var queueStats = _queueStats[0], setQueueStats = _queueStats[1];

    var mountedRef = useRef(true);
    var overlayRef = useRef(null);
    var previousFocusRef = useRef(null);
    var filterBarContainerRef = useRef(null);

    // ── Data loading ────────────────────────────────────────────────────
    function fetchData() {
      Promise.all([Api.get('/admin/os-install/catalog'), Api.get('/admin/os-install/isos')]).then(function (r) {
        if (!mountedRef.current) return;
        var catOk = r[0] && r[0].success;
        var isoOk = r[1] && r[1].success;
        if (catOk) {
          setCatalog(r[0].data || []);
          setCatError(null);
        } else {
          setCatError(t('error_load_failed'));
        }
        if (isoOk) setIsos(r[1].data || []);
        setLoading(false);

        // Fetch initial progress
        Api.get('/admin/os-install/catalog/progress').then(function (pr) {
          if (!mountedRef.current) return;
          setProgress((pr && pr.success) ? (pr.data || []) : []);
        });
      });

      // Fetch queue data (for summary bar + inline status)
      Api.get('/admin/os-install/catalog/queue').then(function (r) {
        if (!mountedRef.current) return;
        if (r && r.success) {
          setQueueItems(r.data && r.data.items || []);
          setQueueStats(r.data && r.data.stats || { pending: 0, downloading: 0, downloaded: 0, error: 0, total: 0 });
        }
      });
    }

    // ── Polling ─────────────────────────────────────────────────────────
    useEffect(function () {
      mountedRef.current = true;
      fetchData();

      var pollId = setInterval(function () {
        if (!mountedRef.current) return;
        Api.get('/admin/os-install/catalog', { silent: true }).then(function (r) {
          if (!mountedRef.current || !r || !r.success) return;
          setCatalog(r.data || []);
          Api.get('/admin/os-install/catalog/progress', { silent: true }).then(function (pr) {
            if (!mountedRef.current) return;
            setProgress((pr && pr.success) ? (pr.data || []) : []);
          });
        });
        Api.get('/admin/os-install/isos', { silent: true }).then(function (r) {
          if (!mountedRef.current || !r || !r.success) return;
          setIsos(r.data || []);
        });
        Api.get('/admin/os-install/catalog/queue', { silent: true }).then(function (r) {
          if (!mountedRef.current || !r || !r.success) return;
          setQueueItems(r.data && r.data.items || []);
          setQueueStats(r.data && r.data.stats || { pending: 0, downloading: 0, downloaded: 0, error: 0, total: 0 });
        });
      }, POLL_INTERVAL);
      App.addTimer(pollId);

      return function () {
        mountedRef.current = false;
        App.clearAllTimers();
      };
    }, []);

    // ── FilterBar initialization ────────────────────────────────────────
    useEffect(function () {
      if (!filterBarContainerRef.current) return;
      Components.FilterBar.init(filterBarContainerRef.current, {
        id: 'iso-cat-filter',
        filters: [
          { key: '', label: t('common_all') },
          { key: 'downloaded', label: t('filter_downloaded') },
          { key: 'pending', label: t('filter_pending') }
        ],
        onChange: function (activeKey) {
          setCatFilter(activeKey);
        }
      });
      return function () { Components.FilterBar.destroy('iso-cat-filter'); };
    }, []);

    // ── Progress lookup by filename ─────────────────────────────────────
    var progressMap = useMemo(function () {
      var m = {};
      progress.forEach(function (p) { if (p.filename) m[p.filename] = p; });
      return m;
    }, [progress]);

    // ── Queue lookup by catalog entry ID ────────────────────────────────
    var queueLookup = useMemo(function () {
      var m = {};
      queueItems.forEach(function (q) { m[q.id] = q; });
      return m;
    }, [queueItems]);

    // ── Group catalog + standalone ISOs by distro (flat, no arch nesting) ─
    var grouped = useMemo(function () {
      var filtered = catFilter ? catalog.filter(function (e) {
        if (catFilter === 'downloaded') return e.status === 'downloaded';
        return e.status !== 'downloaded';
      }) : catalog;

      var groups = {};
      filtered.forEach(function (e) {
        var key = e.distro || 'other';
        if (!groups[key]) groups[key] = [];
        groups[key].push(e);
      });

      // Build set of filenames already covered by catalog entries
      var catalogFiles = {};
      catalog.forEach(function (e) {
        if (e.current_url) {
          catalogFiles[e.current_url.split('/').pop()] = true;
        }
      });

      // Merge standalone ISO files not in catalog
      var standaloneFiltered = catFilter === 'pending' ? [] : isos;
      standaloneFiltered.forEach(function (iso) {
        if (catalogFiles[iso.name]) return;
        var parsed = parseISOFilename(iso.name);
        var key = parsed.distro;
        if (!groups[key]) groups[key] = [];
        groups[key].push({
          id: 'file-' + iso.name,
          name: iso.name,
          distro: parsed.distro,
          arch: parsed.arch,
          status: 'downloaded',
          download_status: 'downloaded',
          size_bytes: iso.size_bytes,
          mod_time: iso.mod_time,
          _isStandalone: true
        });
      });

      var sections = [];
      Object.keys(groups).sort().forEach(function (distro) {
        sections.push({ distro: distro, entries: groups[distro] });
      });
      return sections;
    }, [catalog, isos, catFilter]);

    // ── Actions ─────────────────────────────────────────────────────────
    function handleCheckAll() {
      setCheckAllBtn(true);
      Api.post('/admin/os-install/catalog/check-all', {}).then(function (r) {
        setCheckAllBtn(false);
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        if (r.data) {
          var d = r.data;
          if (d.status === 'new_version') {
            var fnUrl = d.found_url ? d.found_url.split('/').pop() : '';
            showToast(fnUrl ? t('catalog_new_version') + ': ' + Helpers.escapeHtml(fnUrl) : t('catalog_new_version'), 'success');
          } else if (d.status === 'up_to_date') {
            showToast(t('catalog_up_to_date'), 'success');
          } else if (d.status === 'no_match') {
            showToast(t('catalog_no_match'), 'success');
          } else {
            showToast(t('catalog_no_match'), 'success');
          }
        } else showToast(r.message || 'OK', 'success');
        fetchData();
      });
    }

    function handleCheckEntry(id) {
      Api.post('/admin/os-install/catalog/' + id + '/check', {}).then(function (r) {
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        if (r.data) {
          var d = r.data;
          if (d.status === 'new_version') {
            var fnUrl = d.found_url ? d.found_url.split('/').pop() : '';
            showToast(fnUrl ? t('catalog_new_version') + ': ' + Helpers.escapeHtml(fnUrl) : t('catalog_new_version'), 'success');
          } else if (d.status === 'up_to_date') {
            showToast(t('catalog_up_to_date'), 'success');
          } else if (d.status === 'no_match') {
            var entry = catalog.find(function (e) { return e.id === id; });
            if (entry && entry.check_url) {
              showToast(t('catalog_no_match') + ': ' + Helpers.escapeHtml(entry.check_url), 'success');
            } else {
              showToast(t('catalog_no_match'), 'success');
            }
          } else {
            showToast(t('catalog_no_match'), 'success');
          }
        } else showToast(r.message || 'OK', 'success');
        fetchData();
      });
    }

    function handleDownloadEntry(id) {
      Api.get('/admin/dashboard/summary', { silent: true }).then(function (r) {
        var diskPct = (r && r.success && r.data && r.data.system) ? r.data.system.disk_usage_percent : 0;
        if (diskPct > 95) {
          showToast((t('disk_full_error') || 'Insufficient disk space (' + Math.round(diskPct) + '% used)'), 'error');
          return;
        }
        if (diskPct > 85) {
          showToast((t('disk_space_warning') || 'Low disk space: ' + Math.round(diskPct) + '% used'), 'warning');
        }
        Api.post('/admin/os-install/catalog/' + id + '/download', {}).then(function (r) {
          if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(r.message || 'OK', 'success');
          fetchData();
        });
      });
    }

    function handleRetry(id) {
      Api.post('/admin/os-install/catalog/' + id + '/retry', {}).then(function (r) {
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        showToast(r.message || t('catalog_retry_queued'), 'success');
        fetchData();
      });
    }

    function handleCancel(id) {
      showConfirmModal(t('catalog_cancel_download_confirm'), function (mb) {
        var o = mb.textContent; mb.disabled = true; mb.textContent = '...';
        Api.post('/admin/os-install/catalog/' + id + '/cancel', {}).then(function (r) {
          mb.disabled = false; mb.textContent = o;
          if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(t('catalog_download_cancelled'), 'success');
          fetchData();
        });
      });
    }

    function handleEditEntry(e) {
      previousFocusRef.current = document.activeElement;
      setEditEntry(e);
    }

    function handleDeleteEntry(e) {
      showConfirmModal(t('catalog_delete_confirm', { name: e.name }), function (mb) {
        var o = mb.textContent; mb.disabled = true; mb.textContent = '...';
        Api.delete('/admin/os-install/catalog/' + e.id).then(function (r) {
          if (!r || !r.success) { mb.disabled = false; mb.textContent = o; showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(t('catalog_deleted'), 'success');
          fetchData();
        });
      });
    }

    function handleToggleAuto(id, val, ev) {
      var cb = ev.target;
      cb.setAttribute('aria-checked', val);
      Api.put('/admin/os-install/catalog/' + id, { auto_update: val }).then(function (r) {
        if (!r || !r.success) { cb.checked = !val; cb.setAttribute('aria-checked', !val); showToast((r && r.message) || t('error'), 'error'); return; }
        showToast(t('catalog_updated'), 'success');
      });
    }

    function handleDownloadAll() {
      Api.post('/admin/os-install/catalog/download-all', {}).then(function (r) {
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        showToast(r.message || 'OK', 'success');
        fetchData();
      });
    }

    function handleAddCatEntry() {
      previousFocusRef.current = document.activeElement;
      setShowCatModal(true);
    }

    function closeCatModal() {
      setShowCatModal(false);
      setEditEntry(null);
      if (previousFocusRef.current && previousFocusRef.current.focus) previousFocusRef.current.focus();
    }

    function handleDownloadISO(){previousFocusRef.current=document.activeElement;setShowDlModal(true);}
    function closeDlModal(){setShowDlModal(false);if(previousFocusRef.current&&previousFocusRef.current.focus)previousFocusRef.current.focus();}

    function handleTriggerISO(fn, u, done) {
      Api.get('/admin/dashboard/summary', { silent: true }).then(function (r) {
        var diskPct = (r && r.success && r.data && r.data.system) ? r.data.system.disk_usage_percent : 0;
        if(diskPct>95){showToast((t('disk_full_error')||'Insufficient disk space ('+Math.round(diskPct)+'% used)'),'error');done();return;}
        if(diskPct>85)showToast((t('disk_space_warning')||'Low disk space: '+Math.round(diskPct)+'% used'),'warning');
        Api.post('/admin/os-install/iso/download', { filename: fn, url: u }).then(function (r) {
          done();
          if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(r.message || 'OK', 'success');
          fetchData();
        });
      });
    }

    function handleIsoDelete(name) {
      showConfirmModal(t('osinstall_iso_delete_confirm', { name: name }), function (mb) {
        var o = mb.textContent; mb.disabled = true; mb.textContent = t('osinstall_iso_deleting');
        Api.delete('/admin/os-install/isos/' + encodeURIComponent(name)).then(function (r) {
          if (!r || !r.success) { mb.disabled = false; mb.textContent = o; showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(t('osinstall_iso_deleted'), 'success');
          fetchData();
        });
      });
    }

    function handleOverlayClick(e) {
      if (e.target === e.currentTarget) closeCatModal();
    }
    function handleKeyDown(e) {
      if (e.key === 'Escape') closeCatModal();
    }

    // ── Render ──────────────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _subNav('iso') }} />

        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('catalog_title')}</h2>
          <div class="flex gap-2">
            <button class="btn btn-secondary btn-sm"
              onClick=${handleCheckAll} disabled=${checkAllBtn}>
              ${checkAllBtn ? '...' : t('catalog_check_all')}
            </button>
            <button class="btn btn-secondary btn-sm"
              onClick=${handleDownloadISO}>${t('iso_manual_download')}</button>
            <button class="btn btn-primary btn-sm"
              onClick=${handleAddCatEntry}>${t('catalog_add_entry')}</button>
          </div>
        </div>

        <div ref=${filterBarContainerRef} class="mb-4"></div>

        ${!loading ? html`<${DownloadSummaryBar}
          stats=${queueStats}
          onDownloadAll=${handleDownloadAll}
        />` : null}

        ${loading ? html`<div dangerouslySetInnerHTML=${{ __html: Components.skeletonTable(4, 7) }} />` : null}
        ${!loading && catError ? html`
          <div class="anim-fade-in empty-state">
            <p class="text-sm" style="color:var(--color-text-tertiary);margin-bottom:1rem">${catError}</p>
            <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setCatError(null); fetchData(); }}>${t('error_retry')}</button>
          </div>` : null}
        ${!loading && !catError && !catalog.length && !isos.length ? html`
          <div dangerouslySetInnerHTML=${{ __html: Components.emptyState({
            message: t('osinstall_empty'),
            description: t('cta_add_iso_entry_desc'),
            actionLabel: t('cta_add_iso_entry')
          }) }} />` : null}
        ${!loading && !catError && (catalog.length || isos.length) && !grouped.length ? html`
          <div dangerouslySetInnerHTML=${{ __html: Components.emptyState({ message: t('no_results') }) }} />` : null}
        ${!loading && !catError ? html`
          <div id="di-catalog">
            ${grouped.map(function (g) {
              return html`<${CatalogSection} key=${g.distro} distro=${g.distro} entries=${g.entries}
                progressMap=${progressMap} queueLookup=${queueLookup}
                onCheck=${handleCheckEntry} onDownload=${handleDownloadEntry}
                onRetry=${handleRetry} onCancel=${handleCancel}
                onEdit=${handleEditEntry} onDelete=${handleDeleteEntry}
                onDeleteStandalone=${handleIsoDelete}
                onToggleAuto=${handleToggleAuto} />`;
            })}
          </div>` : null}

        <!-- Modals -->
        ${showCatModal || editEntry ? html`
          <${CatalogEntryModal}
            entry=${editEntry}
            overlayRef=${overlayRef}
            onClose=${closeCatModal}
            onSaved=${fetchData}
            handleOverlayClick=${handleOverlayClick}
            handleKeyDown=${handleKeyDown}
          />` : null}
        ${showDlModal ? html`
          <${DownloadISOModal}
            overlayRef=${overlayRef}
            onClose=${closeDlModal}
            onSubmit=${handleTriggerISO}
          />` : null}
      </div>`;
  }

  // ── Public API ───────────────────────────────────────────────────────
  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${ISOPage} />`, app);
  }

  function destroy() {
    App.clearAllTimers();
    Components.Accordion.destroy(ACCORDION_ID);
    Components.FilterBar.destroy('iso-cat-filter');
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, destroy: destroy };
})();
