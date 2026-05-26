
const ShareFiles = (function () {
  'use strict';

  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useMemo = PreactBridge.useMemo;

  // ── Nav tabs ────────────────────────────────────────────────────────────
  function NavTabs() {
    return html`
      <nav class="module-tabs">
        <a href="#/share" class="module-tab">${t('webdav_connection')}</a>
        <a href="#/share/files" class="module-tab active">${t('webdav_files')}</a>
        <a href="#/share/settings" class="module-tab">${t('settings')}</a>
      </nav>`;
  }

  // ── Breadcrumb ──────────────────────────────────────────────────────────
  function Breadcrumb(props) {
    var segments = props.currentPath ? props.currentPath.split('/') : [];
    var parts = [
      html`<span class="sf-bc-seg cursor-pointer"
        onClick=${function () { props.onNavigate(''); }}
        style="color:var(--color-text-secondary)">${t('breadcrumb_webdav')}</span>`
    ];
    var accumulated = '';
    for (var i = 0; i < segments.length; i++) {
      accumulated += (i > 0 ? '/' : '') + segments[i];
      parts.push(html`<span style="color:var(--color-text-quaternary);margin:0 0.25rem">/</span>`);
      var isLast = (i === segments.length - 1);
      if (isLast) {
        parts.push(html`<span style="color:var(--color-text-primary);font-weight:var(--font-weight-medium)">${Helpers.escapeHtml(segments[i])}</span>`);
      } else {
        parts.push(html`<span class="sf-bc-seg cursor-pointer"
          onClick=${(function (p) { return function () { props.onNavigate(p); }; })(accumulated)}
          style="color:var(--color-text-secondary)">${Helpers.escapeHtml(segments[i])}</span>`);
      }
    }
    return html`<div class="sf-breadcrumb" style="font-size:0.8125rem;padding:0.5rem 0;margin-bottom:0.5rem;display:flex;align-items:center;flex-wrap:wrap">${parts}</div>`;
  }

  // ── File table ──────────────────────────────────────────────────────────
  function FileTable(props) {
    var _sort = useState({ by: 'name', dir: 'asc' });
    var sort = _sort[0], setSort = _sort[1];

    function handleSort(col) {
      setSort(function (prev) {
        if (prev.by === col) return { by: col, dir: prev.dir === 'asc' ? 'desc' : 'asc' };
        return { by: col, dir: 'asc' };
      });
    }

    var sorted = useMemo(function () {
      return props.files.slice().sort(function (a, b) {
        if (a.is_dir && !b.is_dir) return -1;
        if (!a.is_dir && b.is_dir) return 1;
        var cmp = 0;
        if (sort.by === 'size') cmp = (a.size || 0) - (b.size || 0);
        else if (sort.by === 'type') cmp = 0;
        else {
          var na = (a.name || '').toLowerCase(), nb = (b.name || '').toLowerCase();
          cmp = na < nb ? -1 : na > nb ? 1 : 0;
        }
        return sort.dir === 'desc' ? -cmp : cmp;
      });
    }, [props.files, sort.by, sort.dir]);

    function arrow(col) {
      if (sort.by !== col) return '';
      return sort.dir === 'asc' ? ' \u25B2' : ' \u25BC';
    }

    var thStyle = 'color:var(--color-text-secondary);font-weight:var(--font-weight-medium);white-space:nowrap';

    return html`
      <table class="w-full" style="font-size:0.8125rem;border-collapse:collapse">
        <thead>
          <tr style="border-bottom:2px solid var(--color-border)">
            <th class="text-left py-2 px-3 cursor-pointer" style="${thStyle}" onClick=${function () { handleSort('name'); }}>${t('webdav_file_name')}${arrow('name')}</th>
            <th class="text-right py-2 px-3 cursor-pointer" style="${thStyle}" onClick=${function () { handleSort('size'); }}>${t('webdav_file_size')}${arrow('size')}</th>
            <th class="text-left py-2 px-3 cursor-pointer" style="${thStyle}" onClick=${function () { handleSort('type'); }}>${t('webdav_file_type')}${arrow('type')}</th>
          </tr>
        </thead>
        <tbody>
          ${props.parentPath !== null ? html`
            <tr class="sf-parent-row cursor-pointer" style="border-bottom:1px solid var(--color-border)"
                onClick=${function () { props.onNavigate(props.parentPath); }}>
              <td class="py-2 px-3" style="color:var(--color-text-secondary)">
                <div class="flex items-center gap-2">
                  <span style="color:var(--color-text-quaternary);flex-shrink:0" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.folder }} />
                  <span>..</span>
                </div>
              </td>
              <td class="py-2 px-3 text-right" style="color:var(--color-text-secondary);white-space:nowrap">-</td>
              <td class="py-2 px-3"><span class="badge badge-default" style="font-size:0.6875rem">${t('webdav_file_folder')}</span></td>
            </tr>
          ` : null}
          ${sorted.map(function (f) {
            var icon = f.is_dir ? Helpers.ICONS.folder : Helpers.ICONS.file;
            var badgeCls = f.is_dir ? 'badge-default' : 'badge-blue';
            var badgeText = f.is_dir ? t('webdav_file_folder') : t('webdav_file_file');
            var fullPath = props.currentPath ? props.currentPath + '/' + f.name : f.name;
            return html`
              <tr class="${f.is_dir ? 'sf-dir-row cursor-pointer' : ''}"
                  style="border-bottom:1px solid var(--color-border)"
                  onClick=${f.is_dir ? function () { props.onNavigate(fullPath); } : null}>
                <td class="py-2 px-3" style="color:var(--color-text)">
                  <div class="flex items-center gap-2">
                    <span style="color:var(--color-text-quaternary);flex-shrink:0" dangerouslySetInnerHTML=${{ __html: icon }} />
                    <span class="truncate">${Helpers.escapeHtml(f.name)}</span>
                  </div>
                </td>
                <td class="py-2 px-3 text-right" style="color:var(--color-text-secondary);white-space:nowrap">${f.is_dir ? '-' : Helpers.formatBytes(f.size)}</td>
                <td class="py-2 px-3"><span class="badge ${badgeCls}" style="font-size:0.6875rem">${badgeText}</span></td>
              </tr>`;
          })}
        </tbody>
      </table>`;
  }

  // ── Main component ──────────────────────────────────────────────────────
  function ShareFilesComponent() {
    var _files = useState([]);
    var files = _files[0], setFiles = _files[1];
    var _path = useState('');
    var currentPath = _path[0], setCurrentPath = _path[1];
    var _parent = useState(null);
    var parentPath = _parent[0], setParentPath = _parent[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];

    var mountedRef = useRef(true);
    var initRef = useRef(false);

    function fetchFiles(path) {
      if (!initRef.current) setLoading(true);
      setError(null);
      var url = '/admin/webdav/files';
      if (path) url += '?path=' + encodeURIComponent(path);
      Api.get(url).then(function (r) {
        if (!mountedRef.current) return;
        if (!r || !r.success) {
          setError((r && r.message) || t('error'));
          setLoading(false);
          return;
        }
        setFiles((r.data && r.data.files) || []);
        setCurrentPath((r.data && r.data.currentPath) || '');
        setParentPath((r.data && r.data.parentPath !== undefined) ? r.data.parentPath : null);
        setLoading(false);
        initRef.current = true;
      });
    }

    function handleNavigate(path) {
      fetchFiles(path || '');
    }

    useEffect(function () {
      mountedRef.current = true;
      fetchFiles('');
      return function () { mountedRef.current = false; };
    }, []);

    var isEmpty = !files.length && parentPath === null && !loading && !error;

    return html`
      <div class="p-4 md:p-6 max-w-4xl mx-auto">
        <${NavTabs} />
        <div class="flex items-center justify-between mb-4">
          <p class="text-xs" style="color:var(--color-text-tertiary)">${t('webdav_files_note')}</p>
          <button class="btn btn-ghost btn-sm"
                  onClick=${function () { fetchFiles(currentPath); }}>${t('webdav_files_refresh')}</button>
        </div>
        <div class="table-container">
          ${loading ? html`<div dangerouslySetInnerHTML=${{ __html: Helpers.loadingSpinner() }} />` : null}
          ${error ? html`<div dangerouslySetInnerHTML=${{ __html: Helpers.errorMessage(error) }} />` : null}
          ${!loading && !error && isEmpty ? html`
            <div class="empty-state" role="region" aria-label="File upload zone">
              <div style="color:var(--color-text-quaternary)" dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
              <p class="text-sm" style="color:var(--color-text-tertiary);margin-top:0.75rem">${t('webdav_files_empty')}</p>
              <div style="margin-top:0.75rem">
                <button class="btn btn-primary btn-sm" disabled title="${t('cta_upload_webdav_desc')}">${t('cta_upload_webdav')}</button>
              </div>
              <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_upload_webdav_desc')}</p>
            </div>
          ` : null}
          ${!loading && !error && !isEmpty ? html`
            <${Breadcrumb} currentPath=${currentPath} onNavigate=${handleNavigate} />
            <${FileTable} files=${files} currentPath=${currentPath} parentPath=${parentPath} onNavigate=${handleNavigate} />
          ` : null}
        </div>
      </div>`;
  }

  // ── Public API ──────────────────────────────────────────────────────────
  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${ShareFilesComponent} />`, app);
  }

  function cleanup() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, cleanup: cleanup };
})();
