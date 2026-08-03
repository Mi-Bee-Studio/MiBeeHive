// Expose to global scope for module system
(function () {
  'use strict';

  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  // ── Tab configuration ────────────────────────────────────────────────────────
  var TAB_ITEMS = [
    { hash: '#/external-service/channels', i18nKey: 'ext_service.tab_channels' },
    { hash: '#/external-service/upload', i18nKey: 'ext_service.tab_upload' },
    { hash: '#/external-service/links', i18nKey: 'ext_service.tab_links' },
    { hash: '#/external-service/endpoints', i18nKey: 'ext_service.tab_endpoints' }
  ];

  // ── Channels & Views tab ──────────────────────────────────────────────────────
  function ChannelsTab() {
    var _data = useState(null);
    var data = _data[0], setData = _data[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var mountedRef = useRef(true);

    useEffect(function () {
      mountedRef.current = true;
      Api.get('/admin/channels').then(function (r) {
        if (!mountedRef.current) return;
        if (!r || !r.success) {
          setError((r && r.message) || t('error'));
          setLoading(false);
          return;
        }
        setData(r.data || []);
        setLoading(false);
      }).catch(function (err) {
        if (!mountedRef.current) return;
        setError(err.message || t('error'));
        setLoading(false);
      });
      return function () { mountedRef.current = false; };
    }, []);

    if (loading) {
      return html`<div dangerouslySetInnerHTML=${{ __html: Helpers.loadingSpinner() }} />`;
    }
    if (error) {
      return html`<div dangerouslySetInnerHTML=${{ __html: Helpers.errorMessage(error) }} />`;
    }
    if (!data || data.length === 0) {
      return html`<div dangerouslySetInnerHTML=${{ __html: Components.emptyState({
        message: t('ext_service.no_channels'),
        description: t('ext_service.no_channels_desc')
      })}} />`;
    }

    return html`
      <div class="channels-list">
        ${data.map(function (channel) {
          return html`
            <div class="card" style="padding:1rem;margin-bottom:0.75rem">
              <div class="flex items-center justify-between">
                <div>
                  <h4 class="text-base font-medium" style="color:var(--color-text)">${channel.name || channel.slug}</h4>
                  <div class="text-sm" style="color:var(--color-text-secondary)">
                    <span class="badge ${channel.type === 'public' ? 'badge-success' : 'badge-default'}">${t(channel.type === 'public' ? 'channel_public' : 'channel_internal')}</span>
                    <span style="margin-left:0.5rem">${t('ext_service.channel_mount')}: ${channel.mount_point || '/'}</span>
                  </div>
                </div>
                <a href="#/channels/${channel.id}" class="btn btn-ghost btn-sm">${t('ext_service.manage_views')}</a>
              </div>
            </div>
          `;
        })}
      </div>
    `;
  }

  // ── Manual Upload tab ───────────────────────────────────────────────────────
  function UploadTab() {
    var _dragging = useState(false);
    var dragging = _dragging[0], setDragging = _dragging[1];
    var _uploading = useState(false);
    var uploading = _uploading[0], setUploading = _uploading[1];
    var _files = useState([]);
    var files = _files[0], setFiles = _files[1];

    function handleDragOver(e) {
      e.preventDefault();
      setDragging(true);
    }

    function handleDragLeave(e) {
      e.preventDefault();
      setDragging(false);
    }

    function handleDrop(e) {
      e.preventDefault();
      setDragging(false);
      var droppedFiles = e.dataTransfer.files;
      if (droppedFiles.length === 0) return;

      setUploading(true);
      var uploadPromises = [];
      for (var i = 0; i < droppedFiles.length; i++) {
        var file = droppedFiles[i];
        uploadPromises.push(uploadFile(file));
      }

      Promise.all(uploadPromises).then(function () {
        setUploading(false);
        Components.showToast(t('ext_service.upload_success'), 'success');
        setFiles([]);
      }).catch(function (err) {
        setUploading(false);
        Components.showToast(t('ext_service.upload_error') + ': ' + err.message, 'error');
      });
    }

    function uploadFile(file) {
      return new Promise(function (resolve, reject) {
        var xhr = new XMLHttpRequest();
        xhr.open('PUT', '/webdav/public/default/' + encodeURIComponent(file.name));
        xhr.onload = function () {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve();
          } else {
            reject(new Error(xhr.statusText || 'Upload failed'));
          }
        };
        xhr.onerror = function () {
          reject(new Error('Network error'));
        };
        xhr.send(file);
      });
    }

    return html`
      <div>
        <div
          class="upload-dropzone ${dragging ? 'upload-dropzone-active' : ''}"
          onDragOver=${handleDragOver}
          onDragLeave=${handleDragLeave}
          onDrop=${handleDrop}
          style="border:2px dashed var(--color-border);border-radius:var(--radius-md);padding:3rem;text-align:center;${dragging ? 'border-color:var(--color-primary);background:var(--color-bg-tertiary)' : ''}">
          ${uploading
            ? html`<div class="text-sm" style="color:var(--color-text-secondary)">${t('ext_service.upload_progress')}</div>`
            : html`
              <div class="text-lg mb-2" style="color:var(--color-text-tertiary)">📁</div>
              <div class="text-sm" style="color:var(--color-text-secondary)">${t('ext_service.upload_drop')}</div>
            `
          }
        </div>
        <div style="margin-top:1rem">
          <div class="flex items-center gap-2 mb-2">
            <button class="btn btn-primary btn-sm" disabled=${uploading}>${t('ext_service.create_folder')}</button>
            <button class="btn btn-ghost btn-sm" disabled=${uploading}>${t('common_delete')}</button>
          </div>
          <div class="text-xs" style="color:var(--color-text-tertiary)">
            ${t('ext_service.upload_note')}
          </div>
        </div>
      </div>
    `;
  }

  // ── Share Links tab ───────────────────────────────────────────────────────────
  function ShareLinksTab() {
    var containerRef = useRef(null);

    useEffect(function () {
      if (containerRef.current && window.ShareLinkDialog && window.ShareLinkDialog.renderList) {
        window.ShareLinkDialog.renderList(containerRef.current);
      }
    }, []);

    return html`<div ref=${containerRef}></div>`;
  }

  // ── Protocol Endpoints tab ─────────────────────────────────────────────────────
  function EndpointsTab() {
    var endpoints = [
      { type: 'apt', label: t('ext_service.apt_endpoint'), url: 'deb http://localhost:9090/apt ./ main' },
      { type: 'pypi', label: t('ext_service.pypi_endpoint'), url: '[global]\nindex-url = http://localhost:9090/pypi/simple' },
      { type: 'repo', label: t('ext_service.repo_endpoint'), url: 'http://localhost:9090/repo/index.json' }
    ];

    return html`
      <div>
        ${endpoints.map(function (ep) {
          var _copied = useState(false);
          var copied = _copied[0], setCopied = _copied[1];

          function handleCopy() {
            Helpers.copyToClipboard(ep.url).then(function () {
              setCopied(true);
              Components.showToast(t('common_copied'), 'success');
              setTimeout(function () { setCopied(false); }, 2000);
            });
          }

          return html`
            <div class="card" style="padding:1rem;margin-bottom:0.75rem">
              <div class="flex items-center justify-between mb-2">
                <span class="text-sm font-medium" style="color:var(--color-text)">${ep.label}</span>
                <button
                  class="btn btn-ghost btn-sm copy-config-btn"
                  onClick=${handleCopy}
                  style="${copied ? 'background:var(--color-success);color:var(--color-text-inverse)' : ''}">
                  ${copied ? '✓' : t('ext_service.copy_config')}
                </button>
              </div>
              <code class="text-xs block p-2" style="background:var(--color-bg-tertiary);border-radius:var(--radius-sm);word-break:break-all;white-space:pre-wrap;color:var(--color-text-secondary)">
                ${Helpers.escapeHtml(ep.url)}
              </code>
            </div>
          `;
        })}
      </div>
    `;
  }

  // ── Main external-service page component ───────────────────────────────────────
  function ExternalServicePage(props) {
    var _activeTab = useState(props.subtab || 'channels');
    var activeTab = _activeTab[0], setActiveTab = _activeTab[1];
    var mountedRef = useRef(true);

    useEffect(function () {
      mountedRef.current = true;
      return function () { mountedRef.current = false; };
    }, []);

    function renderTabContent() {
      switch (activeTab) {
        case 'channels':
          return html`<${ChannelsTab} />`;
        case 'upload':
          return html`<${UploadTab} />`;
        case 'links':
          return html`<${ShareLinksTab} />`;
        case 'endpoints':
          return html`<${EndpointsTab} />`;
        default:
          return html`<${ChannelsTab} />`;
      }
    }

    return html`
      <div class="p-4 md:p-6 max-w-4xl mx-auto">
        <div class="mb-4">
          <h2 class="text-lg font-semibold mb-4" style="color:var(--color-text)">${t('ext_service.title')}</h2>
          <div dangerouslySetInnerHTML=${{ __html: Components.moduleTabs(TAB_ITEMS, 'ext_service.tab_' + activeTab) }} />
        </div>
        <div class="tab-content">
          ${renderTabContent()}
        </div>
      </div>
    `;
  }

  // ── Public API ───────────────────────────────────────────────────────────────
  function render(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${ExternalServicePage} subtab=${params.subtab || 'channels'} />`, app);
  }

  function destroy() {
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  // Expose to global scope
  if (typeof globalThis !== 'undefined') {
    globalThis.ExternalService = {
      render: render,
      destroy: destroy
    };
  }
})();