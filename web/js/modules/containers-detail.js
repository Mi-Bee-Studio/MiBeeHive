const ContainersDetail = (function() {
  'use strict';

  // Private state
  var _chart = null;         // Chart.js instance for CPU/Memory
  var _containerId = '';     // container ID extracted from URL hash
  var _logCount = 0;         // number of log lines currently displayed

  // Constants
  var MAX_LOG_LINES = 200;   // max log lines to display
  var MAX_CHART_POINTS = 60; // max data points on resource chart

  // Reusable style strings (CSS variable references)
  var _styleText = 'color:var(--color-text)';
  var _styleTextTertiary = 'color:var(--color-text-tertiary)';
  var _styleTextSecondary = 'color:var(--color-text-secondary)';

  // Parse any CSS color string (hex, rgb) to #rrggbb hex
  function _parseColor(color) {
    if (!color) return getComputedStyle(document.documentElement).getPropertyValue('--color-chart-fallback').trim() || '#6b7280';
    color = color.trim();
    if (color[0] === '#') {
      return color.length === 4
        ? '#' + color[1] + color[1] + color[2] + color[2] + color[3] + color[3]
        : color.slice(0, 7);
    }
    var match = color.match(/(\d+)\s*,\s*(\d+)\s*,\s*(\d+)/);
    return match
      ? '#' + ((1 << 24) + (+match[1] << 16) + (+match[2] << 8) + (+match[3])).toString(16).slice(1)
      : getComputedStyle(document.documentElement).getPropertyValue('--color-chart-fallback').trim() || '#6b7280';
  }

  // Convert #rrggbb hex + alpha to rgba() string
  function _withAlpha(hex, alpha) {
    return 'rgba(' +
      parseInt(hex.slice(1, 3), 16) + ',' +
      parseInt(hex.slice(3, 5), 16) + ',' +
      parseInt(hex.slice(5, 7), 16) + ',' +
      alpha + ')';
  }

  // Read a CSS variable value from :root
  function _getCssVar(name) {
    return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  }

  // HTML-escape shorthand
  function _esc(str) {
    return Helpers.escapeHtml(str);
  }

  // Set textContent of an element by ID
  function _setText(id, value) {
    var el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  // Extract data from standard API response {success, data}
  function _data(response) {
    return response && response.success ? response.data : null;
  }

  // Format current time as HH:MM:SS for chart labels
  function _formatTime() {
    var d = new Date();
    return d.getHours() + ':' + String(d.getMinutes()).padStart(2, '0') + ':' + String(d.getSeconds()).padStart(2, '0');
  }

  // Clear all tracked intervals
  function _clearTimers() {
    App.clearAllTimers();
  }

  // Start an interval and track it for cleanup
  function _startInterval(fn, ms) {
    var id = setInterval(fn, ms);
    App.addTimer(id);
    return id;
  }

  // Map container status to status-dot CSS class
  function _statusDot(status) {
    return status === 'running' ? 'status-dot-success'
      : (status === 'paused' || status === 'exited') ? 'status-dot-warning'
      : 'status-dot-error';
  }

  // Generate a label+value info row HTML for the info card
  function _infoRow(label, id) {
    return '<div class="flex flex-col gap-0.5 py-2" style="border-bottom:1px solid var(--color-border-subtle)">' +
      '<span class="text-xs font-medium" style="' + _styleTextTertiary + '">' + label + '</span>' +
      '<span id="' + id + '" class="text-sm" style="' + _styleText + ';word-break:break-all">-</span>' +
      '</div>';
  }

  // Load container info (name, status, image, ports, env, volumes)
  async function _loadInfo() {
    var response = await Api.get('/admin/containers/' + _containerId);
    var data = _data(response);
    if (!data) return;

    // Name
    _setText('cd-name', data.name || _containerId);

    // Status with colored dot
    var statusEl = document.getElementById('cd-status');
    var status = data.status;
    if (statusEl) {
      statusEl.innerHTML = '<span class="status-dot ' + _statusDot(status) + '"></span>' + _esc(status);
      statusEl.style.cssText = status === 'running' ? 'color:var(--color-success)'
        : (status === 'paused' || status === 'exited') ? 'color:var(--color-warning)'
        : 'color:var(--color-error)';
    }

    // Detail fields
    _setText('cd-image', data.image || '-');
    _setText('cd-created', data.created ? Helpers.formatTime(data.created) : '-');

    var portsEl = document.getElementById('cd-ports');
    if (portsEl) portsEl.textContent = data.ports && data.ports.length ? data.ports.join(', ') : '-';

    var envEl = document.getElementById('cd-env');
    if (envEl) envEl.innerHTML = data.env && data.env.length ? data.env.map(_esc).join('<br>') : '-';

    var volsEl = document.getElementById('cd-vols');
    if (volsEl) volsEl.innerHTML = data.volumes && data.volumes.length ? data.volumes.map(_esc).join('<br>') : '-';
  }

  // Load and display container logs (tail 100, auto-scroll)
  async function _loadLogs() {
    var response = await Api.get('/admin/containers/' + _containerId + '/logs?tail=100', { silent: true });
    var data = _data(response);
    var el = document.getElementById('cd-logs');
    if (!el) return;

    if (!data || !data.length) {
      if (_logCount === 0) el.textContent = 'No logs';
      return;
    }

    var lines = data.map(function(entry) { return _esc(entry.content || ''); });
    if (lines.length > MAX_LOG_LINES) lines = lines.slice(lines.length - MAX_LOG_LINES);
    el.textContent = lines.join('\n');
    _logCount = lines.length;
    el.scrollTop = el.scrollHeight;
  }

  // Initialize the CPU/Memory resource chart (Chart.js)
  function _initChart() {
    var canvas = document.getElementById('cd-stats-chart');
    if (!canvas || !window.Chart) return;

    var primaryColor = _parseColor(_getCssVar('--color-primary'));
    var successColor = _parseColor(_getCssVar('--color-success'));
    var borderColor = _parseColor(_getCssVar('--color-border'));
    var textColor = _parseColor(_getCssVar('--color-text-tertiary'));

    _chart = new Chart(canvas, {
      type: 'line',
      data: {
        labels: [],
        datasets: [
          {
            label: 'CPU %',
            data: [],
            borderColor: primaryColor,
            backgroundColor: _withAlpha(primaryColor, 0.08),
            fill: true,
            tension: 0.4,
            borderWidth: 2,
            pointRadius: 0
          },
          {
            label: 'Memory MB',
            data: [],
            borderColor: successColor,
            backgroundColor: _withAlpha(successColor, 0.08),
            fill: true,
            tension: 0.4,
            borderWidth: 2,
            pointRadius: 0
          }
        ]
      },
      options: {
        responsive: true,
        maintainAspectRatio: true,
        scales: {
          x: { display: false },
          y: {
            beginAtZero: true,
            grid: { color: _withAlpha(borderColor, 0.04) },
            ticks: { color: textColor }
          }
        },
        plugins: {
          legend: { labels: { color: textColor } },
          tooltip: { backgroundColor: 'rgba(0,0,0,0.78)', padding: 8 }
        }
      }
    });
  }

  // Poll container stats and update chart + text displays
  async function _pollStats() {
    var response = await Api.get('/admin/containers/' + _containerId + '/stats', { silent: true });
    var data = _data(response);
    if (!data || !_chart) return;

    // Push new data point
    _chart.data.labels.push(_formatTime());
    _chart.data.datasets[0].data.push(data.cpu_usage_percent || 0);
    _chart.data.datasets[1].data.push(data.memory_usage_mb || 0);

    // Trim to max points
    if (_chart.data.labels.length > MAX_CHART_POINTS) {
      _chart.data.labels.shift();
      _chart.data.datasets[0].data.shift();
      _chart.data.datasets[1].data.shift();
    }

    _chart.update('none');

    // Update numeric displays
    _setText('cd-cpu', (data.cpu_usage_percent || 0).toFixed(1) + '%');
    _setText('cd-mem', (data.memory_usage_mb || 0).toFixed(1) + '/' + (data.memory_limit_mb || 0).toFixed(0) + 'MB');
  }

  // Render the container detail page
  function render() {
    destroy();

    // Extract container ID from URL hash: #/containers/{id}
    var match = (window.location.hash || '').match(/^#\/containers\/([^\/\?]+)/);
    _containerId = match ? match[1] : '';
    if (!_containerId) return;

    var mc = document.getElementById('main-content');
    if (!mc) return;

    mc.innerHTML =
      '<div class="p-4 md:p-6 max-w-7xl mx-auto">' +
        // Header: back button, container name, status badge
        '<div class="flex items-center gap-3 mb-6">' +
          '<a href="#/containers" class="btn btn-ghost text-sm" style="' + _styleTextSecondary + '">' +
            Helpers.ICONS.arrowLeft + t('back') +
          '</a>' +
          '<h1 id="cd-name" class="text-xl font-bold" style="' + _styleText + '">' + _esc(_containerId) + '</h1>' +
          '<span id="cd-status"></span>' +
        '</div>' +

        // Two-column layout: Info card + Resources card
        '<div class="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">' +

          // Info card
          '<div class="card" style="padding:1.25rem">' +
            '<h2 class="text-sm font-semibold mb-3" style="' + _styleText + '">Info</h2>' +
            _infoRow('Image', 'cd-image') +
            _infoRow('Created', 'cd-created') +
            _infoRow('Ports', 'cd-ports') +
            _infoRow('Env', 'cd-env') +
            _infoRow('Volumes', 'cd-vols') +
          '</div>' +

          // Resources card: CPU/Mem numbers + chart
          '<div class="card" style="padding:1.25rem">' +
            '<h2 class="text-sm font-semibold mb-3" style="' + _styleText + '">Resources</h2>' +
            '<div class="grid grid-cols-2 gap-4 mb-4">' +
              '<div>' +
                '<span class="text-xs" style="' + _styleTextTertiary + '">CPU</span>' +
                '<div id="cd-cpu" class="text-lg font-semibold" style="' + _styleText + '">-</div>' +
              '</div>' +
              '<div>' +
                '<span class="text-xs" style="' + _styleTextTertiary + '">Memory</span>' +
                '<div id="cd-mem" class="text-lg font-semibold" style="' + _styleText + '">-</div>' +
              '</div>' +
            '</div>' +
            '<canvas id="cd-stats-chart"></canvas>' +
          '</div>' +

        '</div>' +

        // Logs card
        '<div class="card" style="padding:1.25rem">' +
          '<h2 class="text-sm font-semibold mb-3" style="' + _styleText + '">Logs</h2>' +
          '<pre id="cd-logs" style="background:var(--color-bg-tertiary);' + _styleTextSecondary + ';padding:1rem;max-height:24rem;overflow-y:auto;margin:0">Loading...</pre>' +
        '</div>' +
      '</div>';

    // Load initial data
    _loadInfo();
    _loadLogs();

    // Lazy-load Chart.js if needed, then init chart and start polling
    if (!window.Chart) {
      var script = document.createElement('script');
      script.src = 'https://cdn.jsdelivr.net/npm/chart.js@4/dist/chart.umd.min.js';
      script.onload = function() { _initChart(); _pollStats(); };
      document.head.appendChild(script);
    } else {
      _initChart();
      _pollStats();
    }

    // Start periodic polling: logs every 5s, stats every 10s
    _startInterval(_loadLogs, 5000);
    _startInterval(_pollStats, 10000);
  }

  // Cleanup: clear timers, destroy chart, reset state
  function destroy() {
    App.clearAllTimers();
    _chart = null;
    _containerId = '';
    _logCount = 0;
  }

  return { render: render, destroy: destroy };
})();
