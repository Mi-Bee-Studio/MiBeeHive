const Helpers = (function () {
  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
  }

  function formatTime(iso) {
    if (!iso) return typeof t === 'function' ? t('never') : 'never';
    try { return new Date(iso).toLocaleString(); }
    catch (e) { return iso; }
  }

  function escapeHtml(s) {
    if (!s) return '';
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function renderBreadcrumb(items) {
    return '<nav class="breadcrumb" aria-label="Breadcrumb">' +
      items.map(function (item, i) {
        const isLast = i === items.length - 1;
        if (isLast) {
          return '<span class="breadcrumb-item active" aria-current="page">' + item.label + '</span>';
        }
        return '<a href="' + item.href + '" class="breadcrumb-item">' + item.label + '</a>' +
          '<span class="breadcrumb-sep">/</span>';
      }).join('') + '</nav>';
  }

  function sourceTypeBadge(type) {
    const cls = {
      github: 'badge-blue',
      go: 'badge-cyan',
      hashicorp: 'badge-purple',
      grafana: 'badge-orange',
    };
    return '<span class="badge ' + (cls[type] || 'badge-default') + '">' + escapeHtml(type) + '</span>';
  }

  function statusBadge(status) {
    var dotCls = {
      complete: 'status-dot-success',
      downloaded: 'status-dot-success',
      imported: 'status-dot-success',
      downloading: 'status-dot-warning',
      pending: 'status-dot-neutral',
      error: 'status-dot-error',
    };
    var colorCls = {
      complete: 'color:var(--color-success)',
      downloaded: 'color:var(--color-success)',
      imported: 'color:var(--color-success)',
      downloading: 'color:var(--color-warning)',
      pending: 'color:var(--color-text-tertiary)',
      error: 'color:var(--color-error)',
    };
    var iconMap = {
      complete: 'statusSuccess',
      downloaded: 'statusSuccess',
      imported: 'statusSuccess',
      downloading: 'statusDownloading',
      pending: 'statusPending',
      error: 'statusError',
    };
    var iconKey = iconMap[status] || 'statusPending';
    // Translate the status label via i18n (status_<value>); fall back to the
    // raw value if no translation exists. Escape the translated text since this
    // HTML string is injected via dangerouslySetInnerHTML.
    var label = (typeof t === 'function' && t('status_' + status) !== 'status_' + status)
      ? t('status_' + status)
      : status;
    return '<span class="inline-flex items-center" style="' + (colorCls[status] || 'color:var(--color-text-tertiary)') + '">' +
      '<span class="status-dot ' + (dotCls[status] || 'status-dot-neutral') + '"></span>' +
      ICONS[iconKey] +
      '<span>' + escapeHtml(label) + '</span></span>';
  }

  function loadingSpinner(size) {
    size = size || 'lg';
    var cls = 'spinner' + (size === 'lg' ? ' spinner-lg' : '');
    return '<div class="flex items-center justify-center py-20">' +
      '<div class="' + cls + '"></div>' +
      '</div>';
  }

  function errorMessage(msg) {
    return '<div class="anim-fade-in" style="padding:1rem;background:var(--color-error-light);border:1px solid var(--color-error);border-radius:var(--radius-lg);color:var(--color-error);font-size:0.875rem">' +
      (typeof t === 'function' ? t('error') : 'Error') + ': ' + escapeHtml(msg) + '</div>';
  }

  function badge(type, label) {
    var cls = {
      success: 'badge-success',
      warning: 'badge-warning',
      error: 'badge-error',
      info: 'badge-default',
      blue: 'badge-blue',
      gray: 'badge-default'
    };
    return '<span class="badge ' + (cls[type] || 'badge-default') + '">' + escapeHtml(label) + '</span>';
  }

  function debounce(fn, ms) {
    ms = ms || 300;
    let timer;
    return function () {
      const args = arguments;
      const ctx = this;
      clearTimeout(timer);
      timer = setTimeout(function () { fn.apply(ctx, args); }, ms);
    };
  }

  function copyToClipboard(text) {
    if (navigator.clipboard) {
      return navigator.clipboard.writeText(text);
    }
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    return Promise.resolve();
  }
  function validateIP(ip) {
    return /^(\d{1,3}\.){3}\d{1,3}$/.test(ip) && ip.split('.').every(function (o) { var n = parseInt(o, 10); return n >= 0 && n <= 255; });
  }

  function validateNetmask(nm) {
    if (!validateIP(nm)) return false;
    var parts = nm.split('.').map(function (o) { return parseInt(o, 10); });
    var bits = (parts[0] << 24 | parts[1] << 16 | parts[2] << 8 | parts[3]) >>> 0;
    var mask = 0;
    for (var i = 31; i >= 0; i--) {
      if ((bits & (1 << i)) >>> 0) mask |= (1 << i) >>> 0; else break;
    }
    return bits === mask;
  }

  function validateURL(url) {
    try { var u = new URL(url); return u.protocol === 'http:' || u.protocol === 'https:'; }
    catch (e) { return false; }
  }

  function slugify(str) {
    if (!str) return '';
    return str.toLowerCase().replace(/[\s\-]+/g, '-').replace(/[^a-z0-9\-]/g, '').replace(/\-+/g, '-').replace(/^-|-$/g, '');
  }

  const ICONS = {
    folder: '<svg aria-hidden="true" class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12.75V12A2.25 2.25 0 014.5 9.75h15A2.25 2.25 0 0121.75 12v.75m-8.69-6.44l-2.12-2.12a1.5 1.5 0 00-1.061-.44H4.5A2.25 2.25 0 002.25 6v12a2.25 2.25 0 002.25 2.25h15A2.25 2.25 0 0021.75 18V9a2.25 2.25 0 00-2.25-2.25h-5.379a1.5 1.5 0 01-1.06-.44z"/></svg>',
    file: '<svg aria-hidden="true" class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"/></svg>',
    disk: '<svg aria-hidden="true" class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M20.25 6.375c0 2.278-3.694 4.125-8.25 4.125S3.75 8.653 3.75 6.375m16.5 0c0-2.278-3.694-4.125-8.25-4.125S3.75 4.097 3.75 6.375m16.5 0v11.25c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125V6.375m16.5 0v3.75m-16.5-3.75v3.75m16.5 0v3.75C20.25 16.153 16.556 18 12 18s-8.25-1.847-8.25-4.125v-3.75m16.5 0c0 2.278-3.694 4.125-8.25 4.125s-8.25-1.847-8.25-4.125"/></svg>',
    clock: '<svg aria-hidden="true" class="w-5 h-5" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>',
    search: '<svg aria-hidden="true" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>',
    arrowLeft: '<svg aria-hidden="true" class="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M15 19l-7-7 7-7"/></svg>',
    download: '<svg aria-hidden="true" class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>',
    link: '<svg aria-hidden="true" class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M13.19 8.688a4.5 4.5 0 011.242 7.244l-4.5 4.5a4.5 4.5 0 01-6.364-6.364l1.757-1.757m13.35-.622l1.757-1.757a4.5 4.5 0 00-6.364-6.364l-4.5 4.5a4.5 4.5 0 001.242 7.244"/></svg>',
    eye: '<svg aria-hidden="true" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>',
    eyeOff: '<svg aria-hidden="true" class="w-4 h-4 hidden" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>',
    inbox: '<svg aria-hidden="true" class="w-8 h-8" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" d="M2.25 13.5h3.86a2.25 2.25 0 012.012 1.244l.256.512a2.25 2.25 0 002.013 1.244h3.218a2.25 2.25 0 002.013-1.244l.256-.512a2.25 2.25 0 012.013-1.244h3.859m-16.5 0V6.75A2.25 2.25 0 015.25 4.5h13.5A2.25 2.25 0 0121 6.75v6.75m-16.5 0v3.75A2.25 2.25 0 005.25 18h13.5A2.25 2.25 0 0021 16.5v-3.75"/></svg>',
    statusSuccess: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><path stroke-linecap="round" stroke-linejoin="round" d="M4 8.5l2.5 2.5L12 5.5"/></svg>',
    statusError: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6.5"/><path stroke-linecap="round" d="M5.5 5.5l5 5m0-5l-5 5"/></svg>',
    statusWarning: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><path stroke-linecap="round" stroke-linejoin="round" d="M8 4.5v4M8 11v.5"/></svg>',
    statusPending: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6.5"/><path stroke-linecap="round" d="M8 4.5V8l2.5 1.5"/></svg>',
    statusRunning: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><path stroke-linecap="round" stroke-linejoin="round" d="M4 4l4 4-4 4M8.5 4l4 4-4 4"/></svg>',
    statusIdle: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><rect x="3.5" y="3.5" width="9" height="9" rx="1"/></svg>',
    statusPaused: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><rect x="4" y="3" width="2.5" height="10" rx="0.5"/><rect x="9.5" y="3" width="2.5" height="10" rx="0.5"/></svg>',
    statusDownloading: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><path stroke-linecap="round" stroke-linejoin="round" d="M8 2v8m0 0l-3-3m3 3l3-3M3 12h10"/></svg>',
    statusCrawl: '<svg aria-hidden="true" class="status-icon" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 16 16"><path stroke-linecap="round" stroke-linejoin="round" d="M2.5 8a5.5 5.5 0 019.3-3.95M13.5 8a5.5 5.5 0 01-9.3 3.95"/><path stroke-linecap="round" d="M12 1.5v2.5h2.5M4 12v2.5H1.5"/></svg>',
  };

  return {
    formatBytes: formatBytes,
    formatTime: formatTime,
    escapeHtml: escapeHtml,
    renderBreadcrumb: renderBreadcrumb,
    sourceTypeBadge: sourceTypeBadge,
    statusBadge: statusBadge,
    loadingSpinner: loadingSpinner,
    errorMessage: errorMessage,
    badge: badge,
    debounce: debounce,
    copyToClipboard: copyToClipboard,
    validateIP: validateIP,
    validateNetmask: validateNetmask,
    validateURL: validateURL,
    slugify: slugify,
    ICONS: ICONS,
  };
})();
