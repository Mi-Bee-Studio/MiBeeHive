const Dashboard = (function () {
  'use strict';

  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;
  var Fragment = PreactBridge.Fragment;

  // ── CSS variable helpers ─────────────────────────────────────────────
  function _cv(n) { return getComputedStyle(document.documentElement).getPropertyValue(n).trim(); }
  function _toHex(c) {
    if (!c) return '#6b7280';
    c = c.trim();
    if (c.charAt(0) === '#') return c.length === 4 ? '#' + c[1]+c[1]+c[2]+c[2]+c[3]+c[3] : c.substring(0, 7);
    var m = c.match(/rgba?\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/);
    if (m) return '#' + ((1<<24)+(+m[1]<<16)+(+m[2]<<8)+(+m[3])).toString(16).slice(1);
    return '#6b7280';
  }
  function _alpha(hex, a) {
    if (!hex || hex.charAt(0) !== '#') return 'rgba(0,0,0,' + a + ')';
    var r = parseInt(hex.slice(1,3),16), g = parseInt(hex.slice(3,5),16), b = parseInt(hex.slice(5,7),16);
    return 'rgba(' + r + ',' + g + ',' + b + ',' + a + ')';
  }
  function _cc() {
    return {
      p: _toHex(_cv('--color-primary')), s: _toHex(_cv('--color-success')),
      w: _toHex(_cv('--color-accent-amber')), e: _toHex(_cv('--color-error')),
      t: _toHex(_cv('--color-text-secondary')), t3: _toHex(_cv('--color-text-tertiary')),
      b: _toHex(_cv('--color-border'))
    };
  }

  // ── Time helpers ─────────────────────────────────────────────────────
  function _fl(ts) { var d = new Date(ts); return (d.getMonth()+1)+'/'+d.getDate()+' '+d.getHours()+':'+String(d.getMinutes()).padStart(2,'0'); }
  function _ft() { var d = new Date(); return d.getHours()+':'+String(d.getMinutes()).padStart(2,'0')+':'+String(d.getSeconds()).padStart(2,'0'); }
  function _ago(ts) {
    var now = Date.now(), dd = new Date(ts).getTime(), diff = now - dd;
    if (diff < 60000) return t('activity_ago', { count: '1m' });
    if (diff < 3600000) return t('activity_ago', { count: Math.floor(diff/60000) + 'm' });
    if (diff < 86400000) return t('activity_ago', { count: Math.floor(diff/3600000) + 'h' });
    return t('activity_ago', { count: Math.floor(diff/86400000) + 'd' });
  }

  function _isFirstRun(data) {
    var files = data.files || {}, deploy = data.deploy || {}, share = data.share || {};
    return (files.project_count || 0) === 0 && (deploy.config_count || 0) === 0 && (deploy.iso_count || 0) === 0 && (share.file_count || 0) === 0;
  }

  function _chartBaseOpts(extra) {
    var c = _cc();
    var base = {
      responsive: true, maintainAspectRatio: true, animation: { duration: 600, easing: 'easeOutQuart' },
      scales: {
        x: { grid: { display: false }, ticks: { color: c.t3, font: { size: 10 }, maxRotation: 45 } },
        y: { beginAtZero: true, grid: { color: _alpha(c.b, 0.04) }, border: { display: false }, ticks: { color: c.t3, font: { size: 10 } } }
      },
      plugins: {
        legend: { display: false },
        tooltip: {
          backgroundColor: 'rgba(0,0,0,0.78)', titleFont: { size: 11 }, bodyFont: { size: 11 },
          padding: 8, cornerRadius: 6, displayColors: true, boxPadding: 4
        }
      },
      elements: { point: { radius: 0, hoverRadius: 4 }, line: { tension: 0.4, borderWidth: 2 } }
    };
    if (extra) for (var k in extra) base[k] = extra[k];
    return base;
  }

  // ── Chart.js loader ──────────────────────────────────────────────────
  var _chartJsReady = false;
  function _loadChartJs() {
    return new Promise(function (res) {
      if (window.Chart) { _chartJsReady = true; res(); return; }
      var s = document.createElement('script');
      s.src = 'https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js';
      s.onload = function () { _chartJsReady = true; res(); };
      s.onerror = function () { console.error('Chart.js load failed'); res(); };
      document.head.appendChild(s);
    });
  }

  // ── Activity icons (SVG) ─────────────────────────────────────────────
  var _actColors = {
    crawl_success: 'var(--color-success)', crawl_error: 'var(--color-error)',
    file_downloaded: 'var(--color-accent-blue)', iso_queued: 'var(--color-warning)',
    iso_downloaded: 'var(--color-success)'
  };

  function _actIcon(type) {
    var icons = {
      crawl_success: '<svg aria-hidden="true" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
      crawl_error: '<svg aria-hidden="true" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z"/></svg>',
      file_downloaded: '<svg aria-hidden="true" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>',
      iso_queued: '<svg aria-hidden="true" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
      iso_downloaded: '<svg aria-hidden="true" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>'
    };
    return icons[type] || icons.crawl_success;
  }

  // ── Quick action icons ───────────────────────────────────────────────
  var _qaIcons = {
    files: '<svg aria-hidden="true" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>',
    deploy: '<svg aria-hidden="true" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"/></svg>',
    share: '<svg aria-hidden="true" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z"/></svg>',
    containers: '<svg aria-hidden="true" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"/></svg>'
  };

  // ── Mini bar component ───────────────────────────────────────────────
  function MiniBar(props) {
    var pct = props.pct || 0;
    var color = pct > 80 ? 'var(--color-error)' : (pct > 50 ? 'var(--color-warning)' : 'var(--color-success)');
    return html`
      <div style="width:48px;text-align:center">
        <div class="text-xs font-medium" style="color:var(--color-text-tertiary)">${pct.toFixed(0)}%</div>
        <div style="height:3px;background:var(--color-bg-tertiary);border-radius:2px;overflow:hidden;margin-top:2px">
          <div style=${{ height:'100%', width: Math.min(pct, 100) + '%', background: color, borderRadius:'2px', transition:'width 0.5s ease' }}></div>
        </div>
      </div>`;
  }

  // ── Status card component ────────────────────────────────────────────
  function StatusCard(props) {
    var Tag = props.href ? 'a' : 'div';
    var extra = props.href ? { href: props.href } : {};
    return html`
      <${Tag} ...${extra} class="card card-hover stat-card-accent ${props.accent}" style="padding:1.25rem;text-decoration:none;display:block">
        <div class="text-xs font-medium uppercase tracking-wide mb-2" style="color:var(--color-text-tertiary)">${t(props.title)}</div>
        ${props.stats.map(function (s) {
          return html`
            <div class="flex items-center justify-between py-0.5">
              <span class="text-sm" style="color:var(--color-text-secondary)">${s.l}</span>
              <span class="text-sm font-semibold" style="color:var(--color-text)">${s.v}</span>
            </div>`;
        })}
      <//>`;
  }

  // ── System stat tile ─────────────────────────────────────────────────
  function SysStat(props) {
    return html`
      <div style="padding:0.5rem 0.75rem;border-radius:var(--radius-md);background:var(--color-bg-secondary)">
        <div class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(props.label)}</div>
        <div class="text-sm font-semibold" style="color:var(--color-text)">${Helpers.escapeHtml(props.value)}</div>
      </div>`;
  }

  // ── Activity item component ──────────────────────────────────────────
  function ActivityItem(props) {
    var evt = props.evt;
    var color = _actColors[evt.type] || 'var(--color-text-tertiary)';
    var labelKey = 'activity_' + evt.type;
    var label = t(labelKey, { name: evt.title || '' });
    var timeStr = evt.timestamp ? _ago(evt.timestamp) : '';
    var iconColorHex = _toHex(color);
    return html`
      <div class="activity-item" data-id=${evt.id}>
        <div style=${{ width:'1.5rem', height:'1.5rem', borderRadius:'50%', display:'flex', alignItems:'center', justifyContent:'center', flexShrink:0, background: _alpha(iconColorHex, 0.1), color: color }} dangerouslySetInnerHTML=${{ __html: _actIcon(evt.type) }} />
        <div style="flex:1;min-width:0">
          <div class="text-sm" style="color:var(--color-text)">${Helpers.escapeHtml(label)}</div>
          ${evt.subtitle ? html`<div class="text-xs" style="color:var(--color-text-tertiary)">${evt.subtitle}</div>` : null}
        </div>
        <div class="text-xs" style="color:var(--color-text-tertiary);flex-shrink:0">${timeStr}</div>
      </div>`;
  }

  // ── Queue item component ─────────────────────────────────────────────
  function QueueItem(props) {
    var f = props.item;
    var dl = f.status === 'downloading';
    var sk = dl ? t('queue_downloading') : t('queue_pending');
    var sc = dl ? 'queue-status-downloading' : 'queue-status-pending';
    return html`
      <div class="queue-item" data-id=${f.id} data-status=${f.status}>
        <div class="flex flex-col gap-1 min-w-0" style="flex:1">
          <span class="text-sm font-medium truncate" style="color:var(--color-text)" title=${f.filename || ''}>${f.filename || ''}</span>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.formatBytes(f.size_bytes)}</span>
          ${dl ? html`
            <div class="dl-progress" data-id=${f.id} role="status">
              <div class="dl-progress-bar">
                <div class="dl-progress-fill" style="width:0%"></div>
              </div>
              <span class="dl-progress-text">0%</span>
            </div>` : null}
        </div>
        <span class="queue-status ${sc}">${sk}</span>
      </div>`;
  }

  // ── ISO queue item component ─────────────────────────────────────────
  function ISOQueueItem(props) {
    var it = props.item;
    var dl = it.download_status === 'downloading';
    var sk = dl ? t('iso_queue_downloading') : t('iso_queue_pending');
    var sc = dl ? 'queue-status-downloading' : 'queue-status-pending';
    var fn = it.current_url ? it.current_url.split('/').pop() : '';
    return html`
      <div class="iso-queue-item" data-iso-id=${it.id} data-status=${it.download_status}>
        <div class="flex flex-col gap-1 min-w-0" style="flex:1">
          <span class="text-sm font-medium truncate" style="color:var(--color-text)" title=${it.name}>${Helpers.escapeHtml(it.name)}</span>
          <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(it.distro)} / ${Helpers.escapeHtml(it.arch)}</span>
          ${dl ? html`
            <div class="dl-progress" role="status" data-iso=${fn}>
              <div class="dl-progress-bar">
                <div class="dl-progress-fill" id=${'iso-bar-' + it.id} style="width:0%"></div>
              </div>
              <span class="dl-progress-text" id=${'iso-text-' + it.id}>0%</span>
            </div>` : null}
        </div>
        <span class="queue-status ${sc}">${sk}</span>
      </div>`;
  }

  // ── Queue legend item helper ─────────────────────────────────────────
  function _qli(color, label, count) {
    return html`
      <div class="queue-stat-item">
        <span class="queue-stat-dot" style=${{ background: color }}></span>
        ${label}
        <span class="queue-stat-count">${count}</span>
      </div>`;
  }

  // ═══════════════════════════════════════════════════════════════════════
  // Main Dashboard Component
  // ═══════════════════════════════════════════════════════════════════════
  function DashboardComponent() {
    var _phase = useState('loading'); // 'loading' | 'error' | 'welcome' | 'main'
    var phase = _phase[0], setPhase = _phase[1];

    var _summary = useState(null);
    var summary = _summary[0], setSummary = _summary[1];

    var _sysStats = useState(null);
    var sysStats = _sysStats[0], setSysStats = _sysStats[1];

    var _monitorCfg = useState({ diskWarning: 80, diskCritical: 90 });
    var monitorCfg = _monitorCfg[0], setMonitorCfg = _monitorCfg[1];

    var _paused = useState(false);
    var paused = _paused[0], setPaused = _paused[1];

    var _refreshMs = useState(10000);
    var refreshMs = _refreshMs[0], setRefreshMs = _refreshMs[1];

    var _range = useState('24h');
    var range = _range[0], setRange = _range[1];

    var _updatedTime = useState('--:--:--');
    var updatedTime = _updatedTime[0], setUpdatedTime = _updatedTime[1];

    var _queueStats = useState({ pending: 0, downloading: 0, complete: 0, error: 0 });
    var queueStats = _queueStats[0], setQueueStats = _queueStats[1];

    var _queueFiles = useState([]);
    var queueFiles = _queueFiles[0], setQueueFiles = _queueFiles[1];

    var _isoData = useState({ stats: {}, items: [] });
    var isoData = _isoData[0], setIsoData = _isoData[1];

    var _activity = useState([]);
    var activity = _activity[0], setActivity = _activity[1];

    var _crawlLogs = useState([]);
    var crawlLogs = _crawlLogs[0], setCrawlLogs = _crawlLogs[1];

    var _connLost = useState(false);
    var connLost = _connLost[0], setConnLost = _connLost[1];

    var _consecFails = useState(0);

    // Refs for Chart.js instances — persist across re-renders
    var cpuMemChartRef = useRef(null);
    var netChartRef = useRef(null);
    var diskChartRef = useRef(null);
    var crawlChartRef = useRef(null);
    var queueChartRef = useRef(null);

    // Refs for DOM canvases
    var cpuMemCanvasRef = useRef(null);
    var netCanvasRef = useRef(null);
    var diskCanvasRef = useRef(null);
    var crawlCanvasRef = useRef(null);
    var queueCanvasRef = useRef(null);

    // Refs for network byte tracking
    var lastNetRxRef = useRef(0);
    var lastNetTxRef = useRef(0);

    // Mounted guard
    var mountedRef = useRef(true);

    // Inflight guards
    var sysInflightRef = useRef(false);
    var summaryInflightRef = useRef(false);
    var queueInflightRef = useRef(false);
    var isoInflightRef = useRef(false);

    // ── Data fetching helpers ───────────────────────────────────────────

    function fetchSummary() {
      return Api.get('/admin/dashboard/summary').then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchMonitorConfig() {
      return Api.get('/admin/config/monitor').then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchSystemStats() {
      return Api.get('/system/stats', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchHistory(rng) {
      return Api.get('/system/stats/history?range=' + (rng || range)).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchQueueStats() {
      return Api.get('/files/queue/stats', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchQueueFiles() {
      return Api.get('/files/queue', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchQueueProgress() {
      return Api.get('/files/queue/progress', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchISOCatalog() {
      return Api.get('/admin/os-install/catalog/queue', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchISOProgress() {
      return Api.get('/admin/os-install/catalog/progress', { silent: true }).then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    function fetchCrawlLogs() {
      return Api.get('/crawl/logs?limit=30').then(function (res) {
        if (!res || !res.success) return null;
        return res.data;
      });
    }

    // ── Initial load ────────────────────────────────────────────────────

    useEffect(function () {
      mountedRef.current = true;
      _consecFails.current = 0;

      async function init() {
        try {
          var data = await fetchSummary();
          if (!mountedRef.current) return;
          if (!data) { setPhase('error'); return; }
          var cfg = await fetchMonitorConfig();
          if (!mountedRef.current) return;
          if (cfg) {
            setMonitorCfg({ diskWarning: cfg.disk_warning_percent || 80, diskCritical: cfg.disk_critical_percent || 90 });
          }
          setSummary(data);
          setActivity(data.activity || []);
          if (_isFirstRun(data)) {
            setPhase('welcome');
            _startWelcomePolling();
          } else {
            setPhase('main');
            _loadChartJs().then(function () {
              if (!mountedRef.current) return;
              _initCharts(data);
            });
            _startMainPolling();
          }
        } catch (e) {
          if (mountedRef.current) setPhase('error');
        }
      }
      init();

      return function () {
        mountedRef.current = false;
      };
    }, []);

    // ── Chart initialization (after Chart.js loaded) ────────────────────

    function _initCharts(data) {
      var sys = data.system || {};
      var files = data.files || {};
      var c = _cc();
      var pctY = { beginAtZero:true, max:100, grid:{color:_alpha(c.b,0.04)}, border:{display:false}, ticks:{color:c.t3, font:{size:10}, callback:function(v){return v+'%';}} };
      var noX = { display:false };

      // CPU/Mem chart
      if (cpuMemCanvasRef.current && window.Chart) {
        cpuMemChartRef.current = new Chart(cpuMemCanvasRef.current, {
          type: 'line',
          data: { labels: [], datasets: [
            { label:'CPU', data:[], borderColor:c.p, backgroundColor:_alpha(c.p,0.08), fill:true, tension:0.4, borderWidth:2, pointRadius:0, hoverRadius:4 },
            { label:t('memory_usage'), data:[], borderColor:c.s, backgroundColor:_alpha(c.s,0.08), fill:true, tension:0.4, borderWidth:2, pointRadius:0, hoverRadius:4 }
          ]},
          options: _chartBaseOpts({ scales:{ x:noX, y:pctY }, plugins:{ legend:{ display:true, labels:{ color:c.t, font:{size:11}, boxWidth:12, padding:8 } } } })
        });
      }

      // Network chart
      if (netCanvasRef.current && window.Chart) {
        netChartRef.current = new Chart(netCanvasRef.current, {
          type: 'line',
          data: { labels: [], datasets: [
            { label:t('chart_rx'), data:[], borderColor:c.p, backgroundColor:'transparent', tension:0.4, borderWidth:2, pointRadius:0, hoverRadius:4 },
            { label:t('chart_tx'), data:[], borderColor:c.w, backgroundColor:'transparent', tension:0.4, borderWidth:2, pointRadius:0, hoverRadius:4 }
          ]},
          options: _chartBaseOpts({ plugins:{ legend:{ display:true, labels:{ color:c.t, font:{size:11}, boxWidth:12, padding:8 } } } })
        });
      }

      // Disk chart
      if (diskCanvasRef.current && window.Chart) {
        var used = sys.disk_used_bytes || 0;
        var total = sys.disk_total_bytes || 1;
        var pct = sys.disk_usage_percent || 0;
        var usedColor = pct > monitorCfg.diskCritical ? c.e : (pct > monitorCfg.diskWarning ? c.w : c.s);
        diskChartRef.current = new Chart(diskCanvasRef.current, {
          type: 'doughnut',
          data: {
            labels: [t('dash_disk_usage'), t('dash_disk_free')],
            datasets: [{ data: [used, total - used], backgroundColor: [usedColor, _alpha(c.b, 0.1)], borderWidth: 0, borderRadius: 4 }]
          },
          options: {
            responsive: true, maintainAspectRatio: true, cutout: '78%',
            animation: { duration: 600, easing: 'easeOutQuart' },
            plugins: { legend: { display: false }, tooltip: { enabled: true } }
          }
        });
      }

      // Crawl chart
      if (crawlCanvasRef.current && window.Chart) {
        crawlChartRef.current = new Chart(crawlCanvasRef.current, {
          type: 'line',
          data: { labels: [], datasets: [
            { label:t('chart_files_downloaded'), data:[], borderColor:c.p, backgroundColor:_alpha(c.p,0.08), tension:0.4, pointRadius:2, pointHoverRadius:5, fill:true, borderWidth:2 },
            { label:t('chart_versions_found'), data:[], borderColor:c.w, backgroundColor:_alpha(c.w,0.08), tension:0.4, pointRadius:2, pointHoverRadius:5, fill:true, borderWidth:2 }
          ]},
          options: {
            responsive:true, maintainAspectRatio:true, animation:{duration:600,easing:'easeOutQuart'},
            interaction:{intersect:false,mode:'index'},
            plugins:{legend:{labels:{color:c.t3,font:{size:11},boxWidth:12,padding:8}},tooltip:{backgroundColor:'rgba(0,0,0,0.78)',titleFont:{size:11},bodyFont:{size:11},padding:8,cornerRadius:6,boxPadding:4}},
            scales:{x:{grid:{display:false},ticks:{color:c.t3,font:{size:10},maxRotation:45}},y:{beginAtZero:true,grid:{color:_alpha(c.b,0.04)},border:{display:false},ticks:{color:c.t3,font:{size:10},stepSize:1}}}
          }
        });
      }

      // Queue chart
      if (queueCanvasRef.current && window.Chart) {
        var pe = files.queue_pending || 0, dn = files.queue_downloading || 0, co = files.queue_complete || 0, er = files.queue_error || 0;
        queueChartRef.current = new Chart(queueCanvasRef.current, {
          type: 'doughnut',
          data: {
            labels: [t('queue_pending'), t('queue_downloading'), t('queue_complete'), t('queue_failed')],
            datasets: [{ data: [pe, dn, co, er], backgroundColor: [_alpha(c.w,0.56), _alpha(c.p,0.56), _alpha(c.s,0.56), _alpha(c.e,0.56)], borderColor: [c.w, c.p, c.s, c.e], borderWidth: 2, hoverOffset: 8, borderRadius: 4 }]
          },
          options: {
            responsive:true, maintainAspectRatio:false, cutout:'65%',
            animation:{duration:600,easing:'easeOutQuart'},
            plugins:{legend:{display:false},tooltip:{backgroundColor:'rgba(0,0,0,0.78)',titleFont:{size:11},bodyFont:{size:11},padding:8,cornerRadius:6,boxPadding:4}}
          }
        });
      }

      // Load historical data
      _loadHistory();
      _loadCrawlData();
      _loadQueueData();
    }

    // ── Load history into charts ────────────────────────────────────────

    function _loadHistory() {
      fetchHistory(range).then(function (pts) {
        if (!mountedRef.current || !pts || !pts.length) return;
        var lb = pts.map(function(p){return _fl(p.timestamp);});
        var cd = pts.map(function(p){return p.cpu_usage_percent;});
        var md = pts.map(function(p){return p.memory_usage_percent;});
        var rx=[], tx=[];
        for (var i=0; i<pts.length; i++) {
          if (i===0) { rx.push(0); tx.push(0); continue; }
          var t1=new Date(pts[i-1].timestamp).getTime(), t2=new Date(pts[i].timestamp).getTime(), dt=(t2-t1)/1000;
          if (dt<=0) { rx.push(0); tx.push(0); continue; }
          rx.push(Math.round((pts[i].network_rx_bytes-pts[i-1].network_rx_bytes)/dt/1024*100)/100);
          tx.push(Math.round((pts[i].network_tx_bytes-pts[i-1].network_tx_bytes)/dt/1024*100)/100);
        }
        lastNetRxRef.current = pts[pts.length-1].network_rx_bytes;
        lastNetTxRef.current = pts[pts.length-1].network_tx_bytes;

        if (cpuMemChartRef.current) {
          cpuMemChartRef.current.data.labels = lb;
          cpuMemChartRef.current.data.datasets[0].data = cd;
          cpuMemChartRef.current.data.datasets[1].data = md;
          cpuMemChartRef.current.update();
        }
        if (netChartRef.current) {
          netChartRef.current.data.labels = lb;
          netChartRef.current.data.datasets[0].data = rx;
          netChartRef.current.data.datasets[1].data = tx;
          netChartRef.current.update();
        }
      });
    }

    function _loadCrawlData() {
      fetchCrawlLogs().then(function (logs) {
        if (!logs || !logs.length || !crawlChartRef.current) return;
        logs.reverse();
        crawlChartRef.current.data.labels = logs.map(function(l){return _fl(l.started_at);});
        crawlChartRef.current.data.datasets[0].data = logs.map(function(l){return l.files_downloaded||0;});
        crawlChartRef.current.data.datasets[1].data = logs.map(function(l){return l.versions_found||0;});
        crawlChartRef.current.update();
      });
    }

    function _loadQueueData() {
      Promise.all([fetchQueueStats(), fetchQueueFiles()]).then(function (r) {
        var st = r[0], fl = r[1];
        if (st) {
          var qpe = st.pending||0, qdn = st.downloading||0, qco = st.complete||0, qer = st.error||0;
          setQueueStats({ pending: qpe, downloading: qdn, complete: qco, error: qer });
          if (queueChartRef.current) {
            queueChartRef.current.data.datasets[0].data = [qpe, qdn, qco, qer];
            queueChartRef.current.update('none');
          }
        }
        if (fl && Array.isArray(fl)) setQueueFiles(fl);
      });
    }

    // ── Polling: system stats (10s) ─────────────────────────────────────

    var sysTimerRef = useRef(null);
    var summaryTimerRef = useRef(null);
    var queueTimerRef = useRef(null);
    var isoTimerRef = useRef(null);
    var welcomeSysTimerRef = useRef(null);
    var welcomeSummaryTimerRef = useRef(null);

    function _clearAllTimers() {
      if (sysTimerRef.current) clearInterval(sysTimerRef.current);
      if (summaryTimerRef.current) clearInterval(summaryTimerRef.current);
      if (queueTimerRef.current) clearInterval(queueTimerRef.current);
      if (isoTimerRef.current) clearInterval(isoTimerRef.current);
      if (welcomeSysTimerRef.current) clearInterval(welcomeSysTimerRef.current);
      if (welcomeSummaryTimerRef.current) clearInterval(welcomeSummaryTimerRef.current);
    }

    function _startMainPolling() {
      _clearAllTimers();

      // System stats poll (10s default)
      sysTimerRef.current = setInterval(function () {
        if (paused || sysInflightRef.current || !mountedRef.current) return;
        sysInflightRef.current = true;
        fetchSystemStats().then(function (d) {
          if (!mountedRef.current || !d) return;
          _consecFails.current = 0;
          if (connLost) setConnLost(false);

          // Update system stats in state
          setSysStats(d);

          var lb = _ft();
          // Push data into cpu/mem chart
          if (cpuMemChartRef.current) {
            cpuMemChartRef.current.data.labels.push(lb);
            cpuMemChartRef.current.data.datasets[0].data.push(d.cpu_usage_percent);
            cpuMemChartRef.current.data.datasets[1].data.push(d.memory_usage_percent);
            if (cpuMemChartRef.current.data.labels.length > 120) {
              cpuMemChartRef.current.data.labels.shift();
              cpuMemChartRef.current.data.datasets[0].data.shift();
              cpuMemChartRef.current.data.datasets[1].data.shift();
            }
            cpuMemChartRef.current.update('none');
          }
          // Push data into network chart
          if (netChartRef.current) {
            var rr = lastNetRxRef.current > 0 ? Math.max(0, (d.network_rx_bytes - lastNetRxRef.current) / (refreshMs / 1000) / 1024) : 0;
            var tr = lastNetTxRef.current > 0 ? Math.max(0, (d.network_tx_bytes - lastNetTxRef.current) / (refreshMs / 1000) / 1024) : 0;
            lastNetRxRef.current = d.network_rx_bytes;
            lastNetTxRef.current = d.network_tx_bytes;
            netChartRef.current.data.labels.push(lb);
            netChartRef.current.data.datasets[0].data.push(Math.round(rr*100)/100);
            netChartRef.current.data.datasets[1].data.push(Math.round(tr*100)/100);
            if (netChartRef.current.data.labels.length > 120) {
              netChartRef.current.data.labels.shift();
              netChartRef.current.data.datasets[0].data.shift();
              netChartRef.current.data.datasets[1].data.shift();
            }
            netChartRef.current.update('none');
          }
          setUpdatedTime(_ft());
        }).catch(function () {
          _consecFails.current++;
          if (_consecFails.current >= 3) setConnLost(true);
        }).finally(function () { sysInflightRef.current = false; });
      }, refreshMs);

      // Summary poll (30s)
      summaryTimerRef.current = setInterval(function () {
        if (paused || summaryInflightRef.current || !mountedRef.current) return;
        summaryInflightRef.current = true;
        fetchSummary().then(function (data) {
          if (!mountedRef.current || !data) return;
          _consecFails.current = 0;
          if (connLost) setConnLost(false);
          setSummary(data);
          setActivity(data.activity || []);
          // Update disk chart in-place
          var sys = data.system || {};
          if (diskChartRef.current) {
            var used = sys.disk_used_bytes || 0;
            var total = sys.disk_total_bytes || 1;
            var pct = sys.disk_usage_percent || 0;
            var c = _cc();
            var usedColor = pct > monitorCfg.diskCritical ? c.e : (pct > monitorCfg.diskWarning ? c.w : c.s);
            diskChartRef.current.data.datasets[0].data = [used, total - used];
            diskChartRef.current.data.datasets[0].backgroundColor = [usedColor, _alpha(c.b, 0.1)];
            diskChartRef.current.update('none');
          }
          setUpdatedTime(_ft());
        }).catch(function () {
          _consecFails.current++;
          if (_consecFails.current >= 3) setConnLost(true);
        }).finally(function () { summaryInflightRef.current = false; });
      }, 30000);

      // Queue poll (5s)
      queueTimerRef.current = setInterval(function () {
        if (paused || queueInflightRef.current || !mountedRef.current) return;
        queueInflightRef.current = true;
        Promise.all([fetchQueueStats(), fetchQueueFiles(), fetchQueueProgress()]).then(function (r) {
          if (!mountedRef.current) return;
          var st = r[0], fl = r[2]; // r[2] is progress data, r[1] is queue files
          if (st) {
            var qpe = st.pending||0, qdn = st.downloading||0, qco = st.complete||0, qer = st.error||0;
            setQueueStats({ pending: qpe, downloading: qdn, complete: qco, error: qer });
            if (queueChartRef.current) {
              queueChartRef.current.data.datasets[0].data = [qpe, qdn, qco, qer];
              queueChartRef.current.update('none');
            }
          }
          if (r[1] && Array.isArray(r[1])) setQueueFiles(r[1]);
          // Progress updates handled via DOM refs
          if (fl) {
            var ids = Object.keys(fl);
            for (var i = 0; i < ids.length; i++) {
              var p = fl[ids[i]];
              var bEl = document.getElementById('dl-bar-' + ids[i]);
              var tEl = document.getElementById('dl-text-' + ids[i]);
              if (bEl) bEl.style.width = p.percent + '%';
              if (tEl) tEl.textContent = p.percent + '%';
            }
          }
        }).catch(function () {}).finally(function () { queueInflightRef.current = false; });
      }, 5000);

      // ISO poll (5s)
      isoTimerRef.current = setInterval(function () {
        if (paused || isoInflightRef.current || !mountedRef.current) return;
        isoInflightRef.current = true;
        Promise.all([fetchISOCatalog(), fetchISOProgress()]).then(function (r) {
          if (!mountedRef.current) return;
          var d = r[0], prog = r[1];
          if (d) setIsoData({ stats: d.stats || {}, items: d.items || [] });
          // ISO progress updates via DOM
          if (prog && Array.isArray(prog)) {
            var bn = {};
            for (var i = 0; i < prog.length; i++) bn[prog[i].filename] = prog[i];
            var ct = document.getElementById('iso-queue-content');
            if (!ct) return;
            var pe = ct.querySelectorAll('.dl-progress[data-iso]');
            for (var j = 0; j < pe.length; j++) {
              var el = pe[j], fn = el.getAttribute('data-iso'), p = bn[fn];
              if (p) {
                var b = el.querySelector('.dl-progress-fill');
                var x = el.querySelector('.dl-progress-text');
                if (b) b.style.width = p.percent + '%';
                if (x) x.textContent = p.percent + '%';
              }
            }
          }
        }).catch(function () {}).finally(function () { isoInflightRef.current = false; });
      }, 5000);

      // Initial queue + ISO load
      _loadQueueData();
      _loadISOData();
    }

    function _startWelcomePolling() {
      _clearAllTimers();

      welcomeSysTimerRef.current = setInterval(function () {
        if (sysInflightRef.current || !mountedRef.current) return;
        sysInflightRef.current = true;
        fetchSystemStats().then(function (d) {
          if (!mountedRef.current || !d) return;
          _consecFails.current = 0;
          if (connLost) setConnLost(false);
          setSysStats(d);
        }).catch(function () {
          _consecFails.current++;
          if (_consecFails.current >= 3) setConnLost(true);
        }).finally(function () { sysInflightRef.current = false; });
      }, refreshMs);

      welcomeSummaryTimerRef.current = setInterval(function () {
        if (summaryInflightRef.current || !mountedRef.current) return;
        summaryInflightRef.current = true;
        fetchSummary().then(function (data) {
          if (!mountedRef.current || !data) return;
          _consecFails.current = 0;
          if (connLost) setConnLost(false);
          if (!_isFirstRun(data)) {
            setSummary(data);
            setActivity(data.activity || []);
            setPhase('main');
            _loadChartJs().then(function () {
              if (!mountedRef.current) return;
              _initCharts(data);
            });
            _startMainPolling();
          }
        }).catch(function () {
          _consecFails.current++;
          if (_consecFails.current >= 3) setConnLost(true);
        }).finally(function () { summaryInflightRef.current = false; });
      }, 30000);
    }

    function _loadISOData() {
      fetchISOCatalog().then(function (d) {
        if (!mountedRef.current || !d) return;
        setIsoData({ stats: d.stats || {}, items: d.items || [] });
      });
    }

    // Cleanup on unmount
    useEffect(function () {
      return function () {
        _clearAllTimers();
        if (cpuMemChartRef.current) cpuMemChartRef.current.destroy();
        if (netChartRef.current) netChartRef.current.destroy();
        if (diskChartRef.current) diskChartRef.current.destroy();
        if (crawlChartRef.current) crawlChartRef.current.destroy();
        if (queueChartRef.current) queueChartRef.current.destroy();
        cpuMemChartRef.current = null;
        netChartRef.current = null;
        diskChartRef.current = null;
        crawlChartRef.current = null;
        queueChartRef.current = null;
      };
    }, []);

    // ── Event handlers ──────────────────────────────────────────────────

    function handleRangeChange(e) {
      var v = e.target.value;
      setRange(v);
      _loadHistory();
    }

    function handleRefreshChange(e) {
      var v = parseInt(e.target.value) * 1000;
      setRefreshMs(v);
      // Restart polling with new interval
      if (phase === 'main') {
        _clearAllTimers();
        _startMainPolling();
      }
    }

    function handleTogglePause() {
      var newPaused = !paused;
      setPaused(newPaused);
      if (newPaused) {
        _clearAllTimers();
      } else {
        if (phase === 'main') _startMainPolling();
        else if (phase === 'welcome') _startWelcomePolling();
      }
    }

    function handleCrawlAll(e) {
      var btn = e.target;
      btn.disabled = true;
      Api.post('/admin/crawl/trigger-all', {}).then(function (r) {
        btn.disabled = false;
        if (r && r.success) Components.showToast(t('action_crawl_triggered'), 'success');
        else Components.showToast((r && r.message) || t('error'), 'error');
      });
    }

    function handleDownloadAllISO() {
      var btn = document.getElementById('iso-download-all-btn');
      if (btn) btn.disabled = true;
      Api.post('/admin/os-install/catalog/download-all', {}).then(function (r) {
        if (btn) btn.disabled = false;
        if (r && r.success) { Components.showToast(r.message || t('iso_queue_download_all'), 'success'); _loadISOData(); }
        else Components.showToast((r && r.message) || t('error'), 'error');
      });
    }

    async function handleRetry() {
      setPhase('loading');
      try {
        var data = await fetchSummary();
        if (!mountedRef.current) return;
        if (!data) { setPhase('error'); return; }
        var cfg = await fetchMonitorConfig();
        if (cfg) setMonitorCfg({ diskWarning: cfg.disk_warning_percent || 80, diskCritical: cfg.disk_critical_percent || 90 });
        setSummary(data);
        setActivity(data.activity || []);
        if (_isFirstRun(data)) {
          setPhase('welcome');
          _startWelcomePolling();
        } else {
          setPhase('main');
          await _loadChartJs();
          if (!mountedRef.current) return;
          _initCharts(data);
          _startMainPolling();
        }
      } catch (e) {
        if (mountedRef.current) setPhase('error');
      }
    }

    // ── Computed values ─────────────────────────────────────────────────

    var sys = (summary && summary.system) || {};
    var files = (summary && summary.files) || {};
    var deploy = (summary && summary.deploy) || {};
    var share = (summary && summary.share) || {};
    var diskPct = sys.disk_usage_percent || 0;
    var diskColor = diskPct > monitorCfg.diskCritical ? 'var(--color-error)' : (diskPct > monitorCfg.diskWarning ? 'var(--color-warning)' : 'var(--color-success)');
    var diskUsedStr = Helpers.formatBytes(sys.disk_used_bytes || 0);
    var diskTotalStr = Helpers.formatBytes(sys.disk_total_bytes || 0);
    var diskFreeStr = Helpers.formatBytes((sys.disk_total_bytes || 0) - (sys.disk_used_bytes || 0));

    // ── Loading skeleton ────────────────────────────────────────────────

    if (phase === 'loading') {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('dashboard') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <div class="flex items-center justify-between mb-6">
              <div>${Components.skeletonHeading && Components.skeletonHeading('200px')}</div>
            </div>
            <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
              ${[1,2,3,4].map(function () { return html`<div class="skeleton" style="height:6rem;border-radius:var(--radius-lg)"></div>`; })}
            </div>
            ${Components.skeletonText && Components.skeletonText(5)}
          </div>
        </div>`;
    }

    // ── Error state with retry ──────────────────────────────────────────

    if (phase === 'error') {
      return html`
        <div>
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('dashboard') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <div class="card" style="padding:3rem;text-align:center">
              <p class="text-sm" style="color:var(--color-text-tertiary)">${t('error_data_load')}</p>
              <button class="btn btn-primary" style="margin-top:1rem" onClick=${handleRetry}>${t('retry') || 'Retry'}</button>
            </div>
          </div>
        </div>`;
    }

    // ── Connection lost banner ──────────────────────────────────────────

    var connBanner = null;
    if (connLost) {
      connBanner = html`<div style="background:var(--color-error);color:var(--color-text-inverse);text-align:center;padding:0.5rem 1rem;font-size:0.875rem;font-weight:600;position:sticky;top:0;z-index:50">${t('connection_lost')}</div>`;
    }

    // ── Welcome (first-run) view ────────────────────────────────────────

    if (phase === 'welcome') {
      var wSys = sysStats || sys;
      var cards = [
        { icon: _qaIcons.files, iconClass: 'welcome-action-icon-files', title: t('cta_welcome_create_project'), desc: t('cta_welcome_create_project_desc'), route: '#/files' },
        { icon: _qaIcons.deploy, iconClass: 'welcome-action-icon-deploy', title: t('cta_welcome_deploy_config'), desc: t('cta_welcome_deploy_config_desc'), route: '#/deploy' },
        { icon: _qaIcons.share, iconClass: 'welcome-action-icon-share', title: t('cta_welcome_share_files'), desc: t('cta_welcome_share_files_desc'), route: '#/share' }
      ];
      if (sys.containers_enabled) {
        cards.push({ icon: _qaIcons.containers, iconClass: 'welcome-action-icon-containers', title: t('cta_welcome_containers'), desc: t('cta_welcome_containers_desc'), route: '#/containers' });
      }

      return html`
        <div>
          ${connBanner}
          <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('dashboard') }} />
          <div class="p-4 md:p-6 max-w-7xl mx-auto">
            <div class="dash-section">
              <div class="flex items-center justify-between mb-1">
                <div>
                  <p class="text-xs" style="color:var(--color-text-tertiary)">${t('dash_version')} ${Helpers.escapeHtml(sys.version || '')}
                    <span style="margin:0 0.5rem">\u00b7</span>${t('dash_uptime')} ${Helpers.escapeHtml(sys.uptime || '-')}
                  </p>
                </div>
                <div class="flex items-center gap-3">
                  <${MiniBar} pct=${wSys.cpu_usage_percent || 0} />
                  <${MiniBar} pct=${wSys.memory_usage_percent || 0} />
                  <${MiniBar} pct=${wSys.disk_usage_percent || 0} />
                </div>
              </div>
            </div>
            <div class="welcome-banner anim-fade-in">
              <h2>${Helpers.escapeHtml(t('welcome_banner_title'))}</h2>
              <p class="welcome-banner-subtitle">${Helpers.escapeHtml(t('welcome_banner_subtitle'))}</p>
              <div class="welcome-actions-grid">
                ${cards.map(function (c, i) {
                  return html`
                    <div class="welcome-action-card anim-fade-in anim-stagger-${i + 1}" onClick=${function () { window.location.hash = c.route; }}>
                      <div class="welcome-action-icon ${c.iconClass}" dangerouslySetInnerHTML=${{ __html: c.icon }} />
                      <h3>${Helpers.escapeHtml(c.title)}</h3>
                      <p>${Helpers.escapeHtml(c.desc)}</p>
                      <button class="btn btn-primary">${Helpers.escapeHtml(c.title)}</button>
                    </div>`;
                })}
              </div>
            </div>
          </div>
        </div>`;
    }

    // ── Main dashboard view ─────────────────────────────────────────────

    var mSys = sysStats || sys;
    var cColors = _cc();
    var qTotal = queueStats.pending + queueStats.downloading + queueStats.complete + queueStats.error;
    var isoItems = (isoData.items || []).filter(function (x) { return x.download_status === 'pending' || x.download_status === 'downloading'; });

    return html`
      <div>
        ${connBanner}
        <div dangerouslySetInnerHTML=${{ __html: SystemStatus._nav('dashboard') }} />
        <div class="p-4 md:p-6 max-w-7xl mx-auto">

          <!-- Header with version, uptime, mini bars -->
          <div class="dash-section">
            <div class="flex items-center justify-between mb-1">
              <div>
                <h1 class="text-xl font-bold tracking-tight" style="color:var(--color-text)">${t('dash_welcome')}</h1>
                <p class="text-xs mt-1" style="color:var(--color-text-tertiary)">
                  <span>${t('dash_version')} ${Helpers.escapeHtml(sys.version||'')}</span>
                  <span style="margin:0 0.5rem">&middot;</span>
                  <span>${t('dash_uptime')} ${Helpers.escapeHtml(sys.uptime||'-')}</span>
                </p>
              </div>
              <div class="flex items-center gap-3">
                <${MiniBar} pct=${mSys.cpu_usage_percent || 0} />
                <${MiniBar} pct=${mSys.memory_usage_percent || 0} />
                <${MiniBar} pct=${mSys.disk_usage_percent || 0} />
              </div>
            </div>
          </div>

          <!-- Status cards grid -->
          <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6 dash-section">
            <${StatusCard} title="dash_files" accent="stat-accent-blue" href="#/files" stats=${[
              { l: t('dash_projects'), v: files.project_count || 0 },
              { l: t('dash_total_files'), v: (files.total_files || 0).toLocaleString() },
              { l: t('queue_downloading'), v: files.queue_downloading || 0 }
            ]} />
            <${StatusCard} title="dash_deploy" accent="stat-accent-emerald" href="#/deploy" stats=${[
              { l: t('dash_configs'), v: deploy.config_count || 0 },
              { l: t('dash_isos'), v: deploy.iso_count || 0 },
              { l: t('iso_queue_pending'), v: deploy.iso_pending || 0 }
            ]} />
            <${StatusCard} title="dash_share" accent="stat-accent-amber" href="#/share" stats=${[
              { l: t('dash_shared_files'), v: share.file_count || 0 },
              { l: t('disk_usage'), v: share.total_size || '0 B' }
            ]} />
            <${StatusCard} title="dash_system" accent="stat-accent-violet" stats=${[
              { l: 'CPU', v: (mSys.cpu_usage_percent || 0).toFixed(1) + '%' },
              { l: t('memory_usage'), v: (mSys.memory_usage_percent || 0).toFixed(1) + '%' },
              { l: t('dash_disk_usage'), v: (mSys.disk_usage_percent || 0).toFixed(1) + '%' }
            ]} />
            ${sys.containers_enabled ? html`
              <${StatusCard} title="dash_containers" accent="stat-accent-blue" href="#/containers" stats=${[
                { l: t('container_count'), v: sys.container_count || 0 },
                { l: t('container_running'), v: sys.container_running || 0 }
              ]} />
            ` : null}
          </div>

          <!-- Monitor controls -->
          <div class="dash-section">
            <div class="monitor-controls anim-fade-in">
              <div class="flex items-center gap-3">
                <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('monitor_time_range')}</label>
                <select value=${range} onChange=${handleRangeChange} class="lang-select">
                  <option value="1h">${t('range_1h')}</option>
                  <option value="6h">${t('range_6h')}</option>
                  <option value="12h">${t('range_12h')}</option>
                  <option value="24h">${t('range_24h')}</option>
                  <option value="3d">${t('range_3d')}</option>
                  <option value="7d">${t('range_7d')}</option>
                  <option value="30d">${t('range_30d')}</option>
                </select>
                <label class="text-xs font-medium" style="color:var(--color-text-secondary)">${t('monitor_refresh')}</label>
                <select value=${String(refreshMs / 1000)} onChange=${handleRefreshChange} class="lang-select">
                  <option value="3">${t('refresh_3s')}</option>
                  <option value="5">${t('refresh_5s')}</option>
                  <option value="10">${t('refresh_10s')}</option>
                  <option value="30">${t('refresh_30s')}</option>
                  <option value="60">${t('refresh_60s')}</option>
                </select>
                <button class="btn btn-ghost text-xs" onClick=${handleTogglePause}>
                  ${paused ? t('chart_resume') : t('chart_pause')}
                </button>
                ${paused ? html`
                  <span class="text-xs font-semibold px-2 py-0.5 rounded-full" style="background:var(--color-warning);color:var(--color-text-inverse)">${t('chart_paused')}</span>
                ` : null}
                <span class="text-xs" style="color:var(--color-text-tertiary);margin-left:auto">${t('chart_updated', { time: updatedTime })}</span>
              </div>
            </div>

            <!-- CPU/Mem + Network charts -->
            <div class="monitor-merged-chart anim-fade-in">
              <div class="monitor-card">
                <h3 class="chart-title">${t('monitor_cpu_memory')}</h3>
                <div class="chart-container">
                  <canvas ref=${cpuMemCanvasRef} id="cpu-mem-chart"></canvas>
                </div>
              </div>
              <div class="monitor-card">
                <h3 class="chart-title">${t('monitor_network')}</h3>
                <div class="chart-container">
                  <canvas ref=${netCanvasRef} id="network-chart"></canvas>
                </div>
              </div>
            </div>
          </div>

          <!-- Disk gauge -->
          <div class="dash-section">
            <div class="card anim-fade-in" style="padding:1.25rem">
              <h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('monitor_disk_capacity')}</h2>
              <div class="flex items-center gap-6 flex-wrap">
                <div style="width:160px;height:160px;position:relative;flex-shrink:0">
                  <canvas ref=${diskCanvasRef} id="disk-chart"></canvas>
                  <div style="position:absolute;inset:0;display:flex;flex-direction:column;align-items:center;justify-content:center">
                    <span class="text-2xl font-bold" style=${{ color: diskColor }}>${diskPct.toFixed(1)}%</span>
                    <span class="text-xs" style="color:var(--color-text-tertiary)">${t('dash_disk_usage')}</span>
                  </div>
                </div>
                <div class="disk-gauge" style="flex:1;min-width:200px">
                  <div class="text-sm" style="color:var(--color-text-secondary)">${t('dash_disk_of_total', { used: diskUsedStr, total: diskTotalStr })}</div>
                  <div class="text-xs mt-1" style="color:var(--color-text-tertiary)">${t('dash_disk_free', { free: diskFreeStr })}</div>
                  <div class="mt-3" style="height:8px;background:var(--color-bg-tertiary);border-radius:4px;overflow:hidden;position:relative">
                    <div style=${{ height:'100%', borderRadius:'4px', transition:'width 0.5s ease', background: diskColor, width: diskPct + '%' }}></div>
                    ${monitorCfg.diskWarning < 100 ? html`<div style="position:absolute;top:0;bottom:0;left:${monitorCfg.diskWarning}%;width:1px;background:var(--color-warning);opacity:0.5"></div>` : null}
                    ${monitorCfg.diskCritical < 100 ? html`<div style="position:absolute;top:0;bottom:0;left:${monitorCfg.diskCritical}%;width:1px;background:var(--color-error);opacity:0.5"></div>` : null}
                  </div>
                  <div class="flex justify-between mt-1">
                    <span class="text-xs" style="color:var(--color-warning)">${monitorCfg.diskWarning}%</span>
                    <span class="text-xs" style="color:var(--color-error)">${monitorCfg.diskCritical}%</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Activity timeline + Quick actions -->
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6 dash-section">
            <!-- Activity -->
            <div class="card anim-fade-in" style="padding:1.25rem">
              <h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('dash_activity_title')}</h2>
              <div class="activity-timeline" aria-live="polite">
                ${activity.length === 0 ? html`
                  <div style="text-align:center;padding:1.5rem 0.5rem">
                    <p class="text-sm" style="color:var(--color-text-tertiary)">${t('activity_empty')}</p>
                  </div>
                ` : activity.map(function (evt) {
                  return html`<${ActivityItem} key=${evt.id} evt=${evt} />`;
                })}
              </div>
            </div>

            <!-- Quick actions + System overview -->
            <div class="card anim-fade-in" style="padding:1.25rem">
              <h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('dash_quick_actions')}</h2>
              <div class="quick-actions">
                <button class="btn btn-primary" onClick=${handleCrawlAll}>${t('action_crawl_all')}</button>
                <button class="btn btn-secondary" onClick=${function () { window.location.hash = '#/files'; }}>${t('action_view_queue')}</button>
                <button class="btn btn-secondary" onClick=${function () { window.location.hash = '#/deploy'; }}>${t('action_download_iso')}</button>
              </div>
              <div class="mt-6">
                <h3 class="text-sm font-semibold mb-3" style="color:var(--color-text)">${t('dash_system_overview')}</h3>
                <div class="grid grid-cols-2 gap-3">
                  <${SysStat} label="CPU" value=${(mSys.cpu_usage_percent||0).toFixed(1) + '%'} />
                  <${SysStat} label=${t('memory_usage')} value=${(mSys.memory_usage_percent||0).toFixed(1) + '%'} />
                  <${SysStat} label=${t('dash_disk_usage')} value=${(mSys.disk_usage_percent||0).toFixed(1) + '%'} />
                  <${SysStat} label=${t('dash_uptime')} value=${sys.uptime || '-'} />
                </div>
              </div>
            </div>
          </div>

          <!-- Crawl activity chart -->
          <div class="card mb-6 anim-fade-in dash-section" style="padding:1.25rem">
            <h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('chart_crawl_activity')}</h2>
            <div class="chart-container">
              <canvas ref=${crawlCanvasRef} id="crawl-chart"></canvas>
            </div>
          </div>

          <!-- Queue overview -->
          <div class="queue-overview-grid anim-fade-in dash-section">
            <div class="queue-chart-section">
              <h3 class="chart-title">${t('chart_queue_overview')}</h3>
              <div class="chart-container" style="height:250px">
                <canvas ref=${queueCanvasRef} id="queue-chart"></canvas>
              </div>
              <div class="queue-legend" aria-live="polite">
                ${_qli(cColors.w, t('queue_pending'), queueStats.pending)}
                ${_qli(cColors.p, t('queue_downloading'), queueStats.downloading)}
                ${_qli(cColors.s, t('queue_complete'), queueStats.complete)}
                ${_qli(cColors.e, t('queue_failed'), queueStats.error)}
                <div class="queue-stat-item" style="border-top:1px solid var(--color-border);padding-top:0.5rem;margin-top:0.25rem;font-weight:600">
                  <span>${t('queue_total')}</span>
                  <span class="queue-stat-count">${qTotal}</span>
                </div>
              </div>
            </div>
            <div class="queue-list-section">
              <h3 class="chart-title">${t('download_queue')}</h3>
              <div aria-live="polite">
                ${queueFiles.length === 0 ? html`
                  <p class="text-sm" style="color:var(--color-text-tertiary);padding:1rem">${t('queue_empty')}</p>
                ` : queueFiles.map(function (f) {
                  return html`<${QueueItem} key=${f.id} item=${f} />`;
                })}
              </div>
            </div>
          </div>

          <!-- ISO queue -->
          <div class="card mb-6 anim-fade-in dash-section" style="padding:1.25rem;margin-top:1.5rem">
            <div class="flex items-center justify-between mb-4">
              <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('iso_queue_title')}</h2>
              <button id="iso-download-all-btn" class="btn btn-primary text-xs" onClick=${handleDownloadAllISO}>${t('iso_queue_download_all')}</button>
            </div>
            <div id="iso-queue-content">
              <!-- ISO stats bar -->
              <div class="flex flex-wrap gap-3 mb-4">
                <div class="iso-queue-stat">
                  <span class="iso-queue-dot" style=${{ background: cColors.w }}></span>
                  ${t('iso_queue_pending')}: <span>${isoData.stats.pending || 0}</span>
                </div>
                <div class="iso-queue-stat">
                  <span class="iso-queue-dot" style=${{ background: cColors.p }}></span>
                  ${t('iso_queue_downloading')}: <span>${isoData.stats.downloading || 0}</span>
                </div>
                <div class="iso-queue-stat">
                  <span class="iso-queue-dot" style=${{ background: cColors.s }}></span>
                  ${t('iso_queue_downloaded')}: <span>${isoData.stats.downloaded || 0}</span>
                </div>
                <div class="iso-queue-stat">
                  <span class="iso-queue-dot" style=${{ background: cColors.e }}></span>
                  ${t('iso_queue_error')}: <span>${isoData.stats.error || 0}</span>
                </div>
              </div>
              ${isoItems.length > 0 ? isoItems.map(function (it) {
                return html`<${ISOQueueItem} key=${it.id} item=${it} />`;
              }) : (isoData.stats.total === 0 ? html`
                <p class="text-sm" style="color:var(--color-text-tertiary)">${t('iso_queue_empty')}</p>
              ` : null)}
            </div>
          </div>

        </div>
      </div>`;
  }

  // ── Public API ───────────────────────────────────────────────────────

  function renderFn() {
    var app = document.getElementById('main-content');
    if (!app) return;
    pRender(html`<${DashboardComponent} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) pRender(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();
