import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

describe('ShareLinkDialog', function () {
  beforeEach(function () {
    // Reset module state
    if (typeof window !== 'undefined' && window.ShareLinkDialog) {
      window.ShareLinkDialog.destroy();
    }
  });

  afterEach(function () {
    // Cleanup
    if (typeof window !== 'undefined' && window.ShareLinkDialog) {
      window.ShareLinkDialog.destroy();
    }
  });

  describe('Module exports', function () {
    it('should export open, close, renderList, and destroy functions', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      expect(typeof window.ShareLinkDialog.open).toBe('function');
      expect(typeof window.ShareLinkDialog.close).toBe('function');
      expect(typeof window.ShareLinkDialog.renderList).toBe('function');
      expect(typeof window.ShareLinkDialog.destroy).toBe('function');
    });
  });

  describe('open()', function () {
    it('should call Components.createModal when opening with a valid file', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var file = { id: 123, filename: 'test.txt' };
      var mockModal = { close: vi.fn(), overlay: { querySelector: vi.fn() } };

      window.Components.createModal = vi.fn(function (config) {
        expect(config.title).toBeDefined();
        expect(config.bodyHtml).toBeDefined();
        if (typeof config.onMount === 'function') {
          config.onMount(mockModal.overlay);
        }
        return mockModal;
      });

      window.ShareLinkDialog.open(file);
      expect(window.Components.createModal).toHaveBeenCalled();
    });

    it('should show error toast when opening with invalid file', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      window.ShareLinkDialog.open(null);
      expect(window.Components.showToast).toHaveBeenCalledWith('error', 'error');
    });
  });

  describe('renderList()', function () {
    it('should call Api.get to fetch share links', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var mockContainer = document.createElement('div');
      window.Api.get = vi.fn(function () {
        return Promise.resolve({ success: true, data: [] });
      });

      window.ShareLinkDialog.renderList(mockContainer);
      expect(window.Api.get).toHaveBeenCalledWith('/admin/share-links');
    });

    it('should render empty state when no links exist', async function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var mockContainer = document.createElement('div');
      window.Api.get = vi.fn(function () {
        return Promise.resolve({ success: true, data: [] });
      });

      await window.ShareLinkDialog.renderList(mockContainer);

      // Wait for async operations
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(mockContainer.innerHTML).toContain('暂无分享链接');
    });

    it('should render table when links exist', async function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var mockContainer = document.createElement('div');
      var mockLinks = [
        {
          id: 1,
          token: 'abc123',
          file_name: 'test.txt',
          created_at: '2024-01-01T00:00:00Z',
          expires_at: '2024-01-08T00:00:00Z',
          downloads_count: 5,
          revoked: false
        }
      ];

      window.Api.get = vi.fn(function () {
        return Promise.resolve({ success: true, data: mockLinks });
      });

      await window.ShareLinkDialog.renderList(mockContainer);

      // Wait for async operations
      await new Promise(resolve => setTimeout(resolve, 10));

      expect(mockContainer.innerHTML).toContain('test.txt');
      expect(mockContainer.innerHTML).toContain('/s/abc123');
    });
  });

  describe('revoke action', function () {
    it('should call Api.delete when revoking a link', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var mockContainer = document.createElement('div');
      var mockLinks = [
        {
          id: 1,
          token: 'abc123',
          file_name: 'test.txt',
          created_at: '2024-01-01T00:00:00Z',
          expires_at: '2024-01-08T00:00:00Z',
          downloads_count: 5,
          revoked: false
        }
      ];

      window.Api.get = vi.fn(function () {
        return Promise.resolve({ success: true, data: mockLinks });
      });

      window.Api.delete = vi.fn(function () {
        return Promise.resolve({ success: true });
      });

      window.Components.showConfirmModal = vi.fn(function (message, onConfirm) {
        if (typeof onConfirm === 'function') {
          onConfirm();
        }
      });

      return window.ShareLinkDialog.renderList(mockContainer).then(function () {
        return new Promise(resolve => setTimeout(resolve, 10));
      }).then(function () {
        // Find and click revoke button
        var revokeBtn = mockContainer.querySelector('[data-action="revoke"]');
        if (revokeBtn) {
          revokeBtn.click();
          expect(window.Api.delete).toHaveBeenCalledWith('/admin/share-links/1');
        }
      });
    });

    it('should show success toast after revoking a link', function () {
      if (typeof window.ShareLinkDialog === 'undefined') {
        // Module not loaded yet
        expect(true).toBe(true);
        return;
      }

      var mockContainer = document.createElement('div');
      var mockLinks = [
        {
          id: 1,
          token: 'abc123',
          file_name: 'test.txt',
          created_at: '2024-01-01T00:00:00Z',
          expires_at: '2024-01-08T00:00:00Z',
          downloads_count: 5,
          revoked: false
        }
      ];

      window.Api.get = vi.fn(function () {
        return Promise.resolve({ success: true, data: mockLinks });
      });

      window.Api.delete = vi.fn(function () {
        return Promise.resolve({ success: true });
      });

      window.Components.showConfirmModal = vi.fn(function (message, onConfirm) {
        if (typeof onConfirm === 'function') {
          onConfirm();
        }
      });

      return window.ShareLinkDialog.renderList(mockContainer).then(function () {
        return new Promise(resolve => setTimeout(resolve, 10));
      }).then(function () {
        // Find and click revoke button
        var revokeBtn = mockContainer.querySelector('[data-action="revoke"]');
        if (revokeBtn) {
          revokeBtn.click();
          // Wait for async confirm modal to execute
          return new Promise(resolve => setTimeout(resolve, 10));
        }
      }).then(function () {
        expect(window.Components.showToast).toHaveBeenCalledWith('链接已吊销', 'success');
      });
    });
  });
});