/**
 * Component tests for file-detail module (File Detail Drawer).
 */
const fs = require('fs');
const path = require('path');

// ── Module loader ──────────────────────────────────────────────────────────
function loadModule(modulePath, globalName) {
  var code = fs.readFileSync(modulePath, 'utf8');
  var transformed = code.replace('const ' + globalName + ' =', 'globalThis.' + globalName + ' =');
  var fn = new Function(transformed);
  fn.call(globalThis);
}

var MODULE_PATH = path.resolve(__dirname, '..', 'js', 'modules', 'file-detail.js');

// ── Sample data ────────────────────────────────────────────────────────────
var SAMPLE_FILE = {
  id: 1,
  public_token: 'tok1',
  filename: 'node-v20.0.0-linux-amd64.tar.gz',
  version: '20.0.0',
  os: 'linux',
  arch: 'arm64',
  size_bytes: 1024,
  source_type: 'github',
  category: 'runtime',
  status: 'complete',
  checksum: 'sha256:abc123',
  created_at: '2026-01-15T00:00:00Z'
};

function panelText() {
  var p = document.querySelector('.drawer-panel');
  return p ? p.textContent : '';
}

describe('FileDetail', function () {
  beforeEach(function () {
    Api.get.mockReset();
    Components.showToast.mockReset();
    Api.get.mockImplementation(function () { return Promise.resolve({ success: true, data: null }); });
    loadModule(MODULE_PATH, 'FileDetail');
  });

  afterEach(function () {
    if (globalThis.FileDetail) {
      try { FileDetail.destroy(); } catch (e) { /* ignore */ }
    }
    document.body.innerHTML = '';
  });

  it('opens drawer and renders metadata', function () {
    FileDetail.open(SAMPLE_FILE);
    expect(document.querySelector('.drawer-panel')).not.toBeNull();
    expect(panelText()).toContain('node-v20.0.0-linux-amd64.tar.gz');
    expect(panelText()).toContain('20.0.0');
    expect(panelText()).toContain('linux');
    expect(panelText()).toContain('arm64');
    expect(panelText()).toContain('github');
    expect(panelText()).toContain('runtime');
    expect(panelText()).toContain('sha256:abc123');
  });

  it('fetches internal details for local_path', function () {
    // Regression guard for #59: physical paths must never appear in the UI.
    // The drawer must not render any local_path, even if one is provided.
    FileDetail.open(Object.assign({}, SAMPLE_FILE, { local_path: '/data/oss/node.tar.gz' }));
    expect(panelText()).not.toContain('/data/oss/node.tar.gz');
  });

  it('close hides the drawer', function () {
    FileDetail.open(SAMPLE_FILE);
    expect(FileDetail.isOpen()).toBe(true);
    FileDetail.close();
    expect(FileDetail.isOpen()).toBe(false);
  });
});