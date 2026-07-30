const ContainersImages = (function() {
  'use strict';

  // Private state
  var _init = false;         // whether initial load has completed
  var _pulling = false;      // whether an image pull is in progress
  var _imageKeys = '';       // cached image ID keys for change detection
  var _cachedImages = null;  // cached image data for instant render

  // Reusable style strings (CSS variable references)
  var _styleTextTertiary = 'color:var(--color-text-tertiary)';

  // Helpers shorthand
  var _esc = Helpers.escapeHtml;
  var _formatTime = Helpers.formatTime;

  // Generate sub-navigation tabs for the containers section
  function _subNav() {
    return '<nav class="module-tabs">' +
      '<a href="#/containers" class="module-tab">' + t('nav_containers') + '</a>' +
      '<a href="#/containers/images" class="module-tab active">' + t('nav_containers_images') + '</a>' +
      '<a href="#/containers/templates" class="module-tab">' + t('nav_containers_templates') + '</a>' +
      '<a href="#/containers/registries" class="module-tab">' + t('nav_registries') + '</a>' +
      '</nav>';
  }

  // Format megabytes to human-readable size string
  function _formatSize(sizeMb) {
    if (!sizeMb || sizeMb <= 0) return '0 MB';
    return sizeMb >= 1024
      ? (sizeMb / 1024).toFixed(1) + ' GB'
      : sizeMb.toFixed(1) + ' MB';
  }

  // Generate a single table row for an image
  function _imageRow(img) {
    var tags = img.repo_tags && img.repo_tags.length
      ? img.repo_tags.map(_esc).join(', ')
      : '&lt;none&gt;';

    return '<tr data-id="' + _esc(img.id) + '">' +
      '<td><div class="font-medium" style="color:var(--color-text)">' + tags + '</div>' +
        '<span class="text-xs" style="' + _styleTextTertiary + '">' + _esc(img.id ? img.id.substring(0, 19) : '') + '</span></td>' +
      '<td><span class="text-xs" style="' + _styleTextTertiary + '">' + _formatSize(img.size_mb) + '</span></td>' +
      '<td><span class="text-xs" style="' + _styleTextTertiary + '">' + _formatTime(img.created_at) + '</span></td>' +
      '<td style="text-align:right"><button class="btn btn-danger d" data-id="' + _esc(img.id) + '" data-t="' + _esc(tags) + '">' + t('delete') + '</button></td>' +
      '</tr>';
  }

  // Bind click handlers for image delete buttons via event delegation
  function _bindActions(container) {
    container.onclick = function(e) {
      var btn = e.target.closest('.d');
      if (!btn) return;

      showConfirmModal(t('image_delete_confirm', {name: btn.dataset.t}), function(modalBtn) {
        var originalText = modalBtn.textContent;
        modalBtn.disabled = true;
        modalBtn.textContent = '...';

        Api.delete('/admin/images/' + encodeURIComponent(btn.dataset.id)).then(function(res) {
          if (!res || !res.success) {
            modalBtn.disabled = false;
            modalBtn.textContent = originalText;
            Components.showToast((res && res.message) || t('error'), 'error');
            return;
          }
          Components.showToast(t('image_deleted'), 'success');
          _loadImages();
        });
      });
    };
  }

  // Render the image list table or empty state with CTA
  function _renderImages(container, images) {
    if (!images.length) {
      container.innerHTML =
        '<div class="empty-state">' +
          '<p class="text-sm" style="' + _styleTextTertiary + '">' + t('images_empty') + '</p>' +
          '<div style="margin-top:0.75rem">' +
            '<button class="btn btn-primary btn-sm" data-action="cta-pull-image">' + t('cta_pull_image') + '</button>' +
          '</div>' +
          '<p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">' + t('cta_pull_image_desc') + '</p>' +
        '</div>';

      var cta = container.querySelector('[data-action="cta-pull-image"]');
      if (cta) cta.addEventListener('click', function() {
        var inp = document.getElementById('ci-pi');
        if (inp) inp.focus();
      });
      return;
    }

    // Incremental DOM: create table on first render, diff rows on subsequent updates
    var tbody = container.querySelector('tbody');
    if (!tbody) {
      container.innerHTML = '';
      var table = document.createElement('table');
      table.innerHTML = '<thead><tr>' +
        '<th>' + t('image_tags') + '</th>' +
        '<th>' + t('image_size') + '</th>' +
        '<th>' + t('image_created') + '</th>' +
        '<th style="text-align:right">' + t('actions') + '</th>' +
        '</tr></thead><tbody></tbody>';
      container.appendChild(table);
      tbody = table.querySelector('tbody');
      _bindActions(container);
    }

    // Build map of existing rows by data-id
    var existingRows = {};
    var rows = tbody.querySelectorAll('tr[data-id]');
    for (var i = 0; i < rows.length; i++) {
      existingRows[rows[i].getAttribute('data-id')] = rows[i];
    }

    // Track IDs present in new data
    var imageIds = {};

    images.forEach(function(img) {
      imageIds[img.id] = true;
      var existing = existingRows[img.id];
      if (existing) {
        // Update existing row in-place
        var tags = img.repo_tags && img.repo_tags.length
          ? img.repo_tags.map(_esc).join(', ')
          : '&lt;none&gt;';
        existing.cells[0].querySelector('.font-medium').innerHTML = tags;
        existing.cells[0].querySelector('.text-xs').textContent = img.id ? img.id.substring(0, 19) : '';
        existing.cells[1].querySelector('.text-xs').textContent = _formatSize(img.size_mb);
        existing.cells[2].querySelector('.text-xs').textContent = _formatTime(img.created_at);
        var delBtn = existing.cells[3].querySelector('.d');
        if (delBtn) {
          delBtn.setAttribute('data-id', img.id);
          delBtn.setAttribute('data-t', tags);
        }
      } else {
        // New row — create and append
        var temp = document.createElement('div');
        temp.innerHTML = '<table><tbody>' + _imageRow(img) + '</tbody></table>';
        tbody.appendChild(temp.querySelector('tr'));
      }
    });

    // Remove rows for images no longer present
    for (var id in existingRows) {
      if (existingRows.hasOwnProperty(id) && !imageIds[id]) {
        existingRows[id].remove();
      }
    }
  }

  // Load images from API and re-render if data changed
  async function _loadImages() {
    var el = document.getElementById('ci-l');
    if (!el) return;

    var res = await Api.get('/admin/images', {silent: true});
    if (!res || !res.success) {
      if (!_init && res && res.message) {
        // Localize the backend "Docker is not available" message; fall back to
        // the raw message for any other error.
        var msg = /docker/i.test(res.message) ? t('error_docker_unavailable') : _esc(res.message);
        el.innerHTML = '<div class="empty-state"><p class="text-sm" style="color:var(--color-error)">' + msg + '</p></div>';
      }
      _init = true;
      return;
    }

    var images = res.data || [];
    _cachedImages = images;
    var keys = images.map(function(img) { return img.id; }).join('|');
    if (!_init || keys !== _imageKeys) {
      _imageKeys = keys;
      _renderImages(el, images);
    }
    _init = true;
  }

  // Pull a new Docker image by name
  function _pullImage() {
    if (_pulling) return;

    var inp = document.getElementById('ci-pi');
    var btn = document.getElementById('ci-pb');
    if (!inp || !btn) return;

    var imageName = inp.value.trim();
    if (!imageName) { inp.focus(); return; }

    _pulling = true;
    var originalText = btn.textContent;
    btn.disabled = true;
    btn.textContent = t('image_pulling');

    Api.post('/admin/images/pull', {image: imageName}).then(function(res) {
      _pulling = false;
      btn.disabled = false;
      btn.textContent = originalText;

      if (!res || !res.success) {
        Components.showToast((res && res.message) || t('error'), 'error');
        return;
      }
      Components.showToast(t('image_pull_started'), 'success');
      inp.value = '';
      setTimeout(_loadImages, 3000);
    });
  }

  // Render the images management page
  function render() {
    destroy();

    var mc = document.getElementById('main-content');
    if (!mc) return;

    mc.innerHTML =
      '<div class="p-4 md:p-6 max-w-7xl mx-auto">' +
        _subNav() +
        '<h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">' + t('images_title') + '</h2>' +
        '<div class="card mb-4" style="padding:1rem">' +
          '<div class="flex gap-2">' +
            '<input id="ci-pi" class="input flex-1" placeholder="' + t('placeholder_image') + '" style="font-size:0.8125rem">' +
            '<button id="ci-pb" class="btn btn-primary btn-sm" style="white-space:nowrap">' + t('image_pull') + '</button>' +
          '</div>' +
        '</div>' +
        '<div class="card"><div id="ci-l">' + (_cachedImages && _init ? '' : Helpers.loadingSpinner()) + '</div></div>' +
      '</div>';
    if (_cachedImages && _init) _renderImages(document.getElementById('ci-l'), _cachedImages);

    // Bind pull button and start loading
    var pullBtn = document.getElementById('ci-pb');
    if (pullBtn) pullBtn.addEventListener('click', _pullImage);

    _loadImages();
    App.addTimer(setInterval(_loadImages, 10000));
  }

  // Cleanup: clear timers, reset state
  function destroy() {
    App.clearAllTimers();
    _init = false;
    _pulling = false;
    _imageKeys = '';
  }

  return { render: render, destroy: destroy };
})();
