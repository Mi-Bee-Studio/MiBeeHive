// Module: modules/file-center — File Center (default home page)
//
// Stub for T24. The full File Center page (search, filters, bulk actions,
// WebDAV card, view switch) lands in a later task. For now this renders a
// placeholder so the new IA (File Center as default home) is navigable.
var FileCenter = (function () {
  'use strict';

  function render(params, query, signal) {
    var main = document.getElementById('main-content');
    if (!main) return;
    main.textContent = ''; // clear
    var placeholder = document.createElement('div');
    placeholder.className = 'p-8 text-center text-gray-500';
    placeholder.textContent = t('file_center.coming_soon') || 'File Center — coming in T24';
    main.appendChild(placeholder);
  }

  function destroy() {
    var main = document.getElementById('main-content');
    if (main) main.textContent = '';
  }

  return { render: render, destroy: destroy };
})();