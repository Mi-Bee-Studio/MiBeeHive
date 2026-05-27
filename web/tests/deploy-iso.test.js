/**
 * Component tests for deploy-iso module.
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

var ISO_PATH = path.resolve(__dirname, '..', 'js', 'modules', 'deploy-iso.js');

// ── Sample data ────────────────────────────────────────────────────────────
var SAMPLE_CATALOG = [
  { id: 1, name: 'Ubuntu Server 24.04', distro: 'ubuntu', arch: 'amd64', status: 'downloaded', auto_update: true, download_status: 'downloaded', current_url: 'https://example.com/ubuntu-24.04-amd64.iso', last_checked: '2026-05-26T10:00:00Z' },
  { id: 2, name: 'Debian 12 Bookworm', distro: 'debian', arch: 'arm64', status: 'available', auto_update: false, download_status: '', current_url: '', last_checked: null },
  { id: 3, name: 'Rocky Linux 9', distro: 'rocky', arch: 'amd64', status: 'available', auto_update: false, download_status: '', current_url: '', last_checked: null }
];

var SAMPLE_QUEUE = {
  items: [],
  stats: { pending: 0, downloading: 0, downloaded: 1, error: 0, total: 1, available: 0 }
};

// ── Tests ──────────────────────────────────────────────────────────────────
describe('DeployISO', function () {
  beforeEach(function () {
    Api.get.mockReset();
    Api.post.mockReset();
    Api.put.mockReset();
    Api.delete.mockReset();
    showToast.mockReset();
    showConfirmModal.mockReset();
    // Default: return empty success
    Api.get.mockResolvedValue({ success: true, data: [] });
    createContainer();
    loadModule(ISO_PATH, 'DeployISO');
  });

  afterEach(function () {
    if (globalThis.DeployISO) {
      try { DeployISO.destroy(); } catch (e) { /* ignore */ }
    }
    removeContainer();
  });

  // 1. ISOPage renders catalog entries
  it('renders catalog entries from API', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/catalog') return Promise.resolve({ success: true, data: SAMPLE_CATALOG });
      if (url === '/admin/os-install/isos') return Promise.resolve({ success: true, data: [] });
      if (url === '/admin/os-install/catalog/queue') return Promise.resolve({ success: true, data: SAMPLE_QUEUE });
      if (url === '/admin/os-install/catalog/progress') return Promise.resolve({ success: true, data: [] });
      return Promise.resolve({ success: true, data: [] });
    });

    DeployISO.render();

    // Wait for distro section headers to appear
    await waitFor(function () {
      expect(document.body.textContent).toContain('Ubuntu');
    });

    // Expand all collapsed distro sections
    var sectionHeaders = document.querySelectorAll('#di-catalog button');
    for (var i = 0; i < sectionHeaders.length; i++) {
      fireEvent.click(sectionHeaders[i]);
    }

    // Now catalog entry names should be visible
    await waitFor(function () {
      expect(screen.getByText('Ubuntu Server 24.04')).toBeDefined();
    });
    expect(screen.getByText('Debian 12 Bookworm')).toBeDefined();
    expect(screen.getByText('Rocky Linux 9')).toBeDefined();
  });

  // 2. DownloadSummaryBar visible when active downloads
  it('shows download summary bar when downloads are active', async function () {
    var activeQueue = {
      items: [{ id: 2, download_status: 'downloading' }],
      stats: { pending: 1, downloading: 1, downloaded: 0, error: 0, total: 2, available: 1 }
    };

    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/catalog') return Promise.resolve({ success: true, data: SAMPLE_CATALOG });
      if (url === '/admin/os-install/isos') return Promise.resolve({ success: true, data: [] });
      if (url === '/admin/os-install/catalog/queue') return Promise.resolve({ success: true, data: activeQueue });
      if (url === '/admin/os-install/catalog/progress') return Promise.resolve({ success: true, data: [] });
      return Promise.resolve({ success: true, data: [] });
    });

    DeployISO.render();

    // The summary bar should appear with the "iso_download_summary" label (t() returns key)
    await waitFor(function () {
      var bar = document.querySelector('.download-summary-bar');
      expect(bar).not.toBeNull();
    });

    // Should show downloading count
    expect(screen.getByText('iso_queue_downloading:')).toBeDefined();
  });

  // 3. CatalogEntryRow shows arch badge
  it('renders arch badge on catalog entries', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/catalog') return Promise.resolve({ success: true, data: SAMPLE_CATALOG });
      if (url === '/admin/os-install/isos') return Promise.resolve({ success: true, data: [] });
      if (url === '/admin/os-install/catalog/queue') return Promise.resolve({ success: true, data: SAMPLE_QUEUE });
      if (url === '/admin/os-install/catalog/progress') return Promise.resolve({ success: true, data: [] });
      return Promise.resolve({ success: true, data: [] });
    });

    DeployISO.render();

    // Wait for distro section headers
    await waitFor(function () {
      expect(document.body.textContent).toContain('Ubuntu');
    });

    // Expand all sections
    var sectionHeaders = document.querySelectorAll('#di-catalog button');
    for (var i = 0; i < sectionHeaders.length; i++) {
      fireEvent.click(sectionHeaders[i]);
    }

    // Now check for arch badges
    await waitFor(function () {
      var badges = document.querySelectorAll('span.badge');
      var archBadges = Array.from(badges).filter(function (b) {
        return b.textContent === 'amd64' || b.textContent === 'arm64';
      });
      expect(archBadges.length).toBeGreaterThan(0);
    });
  });

  // 4. ActionMenu on catalog rows
  it('renders ActionMenu trigger on catalog rows', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/catalog') return Promise.resolve({ success: true, data: SAMPLE_CATALOG });
      if (url === '/admin/os-install/isos') return Promise.resolve({ success: true, data: [] });
      if (url === '/admin/os-install/catalog/queue') return Promise.resolve({ success: true, data: SAMPLE_QUEUE });
      if (url === '/admin/os-install/catalog/progress') return Promise.resolve({ success: true, data: [] });
      return Promise.resolve({ success: true, data: [] });
    });

    DeployISO.render();

    // Wait for distro section headers
    await waitFor(function () {
      expect(document.body.textContent).toContain('Ubuntu');
    });

    // Expand all sections
    var sectionHeaders = document.querySelectorAll('#di-catalog button');
    for (var i = 0; i < sectionHeaders.length; i++) {
      fireEvent.click(sectionHeaders[i]);
    }

    // Now check for ActionMenu triggers
    await waitFor(function () {
      var triggers = document.querySelectorAll('[aria-haspopup="menu"]');
      expect(triggers.length).toBeGreaterThan(0);
    });
  });
});
