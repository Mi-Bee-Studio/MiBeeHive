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
  escapeHtml: vi.fn((s) => s || ''),
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
  ICONS: {},
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
  clearAllTimers: vi.fn(),
};

// t — i18n translation function (i18n.js)
globalThis.t = vi.fn((key) => key);
