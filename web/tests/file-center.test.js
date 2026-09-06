/**
 * Component tests for file-center module (File Center home page).
 */
const fs = require('fs');
const path = require('path');
const { screen, waitFor, fireEvent } = require('@testing-library/preact');

// ── Module loader ──────────────────────────────────────────────────────────
function loadModule(modulePath, globalName) {
  var code = fs.readFileSync(modulePath, 'utf8');
  var transformed = code.replace('const ' + globalName + ' =', 'globalThis.' + globalName + ' =');
  var fn = new Function(transformed);
  fn.call(globalThis);
}

// ── Helpers ────────────────────────────────────────────────────────────────
function createContainer() {
  var el = document.createElement('div');
  el.id = 'main-content';
  document.body.appendChild(el);
  return el;
}

function removeContainer() {
  var el = document.getElementById('main-content');
  if (el) el.remove();
}

var MODULE_PATH = path.resolve(__dirname, '..', 'js', 'modules', 'file-center.js');

// ── Sample data ────────────────────────────────────────────────────────────
var SAMPLE_FILES = [
  { id: 1, public_token: 'tok1', filename: 'node-v20.0.0-linux-amd64.tar.gz', version: '20.0.0', os: 'linux', arch: 'amd64', size_bytes: 1024, source_type: 'github', status: 'complete' },
  { id: 2, public_token: 'tok2', filename: 'prometheus-2.50.0.linux-arm64.tar.gz', version: '2.50.0', os: 'linux', arch: 'arm64', size_bytes: 2048, source_type: 'github', status: 'complete' },
];

var WEBDAV_STATUS = { enabled: true, https_url: 'https://host:9443/webdav/', storage_path: '/data/webdav' };
var CHANNELS = [{ id: 1, name: 'public', slug: 'public' }];

function mockApi(files, total) {
  Api.getWithHeaders.mockImplementation(function (url) {
    if (url.indexOf('/admin/files') === 0) {
      return Promise.resolve({ data: { success: true, data: files }, total: total });
    }
    return Promise.resolve({ data: { success: true, data: [] }, total: 0 });
  });
  Api.get.mockImplementation(function (url) {
    if (url === '/admin/webdav/status') return Promise.resolve({ success: true, data: WEBDAV_STATUS });
    if (url === '/admin/channels') return Promise.resolve({ success: true, data: CHANNELS });
    return Promise.resolve({ success: true, data: [] });
  });
}

// ── Tests ──────────────────────────────────────────────────────────────────
describe('FileCenter', function () {
  beforeEach(function () {
    Api.getWithHeaders.mockReset();
    Api.get.mockReset();
    Api.post.mockReset();
    Components.showToast.mockReset();
    createContainer();
    loadModule(MODULE_PATH, 'FileCenter');
  });

  afterEach(function () {
    if (globalThis.FileCenter) {
      try { FileCenter.destroy(); } catch (e) { /* ignore */ }
    }
    removeContainer();
  });

  it('renders file list from API', async function () {
    mockApi(SAMPLE_FILES, 2);
    FileCenter.render();

    await waitFor(function () {
      expect(screen.getAllByText('node-v20.0.0-linux-amd64.tar.gz').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('prometheus-2.50.0.linux-arm64.tar.gz').length).toBeGreaterThan(0);
    // Version + arch cells
    expect(screen.getAllByText('20.0.0').length).toBeGreaterThan(0);
    expect(screen.getAllByText('arm64').length).toBeGreaterThan(0);
  });

  it('shows empty state when no files', async function () {
    mockApi([], 0);
    FileCenter.render();

    await waitFor(function () {
      expect(document.querySelector('.empty-state')).not.toBeNull();
    });
  });

  it('search input triggers API with q param', async function () {
    mockApi(SAMPLE_FILES, 2);
    FileCenter.render();

    await waitFor(function () {
      expect(screen.getAllByText('node-v20.0.0-linux-amd64.tar.gz').length).toBeGreaterThan(0);
    });

    var searchInput = document.querySelector('.input-search');
    expect(searchInput).not.toBeNull();
    fireEvent.input(searchInput, { target: { value: 'prometheus' } });

    await waitFor(function () {
      var calls = Api.getWithHeaders.mock.calls;
      var last = calls[calls.length - 1][0];
      expect(last).toContain('q=prometheus');
    });
  });

  it('filter button triggers query with os param', async function () {
    mockApi(SAMPLE_FILES, 2);
    FileCenter.render();

    await waitFor(function () {
      expect(screen.getAllByText('node-v20.0.0-linux-amd64.tar.gz').length).toBeGreaterThan(0);
    });

    var linuxBtn = Array.from(document.querySelectorAll('.filter-btn')).find(function (b) {
      return b.textContent === 'linux';
    });
    expect(linuxBtn).toBeDefined();
    fireEvent.click(linuxBtn);

    await waitFor(function () {
      var calls = Api.getWithHeaders.mock.calls;
      var last = calls[calls.length - 1][0];
      expect(last).toContain('os=linux');
    });
  });

  it('renders WebDAV status card', async function () {
    mockApi(SAMPLE_FILES, 2);
    FileCenter.render();

    await waitFor(function () {
      expect(screen.getAllByText('file_center_webdav_card_title').length).toBeGreaterThan(0);
    });
    // Channel dropdown populated from /admin/channels
    await waitFor(function () {
      expect(screen.getAllByText('public').length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText('public').length).toBeGreaterThan(0);
  });

  it('renders pagination summary', async function () {
    mockApi(SAMPLE_FILES, 2);
    FileCenter.render();

    await waitFor(function () {
      expect(screen.getAllByText('file_center_showing').length).toBeGreaterThan(0);
    });
  });
});