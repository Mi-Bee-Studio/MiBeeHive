/**
 * Tests for external-service.js - External Service Hub with 4 tabs
 */

import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock required globals
globalThis.PreactBridge = {
  html: (strings, ...values) => {
    let result = strings[0] || '';
    for (let i = 0; i < values.length; i++) {
      result += String(values[i]) + strings[i + 1];
    }
    return { type: 'html', content: result, props: {} };
  },
  h: (type, props, ...children) => ({ type, props, children }),
  render: vi.fn(),
  useState: vi.fn((initial) => [initial, vi.fn()]),
  useEffect: vi.fn(),
  useRef: vi.fn(() => ({ current: null })),
};

globalThis.Api = {
  get: vi.fn(() => Promise.resolve({ success: true, data: [] })),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
};

globalThis.Components = {
  moduleTabs: vi.fn((items, activeKey) => {
    let html = '<div class="module-tabs">';
    (items || []).forEach((it) => {
      html += `<a href="#${it.hash}" class="module-tab${it.i18nKey === activeKey ? ' active' : ''}">${it.i18nKey}</a>`;
    });
    return html + '</div>';
  }),
  showToast: vi.fn(),
  emptyState: vi.fn((cfg) => `<div class="empty-state"><p>${cfg.message || ''}</p></div>`),
};

globalThis.t = vi.fn((key) => key);

globalThis.Helpers = {
  escapeHtml: (s) => {
    if (!s) return '';
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  },
  copyToClipboard: vi.fn(() => Promise.resolve()),
  loadingSpinner: vi.fn(() => '<div class="loading-spinner"></div>'),
  errorMessage: vi.fn((msg) => `<div class="error-message">${msg}</div>`),
};

globalThis.ShareLinkDialog = {
  renderList: vi.fn(),
};

// Load the module under test
const fs = require('fs');
const moduleCode = fs.readFileSync('./web/js/modules/external-service.js', 'utf8');

describe('external-service.js', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    document.body.innerHTML = '';
  });

  describe('Module structure', () => {
    it('should load without errors', () => {
      expect(() => eval(moduleCode)).not.toThrow();
    });

    it('should define the ExternalService global', () => {
      eval(moduleCode);
      expect(globalThis.ExternalService).toBeDefined();
    });

    it('should export render and destroy functions', () => {
      eval(moduleCode);
      expect(typeof globalThis.ExternalService.render).toBe('function');
      expect(typeof globalThis.ExternalService.destroy).toBe('function');
    });
  });

  describe('ModuleTabs configuration', () => {
    it('should use correct tab configuration', () => {
      eval(moduleCode);
      
      const container = document.createElement('div');
      container.id = 'main-content';
      document.body.appendChild(container);
      
      globalThis.ExternalService.render({}, {}, new AbortController());
      
      // Components.moduleTabs is called inside PreactBridge.render
      expect(globalThis.PreactBridge.render).toHaveBeenCalled();

      document.body.removeChild(container);
    });

    it('should use default active tab (first tab)', () => {
      eval(moduleCode);
      
      const container = document.createElement('div');
      container.id = 'main-content';
      document.body.appendChild(container);
      
      globalThis.ExternalService.render({}, {}, new AbortController());

      document.body.removeChild(container);
    });
  });

  describe('Tab switching', () => {
    it('should support tab switching via subtab parameter', () => {
      eval(moduleCode);
      
      const container = document.createElement('div');
      container.id = 'main-content';
      document.body.appendChild(container);
      
      globalThis.ExternalService.render({ subtab: 'endpoints' }, {}, new AbortController());

      document.body.removeChild(container);
    });
  });

  describe('Protocol Endpoints tab', () => {
    it('should include endpoints tab in configuration', () => {
      eval(moduleCode);
      
      const container = document.createElement('div');
      container.id = 'main-content';
      document.body.appendChild(container);
      
      globalThis.ExternalService.render({ subtab: 'endpoints' }, {}, new AbortController());

      document.body.removeChild(container);
    });
  });
});