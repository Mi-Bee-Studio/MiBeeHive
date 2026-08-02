// Module: modules/share-link-dialog — Share Link Generation Dialog & Management List
//
// Modal for creating share links with expiry/max downloads/notes, and a
// management list for viewing all active/expired/revoked share links.
const ShareLinkDialog = (function () {
  'use strict';

  var _currentFile = null;
  var _currentModal = null;
  var _currentContainer = null;

  function _t(k) { return typeof t === 'function' ? t(k) : k; }

  // Open generation dialog for a file
  function open(file) {
    if (!file || !file.id) {
      if (typeof Components !== 'undefined' && Components.showToast) {
        Components.showToast(_t('error'), 'error');
      }
      return;
    }

    _currentFile = file;

    var bodyHtml = ''
      + '<div style="display:flex;flex-direction:column;gap:var(--space-md)">'
      + '  <div>'
      + '    <label style="display:block;font-size:var(--font-size-sm);color:var(--color-text-tertiary);margin-bottom:var(--space-xs)">' + _t('share_link.expiry') + '</label>'
      + '    <select id="share-link-expiry" style="width:100%;padding:var(--space-sm);border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-bg);color:var(--color-text)">'
      + '      <option value="1h">' + _t('share_link.1h') + '</option>'
      + '      <option value="1d">' + _t('share_link.1d') + '</option>'
      + '      <option value="7d" selected>' + _t('share_link.7d') + '</option>'
      + '      <option value="permanent">' + _t('share_link.permanent') + '</option>'
      + '    </select>'
      + '  </div>'
      + '  <div>'
      + '    <label style="display:block;font-size:var(--font-size-sm);color:var(--color-text-tertiary);margin-bottom:var(--space-xs)">' + _t('share_link.max_downloads') + '</label>'
      + '    <input type="number" id="share-link-max-downloads" value="0" min="0" placeholder="' + _t('share_link.unlimited') + '" style="width:100%;padding:var(--space-sm);border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-bg);color:var(--color-text)">'
      + '    <div style="font-size:var(--font-size-xs);color:var(--color-text-tertiary);margin-top:var(--space-2xs)">' + _t('share_link.unlimited') + '</div>'
      + '  </div>'
      + '  <div>'
      + '    <label style="display:block;font-size:var(--font-size-sm);color:var(--color-text-tertiary);margin-bottom:var(--space-xs)">' + _t('share_link.notes') + '</label>'
      + '    <textarea id="share-link-notes" rows="2" placeholder="" style="width:100%;padding:var(--space-sm);border:1px solid var(--color-border);border-radius:var(--radius-sm);background:var(--color-bg);color:var(--color-text);resize:vertical"></textarea>'
      + '  </div>'
      + '  <div id="share-link-result" style="display:none;padding:var(--space-md);background:var(--color-bg-subtle);border-radius:var(--radius-sm);border:1px solid var(--color-border)">'
      + '    <div style="font-size:var(--font-size-sm);font-weight:var(--font-weight-semibold);color:var(--color-text);margin-bottom:var(--space-sm)">' + _t('share_link.file_name') + '</div>'
      + '    <div id="share-link-url" style="font-family:monospace;font-size:var(--font-size-sm);color:var(--color-text-tertiary);word-break:break-all;margin-bottom:var(--space-sm)"></div>'
      + '    <button id="share-link-copy-btn" class="btn btn-secondary btn-sm">' + _t('share_link.copy') + '</button>'
      + '  </div>'
      + '  <div style="display:flex;justify-content:flex-end;gap:var(--space-sm);margin-top:var(--space-md)">'
      + '    <button id="share-link-cancel" class="btn btn-secondary">' + _t('common_cancel') + '</button>'
      + '    <button id="share-link-generate" class="btn btn-primary">' + _t('share_link.generate') + '</button>'
      + '  </div>'
      + '</div>';

    if (typeof Components !== 'undefined' && Components.createModal) {
      _currentModal = Components.createModal({
        title: _t('share_link.title'),
        bodyHtml: bodyHtml,
        size: '500px',
        onMount: function (overlay) {
          // Bind generate button
          var generateBtn = overlay.querySelector('#share-link-generate');
          var cancelBtn = overlay.querySelector('#share-link-cancel');
          var copyBtn = overlay.querySelector('#share-link-copy-btn');

          if (generateBtn) {
            generateBtn.addEventListener('click', handleGenerate);
          }
          if (cancelBtn) {
            cancelBtn.addEventListener('click', function () {
              if (_currentModal && _currentModal.close) _currentModal.close();
            });
          }
          if (copyBtn) {
            copyBtn.addEventListener('click', handleCopyResult);
          }
        },
        onClose: function () {
          _currentModal = null;
          _currentFile = null;
        }
      });
    }
  }

  function handleGenerate() {
    if (!_currentFile || !_currentModal) return;

    var overlay = _currentModal.overlay;
    if (!overlay) return;

    var expirySelect = overlay.querySelector('#share-link-expiry');
    var maxDownloadsInput = overlay.querySelector('#share-link-max-downloads');
    var notesInput = overlay.querySelector('#share-link-notes');

    var expiry = expirySelect ? expirySelect.value : '7d';
    var maxDownloads = maxDownloadsInput ? parseInt(maxDownloadsInput.value, 10) || 0 : 0;
    var notes = notesInput ? notesInput.value.trim() : '';

    var payload = {
      file_id: _currentFile.id,
      expires_in: expiry === 'permanent' ? '' : expiry,
      max_downloads: maxDownloads,
      notes: notes
    };

    // Call API to create share link
    if (typeof Api !== 'undefined' && Api.post) {
      Api.post('/admin/share-links', payload).then(function (res) {
        if (!res || !res.success) {
          if (typeof Components !== 'undefined' && Components.showToast) {
            Components.showToast(res ? res.message || _t('error') : _t('error'), 'error');
          }
          return;
        }

        // Show result
        var resultDiv = overlay.querySelector('#share-link-result');
        var urlDiv = overlay.querySelector('#share-link-url');
        var generateBtn = overlay.querySelector('#share-link-generate');

        if (resultDiv && urlDiv && res.data && res.data.token) {
          var shareUrl = window.location.origin + '/s/' + res.data.token;
          urlDiv.textContent = shareUrl;
          resultDiv.style.display = 'block';
          if (generateBtn) generateBtn.disabled = true;

          if (typeof Components !== 'undefined' && Components.showToast) {
            Components.showToast(_t('share_link.generate_success'), 'success');
          }
        }
      }).catch(function (err) {
        if (typeof Components !== 'undefined' && Components.showToast) {
          Components.showToast(_t('error') + ': ' + (err.message || err), 'error');
        }
      });
    }
  }

  function handleCopyResult() {
    var urlDiv = _currentModal && _currentModal.overlay && _currentModal.overlay.querySelector('#share-link-url');
    if (!urlDiv || !urlDiv.textContent) return;

    if (typeof Helpers !== 'undefined' && Helpers.copyToClipboard) {
      Helpers.copyToClipboard(urlDiv.textContent).then(function () {
        if (typeof Components !== 'undefined' && Components.showToast) {
          Components.showToast(_t('share_link.copied'), 'success');
        }
      });
    }
  }

  function close() {
    if (_currentModal && _currentModal.close) {
      _currentModal.close();
    }
    _currentModal = null;
    _currentFile = null;
  }

  // Render list of all share links in a container
  function renderList(container) {
    if (!container) return;

    _currentContainer = container;

    // Show loading
    container.innerHTML = '<div style="padding:var(--space-lg);text-align:center">' +
      '<div class="spinner spinner-lg"></div>' +
      '</div>';

    // Fetch share links
    if (typeof Api !== 'undefined' && Api.get) {
      Api.get('/admin/share-links').then(function (res) {
        if (!res || !res.success) {
          renderError(container, res ? res.message : _t('error'));
          return;
        }

        var links = res.data || [];
        renderListContent(container, links);
      }).catch(function (err) {
        renderError(container, err.message || _t('error'));
      });
    }
  }

  function renderListContent(container, links) {
    if (!links || links.length === 0) {
      container.innerHTML = '<div style="padding:var(--space-xl);text-align:center">' +
        '<div style="color:var(--color-text-quaternary);margin-bottom:var(--space-md)">' +
        (typeof Helpers !== 'undefined' && Helpers.ICONS ? Helpers.ICONS.inbox : '') +
        '</div>' +
        '<p style="color:var(--color-text-tertiary);font-size:var(--font-size-sm)">' + _t('share_link.no_links') + '</p>' +
        '</div>';
      return;
    }

    var html = '<table style="width:100%;border-collapse:collapse">' +
      '<thead>' +
      '<tr style="border-bottom:1px solid var(--color-border)">' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('share_link.file_name') + '</th>' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('share_link.link') + '</th>' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('share_link.created') + '</th>' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('share_link.expiry') + '</th>' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('share_link.downloads') + '</th>' +
      '<th style="text-align:left;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">Status</th>' +
      '<th style="text-align:right;padding:var(--space-sm);font-size:var(--font-size-xs);color:var(--color-text-tertiary);font-weight:var(--font-weight-semibold);text-transform:uppercase">' + _t('actions') + '</th>' +
      '</tr>' +
      '</thead>' +
      '<tbody>';

    links.forEach(function (link) {
      var status = getShareLinkStatus(link);
      var statusBadge = getStatusBadgeHtml(status);
      var shareUrl = window.location.origin + '/s/' + link.token;

      html += '<tr style="border-bottom:1px solid var(--color-border-subtle)">' +
        '<td style="padding:var(--space-sm);color:var(--color-text);font-size:var(--font-size-sm)">' + escapeHtml(link.file_name || '-') + '</td>' +
        '<td style="padding:var(--space-sm);color:var(--color-text);font-size:var(--font-size-sm);font-family:monospace;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="' + escapeHtml(shareUrl) + '">' + escapeHtml(shareUrl) + '</td>' +
        '<td style="padding:var(--space-sm);color:var(--color-text-tertiary);font-size:var(--font-size-sm)">' + formatTime(link.created_at) + '</td>' +
        '<td style="padding:var(--space-sm);color:var(--color-text-tertiary);font-size:var(--font-size-sm)">' + formatExpiry(link.expires_at) + '</td>' +
        '<td style="padding:var(--space-sm);color:var(--color-text);font-size:var(--font-size-sm)">' + (link.downloads_count || 0) + '</td>' +
        '<td style="padding:var(--space-sm)">' + statusBadge + '</td>' +
        '<td style="padding:var(--space-sm);text-align:right">' +
        '<button class="btn btn-ghost btn-sm" data-action="copy" data-token="' + escapeHtml(link.token) + '" style="margin-right:var(--space-2xs)">' + _t('share_link.copy') + '</button>' +
        '<button class="btn btn-ghost btn-sm" data-action="revoke" data-id="' + link.id + '" style="color:var(--color-error)">' + _t('share_link.revoke') + '</button>' +
        '</td>' +
        '</tr>';
    });

    html += '</tbody></table>';

    container.innerHTML = html;

    // Bind action buttons
    container.addEventListener('click', handleListAction);
  }

  function handleListAction(e) {
    var target = e.target;
    if (!target || target.tagName !== 'BUTTON') return;

    var action = target.getAttribute('data-action');
    if (!action) return;

    if (action === 'copy') {
      var token = target.getAttribute('data-token');
      if (token) {
        var shareUrl = window.location.origin + '/s/' + token;
        if (typeof Helpers !== 'undefined' && Helpers.copyToClipboard) {
          Helpers.copyToClipboard(shareUrl).then(function () {
            if (typeof Components !== 'undefined' && Components.showToast) {
              Components.showToast(_t('share_link.copied'), 'success');
            }
          });
        }
      }
    } else if (action === 'revoke') {
      var linkId = target.getAttribute('data-id');
      if (linkId) {
        revokeLink(linkId);
      }
    }
  }

  function revokeLink(linkId) {
    if (typeof Components !== 'undefined' && Components.showConfirmModal) {
      Components.showConfirmModal(_t('common_delete'), function () {
        if (typeof Api !== 'undefined' && Api.delete) {
          Api.delete('/admin/share-links/' + linkId).then(function (res) {
            if (!res || !res.success) {
              if (typeof Components !== 'undefined' && Components.showToast) {
                Components.showToast(res ? res.message : _t('error'), 'error');
              }
              return;
            }

            if (typeof Components !== 'undefined' && Components.showToast) {
              Components.showToast(_t('share_link.revoke_success'), 'success');
            }

            // Refresh list
            if (_currentContainer) {
              renderList(_currentContainer);
            }
          }).catch(function (err) {
            if (typeof Components !== 'undefined' && Components.showToast) {
              Components.showToast(_t('error') + ': ' + (err.message || err), 'error');
            }
          });
        }
      });
    }
  }

  function renderError(container, message) {
    if (!container) return;
    container.innerHTML = '<div style="padding:var(--space-lg);text-align:center;color:var(--color-error)">' +
      escapeHtml(message) + '</div>';
  }

  function getShareLinkStatus(link) {
    if (link.revoked) return 'revoked';
    if (link.expires_at && new Date(link.expires_at) < new Date()) return 'expired';
    return 'active';
  }

  function getStatusBadgeHtml(status) {
    var cls = {
      active: 'badge-success',
      expired: 'badge-warning',
      revoked: 'badge-error'
    };
    var label = {
      active: _t('share_link.active'),
      expired: _t('share_link.expired'),
      revoked: _t('share_link.revoked')
    };
    return '<span class="badge ' + (cls[status] || 'badge-default') + '">' + escapeHtml(label[status] || status) + '</span>';
  }

  function escapeHtml(s) {
    if (!s) return '';
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  function formatTime(iso) {
    if (!iso) return '-';
    try { return new Date(iso).toLocaleString(); }
    catch (e) { return iso || '-'; }
  }

  function formatExpiry(iso) {
    if (!iso) return _t('share_link.permanent');
    try { return new Date(iso).toLocaleString(); }
    catch (e) { return iso || '-'; }
  }

  function destroy() {
    close();
    if (_currentContainer) {
      _currentContainer.removeEventListener('click', handleListAction);
      _currentContainer.innerHTML = '';
      _currentContainer = null;
    }
  }

  return {
    open: open,
    close: close,
    renderList: renderList,
    destroy: destroy
  };
})();