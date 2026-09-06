// Tests for modules/file-bulk.js — bulk operations for the file center.
import { describe, it, expect, vi, beforeEach } from 'vitest';
import '../js/modules/file-bulk.js';

const FileBulk = window.FileBulk;

// loadFiles() runs in the mount effect (scheduled async by Preact). Give the
// mock a persistent resolved response so the effect never throws, even after
// vi.clearAllMocks() in beforeEach (which keeps implementations).
Api.getWithHeaders.mockResolvedValue({ data: { success: true, data: [] }, total: 0 });

describe('file-bulk', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the bulk action bar with all action buttons', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    PreactBridge.render(
      PreactBridge.h(FileBulk.FileBulkComponent, { initialSelected: ['1', '2'] }),
      container
    );
    const bar = container.querySelector('.bulk-bar');
    expect(bar).not.toBeNull();
    const text = bar.textContent;
    expect(text).toContain('bulk.zip_download');
    expect(text).toContain('bulk.copy_link');
    expect(text).toContain('bulk.publish_to_view');
    expect(text).toContain('bulk.delete');
    PreactBridge.render(null, container);
    document.body.removeChild(container);
  });

  it('checkbox selection toggles the selected set', () => {
    const c = new FileBulk.SelectionController();
    c.toggle('1', 0);
    expect(c.isSelected('1')).toBe(true);
    expect(c.count()).toBe(1);
    c.toggle('1', 0);
    expect(c.isSelected('1')).toBe(false);
    expect(c.count()).toBe(0);
  });

  it('select all toggles all items', () => {
    const c = new FileBulk.SelectionController();
    c.selectAll([{ id: 1 }, { id: 2 }, { id: 3 }]);
    expect(c.count()).toBe(3);
    c.deselectAll();
    expect(c.count()).toBe(0);
  });

  it('delete action triggers the confirm modal', () => {
    const onConfirm = vi.fn();
    FileBulk.confirmDelete(['1', '2'], onConfirm);
    expect(Components.showConfirmModal).toHaveBeenCalledTimes(1);
    const [message, cb] = Components.showConfirmModal.mock.calls[0];
    expect(message).toContain('bulk.delete_confirm_message');
    expect(typeof cb).toBe('function');
    cb();
    expect(onConfirm).toHaveBeenCalled();
  });

  it('zip download posts to the correct API endpoint', async () => {
    const fetchMock = vi.fn(() =>
      Promise.resolve({ ok: true, blob: () => Promise.resolve(new Blob(['x'])) })
    );
    global.fetch = fetchMock;
    URL.createObjectURL = vi.fn(() => 'blob:mock');
    URL.revokeObjectURL = vi.fn();

    await FileBulk.BulkActions.downloadZip(['1', '2']);

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/files/zip',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ file_ids: ['1', '2'] })
      })
    );
  });
});