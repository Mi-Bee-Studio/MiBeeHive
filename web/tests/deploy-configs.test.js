/**
 * Component tests for deploy-configs module.
 */
const fs = require('fs');
const path = require('path');
const { screen, waitFor, fireEvent, within } = require('@testing-library/preact');

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

var CONFIGS_PATH = path.resolve(__dirname, '..', 'js', 'modules', 'deploy-configs.js');

// ── Sample data ────────────────────────────────────────────────────────────
var SAMPLE_CONFIGS = [
  { id: 1, name: 'web-server-01', config_name: 'web-server-01', os_type: 'debian', enabled: true, config: '{"hostname":"web01"}' },
  { id: 2, name: 'db-server', config_name: 'db-server', os_type: 'ubuntu', enabled: false, config: '{}' },
  { id: 3, name: 'cache-node', config_name: 'cache-node', os_type: 'debian', enabled: true, config: '{}' }
];

// ── Tests ──────────────────────────────────────────────────────────────────
describe('DeployConfigs', function () {
  beforeEach(function () {
    // Reset API mocks
    Api.get.mockReset();
    Api.post.mockReset();
    Api.put.mockReset();
    Api.delete.mockReset();
    showToast.mockReset();
    showConfirmModal.mockReset();
    // Default: return empty success
    Api.get.mockResolvedValue({ success: true, data: [] });
    createContainer();
    // Load module fresh
    loadModule(CONFIGS_PATH, 'DeployConfigs');
  });

  afterEach(function () {
    if (globalThis.DeployConfigs) {
      try { DeployConfigs.destroy(); } catch (e) { /* ignore */ }
    }
    removeContainer();
  });

  // 1. ConfigsPage renders config list
  it('renders config list from API', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/configs') {
        return Promise.resolve({ success: true, data: SAMPLE_CONFIGS });
      }
      if (url === '/admin/os-install/catalog') {
        return Promise.resolve({ success: true, data: [] });
      }
      return Promise.resolve({ success: true, data: [] });
    });

    DeployConfigs.render();

    await waitFor(function () {
      expect(screen.getAllByText('web-server-01').length).toBeGreaterThan(0);
    });

    expect(screen.getAllByText('db-server').length).toBeGreaterThan(0);
    expect(screen.getAllByText('cache-node').length).toBeGreaterThan(0);
  });

  // 2. ConfigsPage empty state
  it('shows empty state when no configs', async function () {
    Api.get.mockResolvedValue({ success: true, data: [] });

    DeployConfigs.render();

    await waitFor(function () {
      // t() returns the key, so look for the i18n key used in empty state
      var el = document.querySelector('.empty-state');
      expect(el).not.toBeNull();
    });
  });

  // 3. Search filters configs
  it('search input filters config list', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/configs') {
        return Promise.resolve({ success: true, data: SAMPLE_CONFIGS });
      }
      return Promise.resolve({ success: true, data: [] });
    });

    DeployConfigs.render();

    // Wait for configs to render
    await waitFor(function () {
      expect(screen.getAllByText('web-server-01').length).toBeGreaterThan(0);
    });

    // Type search query
    var searchInput = document.querySelector('.search-input');
    expect(searchInput).not.toBeNull();
    fireEvent.input(searchInput, { target: { value: 'web' } });

    // web-server-01 should still be visible, db-server and cache-node should be filtered
    await waitFor(function () {
      expect(screen.queryByText('db-server')).toBeNull();
    });
    expect(screen.getAllByText('web-server-01').length).toBeGreaterThan(0);
  });

  // 4. ConfigEditorModal renders all fields in create mode
  it('editor modal shows name, OS type, hostname, username fields', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/configs') {
        return Promise.resolve({ success: true, data: SAMPLE_CONFIGS });
      }
      return Promise.resolve({ success: true, data: [] });
    });

    DeployConfigs.render();

    await waitFor(function () {
      expect(screen.getAllByText('web-server-01').length).toBeGreaterThan(0);
    });

    // Click the "New" button (t() returns key 'osinstall_new')
    var newButtons = Array.from(document.querySelectorAll('button'));
    var newBtn = newButtons.find(function (b) { return b.textContent === 'osinstall_new'; });
    expect(newBtn).toBeDefined();
    fireEvent.click(newBtn);

    // Wait for modal to appear
    await waitFor(function () {
      var modal = document.querySelector('.modal-overlay');
      expect(modal).not.toBeNull();
    });

    // Scope queries to the modal to avoid matching table headers
    var modalEl = document.querySelector('.modal-overlay');
    var modalQueries = within(modalEl);
    expect(modalQueries.getAllByText('osinstall_name').length).toBeGreaterThan(0);
    expect(modalQueries.getAllByText('osinstall_os_type').length).toBeGreaterThan(0);
    expect(modalQueries.getByText('osinstall_hostname')).toBeDefined();
    expect(modalQueries.getByText('osinstall_username')).toBeDefined();

    // Advanced toggle should be present
    expect(modalQueries.getByText('osinstall_advanced_settings')).toBeDefined();
  });

  // 5. ConfigEditorModal validation on empty submit
  it('editor modal shows validation errors on empty submit', async function () {
    Api.get.mockImplementation(function (url) {
      if (url === '/admin/os-install/configs') {
        return Promise.resolve({ success: true, data: SAMPLE_CONFIGS });
      }
      return Promise.resolve({ success: true, data: [] });
    });

    DeployConfigs.render();

    await waitFor(function () {
      expect(screen.getAllByText('web-server-01').length).toBeGreaterThan(0);
    });

    // Open create modal
    var newButtons = Array.from(document.querySelectorAll('button'));
    var newBtn = newButtons.find(function (b) { return b.textContent === 'osinstall_new'; });
    expect(newBtn).toBeDefined();
    fireEvent.click(newBtn);

    await waitFor(function () {
      expect(document.querySelector('.modal-overlay')).not.toBeNull();
    });

    // Click Save without filling in any fields — t('save') returns 'save'
    var modalEl = document.querySelector('.modal-overlay');
    var modalQueries = within(modalEl);
    var saveBtn = Array.from(modalEl.querySelectorAll('button')).find(function (b) { return b.textContent === 'save'; });
    expect(saveBtn).toBeDefined();
    fireEvent.click(saveBtn);

    // Validation errors should appear — t() returns the key, so error text is 'validation_required'
    await waitFor(function () {
      expect(modalQueries.getAllByText('validation_required').length).toBeGreaterThan(0);
    });
  });
});
