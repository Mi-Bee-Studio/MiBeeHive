/**
 * Reusable UI components built with Preact + HTM via PreactBridge.
 * Exposes window.Components global with backward-compatible API.
 * Skeleton functions return HTML strings (for innerHTML usage by callers).
 * Toast, Modal, FilterBar, Accordion use Preact internally.
 */
(function () {
  'use strict';

  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;
  var Fragment = PreactBridge.Fragment;

  // ── Helpers ─────────────────────────────────────────────────────────────

  /**
   * Convert a Preact VNode to an HTML string for backward-compatible
   * functions that other modules embed via innerHTML.
   */
  function _toHtml(vnode) {
    var d = document.createElement('div');
    render(vnode, d);
    var s = d.innerHTML;
    render(null, d);
    return s;
  }

  // ── Field Validation (DOM-imperative, no Preact benefit) ────────────────

  function showFieldError(fieldId, message) {
    var input = document.getElementById(fieldId);
    if (!input) return;
    input.setAttribute('aria-invalid', 'true');
    var errId = fieldId + '-error';
    var span = document.getElementById(errId);
    if (!span) {
      span = document.createElement('span');
      span.id = errId;
      span.className = 'text-xs';
      span.style.color = 'var(--color-error)';
      input.parentNode.appendChild(span);
    }
    span.textContent = message;
    input.setAttribute('aria-describedby', errId);
  }

  function clearFieldErrors(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    container.querySelectorAll('[aria-invalid="true"]').forEach(function (el) {
      el.removeAttribute('aria-invalid');
      el.removeAttribute('aria-describedby');
    });
    container.querySelectorAll('.field-error-inline').forEach(function (el) {
      el.remove();
    });
    container.querySelectorAll('[id$="-error"]').forEach(function (el) {
      if (el.classList.contains('text-xs') && el.style.color) {
        el.remove();
      }
    });
  }

  // ── Toast System ────────────────────────────────────────────────────────

  var _toastQueue = [];
  var _activeToasts = [];
  var _maxVisible = 3;
  var _toastRoot = null;
  var _toastSetState = null; // setter for React-like state update

  function _getToastDuration(type, options) {
    var defaults = { success: 3000, warning: 5000, info: 4000 };
    var dur = defaults[type];
    if (type === 'error') dur = null;
    if (dur === undefined) dur = 3000;
    if (typeof options.duration === 'number') {
      dur = options.duration;
    }
    if (type === 'error' && dur !== null) {
      return Math.min(dur, 15000);
    }
    return dur;
  }

  function _getToastZIndex(type) {
    var zMap = { error: 4, warning: 3, info: 2, success: 1 };
    return 300 + (zMap[type] || 1);
  }

  /**
   * Preact ToastItem component with auto-dismiss.
   */
  function ToastItem(props) {
    var ref = useRef(null);
    var timerRef = useRef(null);

    useEffect(function () {
      var duration = props.duration;
      if (duration !== null && duration !== undefined) {
        timerRef.current = setTimeout(function () {
          props.onDismiss(props.id);
        }, duration);
      }
      return function () {
        if (timerRef.current) clearTimeout(timerRef.current);
      };
    }, []);

    function handleClose() {
      if (timerRef.current) clearTimeout(timerRef.current);
      props.onDismiss(props.id);
    }

    function handleRetry() {
      if (typeof props.retry === 'function') {
        props.retry();
        handleClose();
      }
    }

    var msgContent = props.isHtml
      ? html`<span dangerouslySetInnerHTML=${{ __html: props.message }} />`
      : html`<span>${props.message}</span>`;

    return html`
      <div ref=${ref} class="toast toast-${props.type}"
           style="z-index:${_getToastZIndex(props.type)};display:flex;align-items:center">
        <span style="flex:1">${msgContent}</span>
        ${props.retry ? html`
          <button class="toast-retry-btn" onClick=${handleRetry}>
            ${props.retryLabel || t('toast_retry')}
          </button>
        ` : null}
        <button onClick=${handleClose}
                aria-label=${t('close_notification')}
                style="background:none;border:none;color:inherit;cursor:pointer;padding:0 0 0 0.5rem;font-size:1.1rem;line-height:1;opacity:0.7;flex-shrink:0">
          ${'\u00d7'}
        </button>
      </div>
    `;
  }

  /**
   * Preact ToastContainer — renders all active toasts.
   */
  function ToastContainer(props) {
    var toasts = props.toasts;
    var onDismiss = props.onDismiss;

    var dismissAll = toasts.length >= 3;

    return html`
      <div>
        ${toasts.map(function (toast) {
          return html`<${ToastItem} key=${toast.id} ...${toast} onDismiss=${onDismiss} />`;
        })}
        ${dismissAll ? html`
          <button class="toast-dismiss-all" onClick=${props.onDismissAll}>
            ${t('toast_dismiss_all')}
          </button>
        ` : null}
      </div>
    `;
  }

  function _ensureToastRoot() {
    if (!_toastRoot) {
      var container = document.getElementById('toast-container');
      if (!container) return null;
      _toastRoot = container;
    }
    return _toastRoot;
  }

  function _renderToasts() {
    var root = _ensureToastRoot();
    if (!root) return;
    render(html`<${ToastContainer}
      toasts=${_activeToasts}
      onDismiss=${_dismissToast}
      onDismissAll=${_dismissAllToasts}
    />`, root);
  }

  function _dismissToast(id) {
    var idx = -1;
    for (var i = 0; i < _activeToasts.length; i++) {
      if (_activeToasts[i].id === id) { idx = i; break; }
    }
    if (idx !== -1) {
      _activeToasts.splice(idx, 1);
    }
    _showNextQueuedToast();
    _renderToasts();
  }

  function _dismissAllToasts() {
    _toastQueue = [];
    _activeToasts = [];
    _renderToasts();
  }

  function _showNextQueuedToast() {
    if (_toastQueue.length > 0 && _activeToasts.length < _maxVisible) {
      var next = _toastQueue.shift();
      next.duration = _getToastDuration(next.type, next.options);
      _activeToasts.push(next);
    }
  }

  var _toastIdCounter = 0;

  function showToast(message, type, options) {
    type = type || 'success';
    if (typeof options === 'boolean') {
      options = { isHtml: options };
    }
    options = options || {};

    var toastData = {
      id: 'toast-' + (++_toastIdCounter),
      message: message,
      type: type,
      isHtml: !!options.isHtml,
      retry: options.retry || null,
      retryLabel: options.retryLabel || null,
      options: options,
      duration: null
    };
    toastData.duration = _getToastDuration(type, options);

    if (_activeToasts.length >= _maxVisible) {
      _toastQueue.push(toastData);
      return;
    }

    _activeToasts.push(toastData);
    _renderToasts();
  }

  // ── Confirm Modal ───────────────────────────────────────────────────────

  function ConfirmModal(props) {
    var ref = useRef(null);
    var previousFocusRef = useRef(null);

    useEffect(function () {
      previousFocusRef.current = document.activeElement;
      // Focus confirm button
      var confirmBtn = ref.current && ref.current.querySelector('#modal-confirm');
      if (confirmBtn) confirmBtn.focus();
    }, []);

    function close() {
      var root = document.getElementById('modal-root-' + props._modalId);
      if (root) {
        render(null, root);
        root.remove();
      }
      if (previousFocusRef.current && previousFocusRef.current.focus) {
        previousFocusRef.current.focus();
      }
    }

    function handleConfirm() {
      var confirmBtn = ref.current && ref.current.querySelector('#modal-confirm');
      if (typeof props.onConfirm === 'function') props.onConfirm(confirmBtn);
      close();
    }

    function handleOverlayClick(e) {
      if (e.target === ref.current) close();
    }

    function handleKeyDown(e) {
      if (e.key === 'Escape') { close(); return; }
      if (e.key === 'Tab') {
        var focusable = ref.current.querySelectorAll('button');
        if (focusable.length === 0) return;
        var first = focusable[0], last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    }

    return html`
      <div ref=${ref} class="modal-overlay" onClick=${handleOverlayClick} onKeyDown=${handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" aria-labelledby="modal-title">
          <p id="modal-title" class="text-sm mb-6" style="color:var(--color-text)">${props.message}</p>
          <div class="flex justify-end gap-3">
            <button id="modal-cancel" class="btn btn-secondary" onClick=${close}>${t('cancel')}</button>
            <button id="modal-confirm" class="btn btn-primary" onClick=${handleConfirm}>${t('confirm')}</button>
          </div>
        </div>
      </div>
    `;
  }

  function showConfirmModal(message, onConfirm) {
    var modalId = 'cm-' + Math.random().toString(36).substr(2, 6);
    var root = document.createElement('div');
    root.id = 'modal-root-' + modalId;
    document.body.appendChild(root);
    render(html`<${ConfirmModal} message=${message} onConfirm=${onConfirm} _modalId=${modalId} />`, root);
  }

  // ── Generic Modal (createModal) ─────────────────────────────────────────

  function ModalComponent(props) {
    var ref = useRef(null);
    var previousFocusRef = useRef(null);

    useEffect(function () {
      previousFocusRef.current = document.activeElement;

      if (typeof props.onMount === 'function') {
        // Provide the overlay DOM node for event binding
        props.onMount(ref.current);
      }

      // Auto-focus first focusable element if onMount didn't set focus
      if (!ref.current.contains(document.activeElement)) {
        var autoFocusEl = ref.current.querySelector('button, input, select, textarea, [tabindex]:not([tabindex="-1"])');
        if (autoFocusEl) autoFocusEl.focus();
      }
    }, []);

    function close() {
      if (typeof props.onClose === 'function') props.onClose();
      if (previousFocusRef.current && previousFocusRef.current.focus) {
        previousFocusRef.current.focus();
      }
    }

    function handleOverlayClick(e) {
      if (e.target === ref.current) close();
    }

    function handleKeyDown(e) {
      if (e.key === 'Escape') { close(); return; }
      if (e.key === 'Tab') {
        var focusable = ref.current.querySelectorAll('button, input, select, textarea, [tabindex]:not([tabindex="-1"])');
        if (focusable.length === 0) return;
        var first = focusable[0], last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last.focus(); }
        else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first.focus(); }
      }
    }

    var sizeStyle = props.size ? 'max-width:' + props.size : undefined;

    return html`
      <div ref=${ref} class="modal-overlay" onClick=${handleOverlayClick} onKeyDown=${handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true"
             aria-labelledby=${props.titleId} style=${sizeStyle ? { maxWidth: props.size } : undefined}>
          <h3 id=${props.titleId} class="text-base font-semibold mb-4"
              style="color:var(--color-text)">${props.title}</h3>
          <div dangerouslySetInnerHTML=${{ __html: props.bodyHtml }} />
        </div>
      </div>
    `;
  }

  function createModal(config) {
    var titleId = 'modal-title-' + Math.random().toString(36).substr(2, 6);
    var root = document.createElement('div');
    document.body.appendChild(root);

    var closed = false;

    function close() {
      if (closed) return;
      closed = true;
      render(null, root);
      root.remove();
    }

    var overlay = null;

    function onMount(overlayEl) {
      overlay = overlayEl;
      if (typeof config.onMount === 'function') {
        config.onMount(overlayEl);
      }
    }

    render(html`
      <${ModalComponent}
        title=${config.title}
        titleId=${titleId}
        bodyHtml=${config.bodyHtml}
        size=${config.size}
        onMount=${onMount}
        onClose=${close}
      />
    `, root);

    // The overlay ref is set after mount; we need to provide a stable reference.
    // createModal consumers use overlay for querySelector, so we proxy via a getter.
    var overlayProxy = {
      querySelector: function (sel) {
        return root.querySelector(sel);
      },
      querySelectorAll: function (sel) {
        return root.querySelectorAll(sel);
      },
      contains: function (el) {
        return root.contains(el);
      }
    };

    return { overlay: overlayProxy, close: close };
  }

  // ── Skeleton Components ─────────────────────────────────────────────────

  function SkeletonCardComponent() {
    return html`<div class="skeleton skeleton-card"></div>`;
  }

  function SkeletonTableComponent(props) {
    var rows = props.rows || 4;
    var cols = props.cols || 4;
    var widths = ['60%', '80%', '45%', '70%', '55%', '90%'];
    var rowElements = [];
    for (var r = 0; r < rows; r++) {
      var cells = [];
      for (var c = 0; c < cols; c++) {
        var w = widths[c % widths.length];
        cells.push(html`
          <div class="skeleton skeleton-table-cell"
               style="flex:${100 / cols}%;max-width:${w}"></div>
        `);
      }
      rowElements.push(html`<div class="skeleton-table-row">${cells}</div>`);
    }
    return html`<div class="skeleton-table">${rowElements}</div>`;
  }

  function SkeletonTreeComponent(props) {
    var depth = props.depth || 5;
    var widths = ['70%', '55%', '60%', '45%', '65%'];
    var nodes = [];
    for (var i = 0; i < depth; i++) {
      var indent = i > 0 ? (Math.min(i, 3) * 1.25) : 0;
      var w = widths[i % widths.length];
      nodes.push(html`
        <div class="skeleton-tree-node" style="padding-left:${indent}rem">
          <div class="skeleton" style="width:0.75rem;height:0.75rem;border-radius:2px;flex-shrink:0"></div>
          <div class="skeleton" style="width:${w};height:0.75rem;border-radius:var(--radius-sm)"></div>
        </div>
      `);
    }
    return html`<div class="skeleton-tree">${nodes}</div>`;
  }

  function SkeletonTextComponent(props) {
    var lines = props.lines || 3;
    var elements = [];
    for (var i = 0; i < lines; i++) {
      var cls = 'skeleton skeleton-text' + (i === lines - 1 ? ' skeleton-text-short' : '');
      elements.push(html`<div class="${cls}"></div>`);
    }
    return html`<${Fragment}>${elements}<//>`;
  }

  function SkeletonHeadingComponent(props) {
    return html`<div class="skeleton skeleton-heading" style="width:${props.width || '50%'}"></div>`;
  }

  // Backward-compat wrappers returning HTML strings
  function skeletonCard() {
    return _toHtml(html`<${SkeletonCardComponent} />`);
  }

  function skeletonTable(rows, cols) {
    return _toHtml(html`<${SkeletonTableComponent} rows=${rows} cols=${cols} />`);
  }

  function skeletonTree(depth) {
    return _toHtml(html`<${SkeletonTreeComponent} depth=${depth} />`);
  }

  function skeletonText(lines) {
    return _toHtml(html`<${SkeletonTextComponent} lines=${lines} />`);
  }

  function skeletonHeading(width) {
    return _toHtml(html`<${SkeletonHeadingComponent} width=${width || '50%'} />`);
  }

  // ── Empty State ─────────────────────────────────────────────────────────

  function EmptyStateComponent(props) {
    var iconHtml = props.icon || Helpers.ICONS.inbox;
    return html`
      <div class="empty-state" role="status" aria-live="polite">
        <div style="margin-bottom:0.75rem" dangerouslySetInnerHTML=${{ __html: iconHtml }} />
        <p class="text-sm" style="color:var(--color-text-tertiary)">${props.message}</p>
        ${props.description ? html`
          <p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">${props.description}</p>
        ` : null}
        ${props.actionLabel ? html`
          <div style="margin-top:0.75rem">
            <button class="btn btn-primary btn-sm" data-action="empty-state-action">${props.actionLabel}</button>
          </div>
        ` : null}
      </div>
    `;
  }

  function emptyState(config) {
    return _toHtml(html`<${EmptyStateComponent}
      icon=${config.icon}
      message=${config.message}
      description=${config.description}
      actionLabel=${config.actionLabel}
    />`);
  }

  // ── Retry Error ─────────────────────────────────────────────────────────

  function renderRetryError(container, message, retryFn) {
    var root = typeof container === 'string' ? document.getElementById(container) : container;
    if (!root) return;

    function handleRetry() {
      if (typeof retryFn === 'function') retryFn();
    }

    render(html`
      <div class="anim-fade-in" style="padding:2rem;text-align:center">
        <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem"
             dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
        <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${message}</p>
        <button class="btn btn-primary btn-sm retry-btn" onClick=${handleRetry}>${t('error_retry')}</button>
      </div>
    `, root);
  }

  // ── Download Progress ───────────────────────────────────────────────────

  function DownloadProgressComponent(props) {
    return html`
      <div>
        ${props.label ? html`
          <div class="text-xs" style="color:var(--color-text-tertiary);margin-bottom:0.25rem">${props.label}</div>
        ` : null}
        <div class="dl-progress" role="status">
          <div class="dl-progress-bar">
            <div class="dl-progress-fill" id="${props.id}-fill" style="width:0%"></div>
          </div>
          <span class="dl-progress-text" id="${props.id}-text">0%</span>
          <span class="dl-progress-info" id="${props.id}-info" style="display:none"></span>
        </div>
      </div>
    `;
  }

  function downloadProgress(config) {
    return _toHtml(html`<${DownloadProgressComponent} id=${config.id} label=${config.label} />`);
  }

  function updateProgress(id, percent, speed, eta) {
    var fill = document.getElementById(id + '-fill');
    var text = document.getElementById(id + '-text');
    if (!fill || !text) return;
    percent = Math.min(Math.max(0, Math.round(percent)), 100);
    fill.style.width = percent + '%';
    text.textContent = percent + '%';
    if (typeof speed !== 'undefined' || typeof eta !== 'undefined') {
      var info = document.getElementById(id + '-info');
      if (info) {
        var parts = [];
        if (typeof speed !== 'undefined') parts.push(speed);
        if (typeof eta !== 'undefined') parts.push(eta);
        info.textContent = parts.join(' - ');
        info.style.display = '';
      }
    }
  }

  // ── Pagination ──────────────────────────────────────────────────────────

  var _paginationInstances = {};

  function PaginationComponent(props) {
    var ref = useRef(null);
    var loadingRef = useRef(false);
    var offset = props.offset || 0;
    var limit = props.limit || 50;
    var total = props.total || 0;
    var allLoaded = (offset >= total) || (total === 0);
    var start = total > 0 ? 1 : 0;
    var end = Math.min(offset, total);

    function handleLoadMore() {
      if (loadingRef.current) return;
      loadingRef.current = true;
      if (typeof props.onLoadMore === 'function') {
        props.onLoadMore(offset);
      }
    }

    return html`
      <div ref=${ref} class="pagination-bar"
           style="display:flex;align-items:center;justify-content:center;gap:0.75rem;padding:0.75rem 0">
        <span class="pagination-count" style="font-size:0.75rem;color:var(--color-text-tertiary)">
          ${t('showing_range', { start: start, end: end, total: total })}
        </span>
        ${allLoaded ? (offset > 0 ? html`
          <button class="btn btn-ghost pagination-load-more" disabled
                  style="font-size:0.8125rem">${t('all_loaded')}</button>
        ` : null) : html`
          <button class="btn btn-ghost pagination-load-more"
                  style="font-size:0.8125rem"
                  onClick=${handleLoadMore}>${t('load_more')}</button>
        `}
      </div>
    `;
  }

  function renderPagination(containerId, opts) {
    var container = document.getElementById(containerId);
    if (!container) return;
    render(html`<${PaginationComponent}
      offset=${opts.offset}
      limit=${opts.limit}
      total=${opts.total}
      onLoadMore=${opts.onLoadMore}
    />`, container);
  }

  function removePagination(containerId) {
    var container = document.getElementById(containerId);
    if (!container) return;
    render(null, container);
  }

  // ── FilterBar ───────────────────────────────────────────────────────────

  var FilterBar = {
    _instances: {},

    init: function (container, config) {
      var id = config.id;
      var filters = config.filters || [];
      var onChange = config.onChange;

      var initialKey = null;
      for (var i = 0; i < filters.length; i++) {
        if (filters[i].active) { initialKey = filters[i].key; break; }
      }
      if (initialKey === null && filters.length > 0) {
        initialKey = filters[0].key;
      }

      var instanceState = { activeKey: initialKey, onChange: onChange, filters: filters };
      this._instances[id] = instanceState;

      function FilterBarComponent() {
        var state = useState(initialKey);
        var active = state[0];
        var setActive = state[1];

        var handleClick = useCallback(function (key) {
          setActive(key);
          instanceState.activeKey = key;
          if (typeof onChange === 'function') onChange(key);
        }, [onChange]);

        return html`
          <div class="filter-bar">
            ${filters.map(function (f) {
              return html`
                <button key=${f.key}
                        class=${'filter-btn' + (f.key === active ? ' active' : '')}
                        onClick=${function () { handleClick(f.key); }}>
                  ${f.label}
                </button>
              `;
            })}
          </div>
        `;
      }

      render(html`<${FilterBarComponent} />`, container);

      // Store container for cleanup
      instanceState._container = container;

      return id;
    },

    setActive: function (instanceId, key) {
      var instance = this._instances[instanceId];
      if (!instance) return;
      // Re-render with new active key
      var filters = instance.filters;
      var onChange = instance.onChange;
      instance.activeKey = key;

      function FilterBarUpdate() {
        return html`
          <div class="filter-bar">
            ${filters.map(function (f) {
              return html`
                <button key=${f.key}
                        class=${'filter-btn' + (f.key === key ? ' active' : '')}
                        onClick=${function () {
                          instance.activeKey = f.key;
                          if (typeof onChange === 'function') onChange(f.key);
                          // Re-render this instance
                          render(html`<${FilterBarUpdate} />`, instance._container);
                        }}>
                  ${f.label}
                </button>
              `;
            })}
          </div>
        `;
      }

      render(html`<${FilterBarUpdate} />`, instance._container);
    },

    getActive: function (instanceId) {
      var instance = this._instances[instanceId];
      return instance ? instance.activeKey : null;
    },

    destroy: function (instanceId) {
      var instance = this._instances[instanceId];
      if (!instance) return;
      if (instance._container) {
        render(null, instance._container);
      }
      delete this._instances[instanceId];
    }
  };

  // ── Accordion ───────────────────────────────────────────────────────────

  var Accordion = {
    _state: {},

    init: function (container, config) {
      var accordionId = config.id;
      var sections = config.sections || [];
      var singleOpen = config.singleOpen !== false;
      var self = this;

      var openSections = {};
      this._state[accordionId] = {
        singleOpen: singleOpen,
        openSections: openSections,
        sections: sections,
        _container: container
      };

      function AccordionRoot() {
        var state = useState({});
        var forceUpdate = state[1];

        function toggle(sectionId) {
          var isOpen = !!self._state[accordionId].openSections[sectionId];

          if (self._state[accordionId].singleOpen && !isOpen) {
            var openIds = Object.keys(self._state[accordionId].openSections);
            for (var i = 0; i < openIds.length; i++) {
              delete self._state[accordionId].openSections[openIds[i]];
            }
          }

          if (isOpen) {
            delete self._state[accordionId].openSections[sectionId];
          } else {
            self._state[accordionId].openSections[sectionId] = true;
          }
          forceUpdate({});
        }

        return html`
          <div class="accordion" data-accordion-id=${accordionId}>
            ${sections.map(function (section) {
              var isOpen = !!self._state[accordionId].openSections[section.id];
              return html`
                <div key=${section.id} class=${'accordion-section' + (isOpen ? ' open' : '')}
                     data-section-id=${section.id}>
                  <button class="accordion-header"
                          id=${'accordion-header-' + section.id}
                          aria-expanded=${isOpen ? 'true' : 'false'}
                          aria-controls=${'accordion-body-' + section.id}
                          onClick=${function () { toggle(section.id); }}>
                    <span class="accordion-chevron">
                      <svg aria-hidden="true" width="16" height="16" fill="none"
                           stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
                      </svg>
                    </span>
                    ${section.icon ? html`
                      <span class="accordion-icon" dangerouslySetInnerHTML=${{ __html: section.icon }} />
                    ` : null}
                    <span class="accordion-title">${section.title}</span>
                    ${typeof section.count === 'number' ? html`
                      <span class="accordion-badge">${String(section.count)}</span>
                    ` : null}
                  </button>
                  <div class="accordion-body" id=${'accordion-body-' + section.id}
                       role="region" aria-labelledby=${'accordion-header-' + section.id}>
                    <div class="accordion-body-inner"
                         dangerouslySetInnerHTML=${{ __html: section.content || '' }} />
                  </div>
                </div>
              `;
            })}
          </div>
        `;
      }

      // Store the component constructor for re-rendering
      this._state[accordionId]._render = function () {
        render(html`<${AccordionRoot} />`, container);
      };

      render(html`<${AccordionRoot} />`, container);
    },

    _toggle: function (accordionId, sectionId) {
      var state = this._state[accordionId];
      if (!state || !state._render) return;
      // State mutation triggers re-render via the component's internal useState
      state._render();
    },

    _setSectionOpen: function (accordionId, sectionId, open) {
      var state = this._state[accordionId];
      if (!state) return;
      if (open) {
        state.openSections[sectionId] = true;
      } else {
        delete state.openSections[sectionId];
      }
    },

    update: function (accordionId, sectionId, newContent) {
      var state = this._state[accordionId];
      if (!state) return;

      // Update section content in data model
      for (var i = 0; i < state.sections.length; i++) {
        if (state.sections[i].id === sectionId) {
          state.sections[i].content = newContent;
          break;
        }
      }

      // Re-render if we have the render function
      if (state._render) {
        state._render();
      }
    },

    getOpenSection: function (accordionId) {
      var state = this._state[accordionId];
      if (!state) return null;
      var openIds = Object.keys(state.openSections);
      return openIds.length > 0 ? openIds[0] : null;
    },

    destroy: function (accordionId) {
      var state = this._state[accordionId];
      if (!state) return;
      if (state._container) {
        render(null, state._container);
      }
      delete this._state[accordionId];
    }
  };

  // ── ActionMenu ─────────────────────────────────────────────────────────

  function ActionMenu(props) {
    var items = props.items || [];
    var align = props.align || 'right';

    var openState = useState(false);
    var isOpen = openState[0];
    var setOpen = openState[1];

    var containerRef = useRef(null);
    var menuRef = useRef(null);

    function toggle() {
      setOpen(!isOpen);
    }

    function close() {
      setOpen(false);
    }

    function handleItemClick(item) {
      close();
      if (typeof item.onClick === 'function') item.onClick();
    }

    useEffect(function () {
      if (!isOpen) return;

      function onClickOutside(e) {
        if (containerRef.current && !containerRef.current.contains(e.target)) {
          close();
        }
      }

      function onEscape(e) {
        if (e.key === 'Escape') close();
      }

      document.addEventListener('click', onClickOutside);
      document.addEventListener('keydown', onEscape);

      // Focus first menu item for accessibility
      if (menuRef.current) {
        var firstItem = menuRef.current.querySelector('.action-menu-item');
        if (firstItem) firstItem.focus();
      }

      return function () {
        document.removeEventListener('click', onClickOutside);
        document.removeEventListener('keydown', onEscape);
      };
    }, [isOpen]);

    return html`
      <div ref=${containerRef} style="position:relative">
        <button class="action-menu-trigger"
                aria-haspopup="menu"
                aria-expanded=${isOpen ? 'true' : 'false'}
                onClick=${toggle}>
          <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
            <circle cx="8" cy="3" r="1.5" />
            <circle cx="8" cy="8" r="1.5" />
            <circle cx="8" cy="13" r="1.5" />
          </svg>
        </button>
        ${isOpen ? html`
          <div ref=${menuRef} class="action-menu" role="menu"
               style=${align === 'left' ? 'left:0' : 'right:0'}>
            ${items.map(function (item, i) {
              return html`
                <button key=${i}
                        class=${'action-menu-item' + (item.danger ? ' danger' : '')}
                        role="menuitem"
                        onClick=${function () { handleItemClick(item); }}>
                  ${item.label}
                </button>
              `;
            })}
          </div>
        ` : null}
      </div>
    `;
  }

  // ── Global API ──────────────────────────────────────────────────────────

  window.Components = {
    showFieldError: showFieldError,
    clearFieldErrors: clearFieldErrors,
    showConfirmModal: showConfirmModal,
    showToast: showToast,
    skeletonCard: skeletonCard,
    skeletonTable: skeletonTable,
    skeletonTree: skeletonTree,
    skeletonText: skeletonText,
    skeletonHeading: skeletonHeading,
    renderPagination: renderPagination,
    removePagination: removePagination,
    renderRetryError: renderRetryError,
    createModal: createModal,
    FilterBar: FilterBar,
    Accordion: Accordion,
    emptyState: emptyState,
    downloadProgress: downloadProgress,
    updateProgress: updateProgress,
    ActionMenu: ActionMenu
  };
})();
