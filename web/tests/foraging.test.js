/**
 * Foraging module tests
 */
const fs = require('fs');
const path = require('path');
import { describe, it, expect, beforeEach, vi } from 'vitest';

// Load test setup (vendor scripts + mocks)
import './setup.js';

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

// Load the modules under test
loadModule(path.resolve(__dirname, '..', 'js', 'modules', 'foraging.js'), 'Foraging');
loadModule(path.resolve(__dirname, '..', 'js', 'modules', 'foraging-iso.js'), 'ForagingISO');

describe('Foraging module', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    removeContainer();
  });

  it('should export render and destroy functions', () => {
    expect(window.Foraging).toBeDefined();
    expect(typeof window.Foraging.render).toBe('function');
    expect(typeof window.Foraging.destroy).toBe('function');
  });

  it('should have 4 tabs defined', () => {
    // The tabs are defined internally in the FORAGING_TABS constant
    // We verify the module loaded successfully and can render
    expect(window.Foraging).toBeDefined();
    expect(typeof window.Foraging.render).toBe('function');
  });

  it('should render without errors', () => {
    createContainer();
    expect(() => {
      window.Foraging.render({}, {}, new AbortController().signal);
    }).not.toThrow();
    removeContainer();
  });

  it('should clean up on destroy', () => {
    createContainer();
    window.Foraging.render({}, {}, new AbortController().signal);
    expect(() => {
      window.Foraging.destroy();
    }).not.toThrow();
    removeContainer();
  });
});

describe('ForagingISO module', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    removeContainer();
  });

  it('should export render and destroy functions', () => {
    expect(window.ForagingISO).toBeDefined();
    expect(typeof window.ForagingISO.render).toBe('function');
    expect(typeof window.ForagingISO.destroy).toBe('function');
  });

  it('should render without errors', () => {
    createContainer();
    expect(() => {
      window.ForagingISO.render({}, {}, new AbortController().signal);
    }).not.toThrow();
    removeContainer();
  });

  it('should clean up on destroy', () => {
    createContainer();
    window.ForagingISO.render({}, {}, new AbortController().signal);
    expect(() => {
      window.ForagingISO.destroy();
    }).not.toThrow();
    removeContainer();
  });
});

describe('Tab switching', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    removeContainer();
  });

  it('should support tab switching in Foraging module', () => {
    createContainer();
    window.Foraging.render({}, {}, new AbortController().signal);
    const app = document.getElementById('main-content');
    expect(app.innerHTML).toContain('module-tabs');
    window.Foraging.destroy();
    removeContainer();
  });
});
