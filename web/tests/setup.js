/**
 * Vitest setup — loads vendor UMD scripts and mocks global dependencies.
 *
 * Loading order mirrors web/index.html:
 *   preact.min.js → preact-hooks.min.js → htm.min.js → preact.js (bridge)
 */
const fs = require('fs');
const path = require('path');

// ---------------------------------------------------------------------------
// 1. Load vendor UMD scripts into jsdom global scope
// ---------------------------------------------------------------------------
const vendorDir = path.resolve(__dirname, '..', 'vendor');
const coreDir = path.resolve(__dirname, '..', 'js', 'core');

function evalFile(filePath) {
  const code = fs.readFileSync(filePath, 'utf8');
  // Use Function constructor so strict UMD scripts get `this` === globalThis
  const fn = new Function(code);
  fn.call(globalThis);
}

// Preact core → window.preact
evalFile(path.join(vendorDir, 'preact.min.js'));

// Preact hooks → window.preactHooks
evalFile(path.join(vendorDir, 'preact-hooks.min.js'));

// HTM → window.htm
evalFile(path.join(vendorDir, 'htm.min.js'));

// PreactBridge → window.PreactBridge
evalFile(path.join(coreDir, 'preact.js'));

// ---------------------------------------------------------------------------
// 2. Mock global dependencies used by all modules
// ---------------------------------------------------------------------------

// Api — HTTP client (api.js)
globalThis.Api = {
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
  patch: vi.fn(),
  getWithHeaders: vi.fn(),
};

// Auth — authentication module (auth.js)
globalThis.Auth = {
  getToken: vi.fn(() => null),
  isLoggedIn: vi.fn(() => false),
  login: vi.fn(),
  logout: vi.fn(),
  refreshToken: vi.fn(),
};

// Helpers — utility functions (helpers.js)
globalThis.Helpers = {
  formatBytes: vi.fn((b) => b + ' B'),
  formatTime: vi.fn((iso) => iso || 'never'),
  escapeHtml: vi.fn((s) => {
    if (!s) return '';
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }),
  renderBreadcrumb: vi.fn(() => ''),
  sourceTypeBadge: vi.fn(() => ''),
  statusBadge: vi.fn(() => ''),
  loadingSpinner: vi.fn(() => ''),
  errorMessage: vi.fn((msg) => 'Error: ' + msg),
  badge: vi.fn(() => ''),
  debounce: vi.fn((fn) => fn),
  copyToClipboard: vi.fn(() => Promise.resolve()),
  validateIP: vi.fn(() => true),
  validateNetmask: vi.fn(() => true),
  validateURL: vi.fn(() => true),
  ICONS: { inbox: '<svg class="icon-inbox"></svg>' },
  slugify: vi.fn((s) => (s || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, ''))
};
// Cache — in-memory cache singleton (cache.js)
globalThis.Cache = {
  get: vi.fn(() => null),
  set: vi.fn(),
  invalidate: vi.fn(),
  invalidatePattern: vi.fn(),
  clear: vi.fn(),
};

// Components — reusable UI (components.js)
globalThis.Components = {
  toast: vi.fn(),
  modal: vi.fn(),
  table: vi.fn(),
  tabs: vi.fn(),
  skeletonCard: vi.fn(),
  skeletonTable: vi.fn(() => '<div class="skeleton-table"></div>'),
  emptyState: vi.fn((cfg) => '<div class="empty-state"><p>' + (cfg.message || '') + '</p>' + (cfg.description ? '<p>' + cfg.description + '</p>' : '') + (cfg.actionLabel ? '<button data-action="empty-state-action">' + cfg.actionLabel + '</button>' : '') + '</div>'),
  FilterBar: {
    _instances: {},
    init: vi.fn(function (container, config) { return config.id; }),
    destroy: vi.fn(),
    setActive: vi.fn(),
    getActive: vi.fn(() => null),
  },
  Accordion: {
    _state: {},
    init: vi.fn(),
    destroy: vi.fn(),
    update: vi.fn(),
    getOpenSection: vi.fn(() => null),
  },
  ActionMenu: function ActionMenu(props) {
    return PreactBridge.html`<div style="position:relative"><button aria-haspopup="menu" aria-expanded="false"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><circle cx="8" cy="3" r="1.5" /><circle cx="8" cy="8" r="1.5" /><circle cx="8" cy="13" r="1.5" /></svg></button></div>`;
  },
  renderPagination: vi.fn(),
  removePagination: vi.fn(),
  renderRetryError: vi.fn(),
  createModal: vi.fn(),
  downloadProgress: vi.fn(),
  updateProgress: vi.fn(),
  showToast: vi.fn(),
  showConfirmModal: vi.fn(() => Promise.resolve(true)),
  showFieldError: vi.fn(),
  clearFieldErrors: vi.fn(),
};

// showToast / showConfirmModal — global UI helpers
globalThis.showToast = vi.fn();
globalThis.showConfirmModal = vi.fn(() => Promise.resolve(true));

// App — event bus + timer management (state.js)
globalThis.App = {
  on: vi.fn(),
  off: vi.fn(),
  emit: vi.fn(),
  addTimer: vi.fn(),
  clearScope: vi.fn(),
  clearAllTimers: vi.fn(),
  cache: { invalidatePattern: vi.fn(), get: vi.fn(() => null), set: vi.fn() },
};

// t — i18n translation function (i18n.js)
globalThis.t = vi.fn((key) => key);
