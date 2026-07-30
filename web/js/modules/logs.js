const Logs = (function () {
  'use strict';
  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  var PAGE_SIZE = 50;
  var TYPES = ['crawl', 'download', 'app'];
  var TYPE_LABELS = { crawl: t('files_crawl') || 'Crawl', download: t('common_download') || 'Download', app: 'App' };

  function LogsComponent() {
    var _type = useState('crawl');
    var type = _type[0], setType = _type[1];
    var _logs = useState([]);
    var logs = _logs[0], setLogs = _logs[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _total = useState(0);
    var total = _total[0], setTotal = _total[1];
    var _offset = useState(0);
    var offset = _offset[0], setOffset = _offset[1];

    var typeRef = useRef('crawl');
    var mountedRef = useRef(true);
    var initRef = useRef(false);

    async function fetchLogs(typeVal, offsetVal, append) {
      try {
        var r = await Api.getWithHeaders('/admin/logs?type=' + typeVal + '&limit=' + PAGE_SIZE + '&offset=' + offsetVal, { silent: true });
        if (!r || !r.data || !r.data.success) { setLoading(false); return; }
        var d = r.data.data || [];
        setTotal(r.total || 0);
        setLoading(false);
        if (append) {
          setLogs(function (prev) { return prev.concat(d); });
        } else {
          setLogs(d);
        }
      } catch (e) {
        setLoading(false);
      }
    }

    function handleTabClick(tp) {
      if (tp === type) return;
      typeRef.current = tp;
      setType(tp);
      setOffset(0);
      setLogs([]);
      setTotal(0);
      setLoading(false);
      fetchLogs(tp, 0, false);
    }

    function handleLoadMore(currentOffset) {
      var newOffset = currentOffset + PAGE_SIZE;
      setOffset(newOffset);
      fetchLogs(typeRef.current, newOffset, true);
    }

    // Initial load + auto-refresh timer
    useEffect(function () {
      mountedRef.current = true;
      fetchLogs('crawl', 0, false);
      var tid = setInterval(function () {
        if (!mountedRef.current) return;
        var curType = typeRef.current;
        setOffset(0);
        fetchLogs(curType, 0, false);
      }, 15000);
      return function () {
        mountedRef.current = false;
        clearInterval(tid);
      };
    }, []);

    // Pagination: re-render after state updates
    useEffect(function () {
      Components.renderPagination('logs-pagination', {
        offset: offset,
        limit: PAGE_SIZE,
        total: total,
        onLoadMore: handleLoadMore
      });
      return function () {
        Components.removePagination('logs-pagination');
      };
    }, [offset, total, logs.length]);

    // ── Loading skeleton ───────────────────────────────────────────────────
    if (loading) {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('logs') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('logs_title') || 'Logs'}</h1>
            <div class="module-tabs">
              ${TYPES.map(function (v) {
                return html`<a href="javascript:void(0)" class="module-tab logs-tab ${v === 'crawl' ? 'active' : ''}" onClick=${function () { handleTabClick(v); }}>${TYPE_LABELS[v]}</a>`;
              })}
            </div>
            <div class="card" style="overflow:hidden">
              <div style="max-height:70vh;overflow-y:auto;padding:0.5rem">
                ${[1,2,3,4,5].map(function () { return html`<div class="skeleton" style="height:3rem;margin:0.25rem"></div>`; })}
              </div>
            </div>
          </div>
        </div>`;
    }

    // ── Log level badge ────────────────────────────────────────────────────
    var lvlClasses = { error: 'badge-error', warn: 'badge-warning', info: 'badge-blue', debug: 'badge-default' };

    // ── List content ───────────────────────────────────────────────────────
    var listContent;
    if (!logs.length) {
      listContent = html`
        <div class="empty-state">
          <p class="text-sm" style="color:var(--color-text-tertiary)">${t('cta_no_logs')}</p>
          <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${t('cta_no_logs_desc')}</p>
        </div>`;
    } else {
      listContent = logs.map(function (e) {
        var bc = lvlClasses[e.level] || 'badge-default';
        return html`
          <div class="log-entry" key=${e.id} data-id="${e.id}">
            <div class="flex items-center gap-2 mb-1">
              <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.formatTime(e.timestamp)}</span>
              <span class="badge ${bc}">${Helpers.escapeHtml(e.level)}</span>
              ${e.source ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(e.source)}</span>` : null}
            </div>
            <div class="text-sm" style="color:var(--color-text-secondary)">${e.message}</div>
          </div>`;
      });
    }

    return html`
      <div>
        <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('logs') }} />
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <h1 class="text-xl font-bold tracking-tight mb-4" style="color:var(--color-text)">${t('logs_title') || 'Logs'}</h1>
          <div class="module-tabs">
            ${TYPES.map(function (v) {
              return html`<a href="javascript:void(0)" class="module-tab logs-tab ${v === type ? 'active' : ''}" onClick=${function () { handleTabClick(v); }}>${TYPE_LABELS[v]}</a>`;
            })}
          </div>
          <div class="card" style="overflow:hidden">
            <div style="max-height:70vh;overflow-y:auto;padding:0.5rem">
              ${listContent}
            </div>
            <div id="logs-pagination" style="border-top:1px solid var(--color-border)"></div>
          </div>
        </div>
      </div>`;
  }

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${LogsComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
