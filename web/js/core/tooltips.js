// Module: core/tooltips — Reusable tooltip system
// Show on hover with 300ms delay, auto-position with viewport-edge flip
// Supports data-tooltip attribute pattern + programmatic Tooltips.show()

const Tooltips = (function () {
  'use strict';

  var _activeEl = null;
  var _anchorEl = null;
  var _timer = null;

  /**
   * Show a tooltip anchored to an element.
   * @param {Element} anchorEl - The element to anchor to
   * @param {string} content - HTML content of the tooltip
   * @param {'top'|'bottom'|'left'|'right'} [position='top'] - Preferred position
   * @returns {Element} The tooltip DOM element
   */
  function show(anchorEl, content, position) {
    hide();
    position = position || 'top';

    var tooltip = document.createElement('div');
    tooltip.className = 'tooltip';
    tooltip.setAttribute('role', 'tooltip');
    tooltip.innerHTML = content;
    document.body.appendChild(tooltip);

    var anchorRect = anchorEl.getBoundingClientRect();
    var tooltipRect = tooltip.getBoundingClientRect();
    var padding = 8;

    var top, left;
    switch (position) {
      case 'bottom':
        top = anchorRect.bottom + 6;
        left = anchorRect.left + (anchorRect.width - tooltipRect.width) / 2;
        break;
      case 'left':
        top = anchorRect.top + (anchorRect.height - tooltipRect.height) / 2;
        left = anchorRect.left - tooltipRect.width - 6;
        break;
      case 'right':
        top = anchorRect.top + (anchorRect.height - tooltipRect.height) / 2;
        left = anchorRect.right + 6;
        break;
      default: // 'top'
        top = anchorRect.top - tooltipRect.height - 6;
        left = anchorRect.left + (anchorRect.width - tooltipRect.width) / 2;
    }

    // Auto-flip if near viewport edge
    if (left < padding) left = padding;
    if (left + tooltipRect.width > window.innerWidth - padding) {
      left = window.innerWidth - tooltipRect.width - padding;
    }
    if (top < padding) {
      if (position === 'top') {
        // Flip to bottom
        top = anchorRect.bottom + 6;
        tooltip.setAttribute('data-tooltip-pos', 'bottom');
      } else {
        top = padding;
      }
    } else if (top + tooltipRect.height > window.innerHeight - padding) {
      if (position === 'bottom') {
        // Flip to top
        top = anchorRect.top - tooltipRect.height - 6;
        tooltip.setAttribute('data-tooltip-pos', 'top');
      } else {
        top = window.innerHeight - tooltipRect.height - padding;
      }
    }

    tooltip.style.left = left + 'px';
    tooltip.style.top = top + 'px';

    if (!tooltip.hasAttribute('data-tooltip-pos')) {
      tooltip.setAttribute('data-tooltip-pos', position);
    }

    _activeEl = tooltip;
    _anchorEl = anchorEl;

    // Hide on Escape
    function onKeydown(e) {
      if (e.key === 'Escape') { hide(); document.removeEventListener('keydown', onKeydown); }
    }
    document.addEventListener('keydown', onKeydown);

    return tooltip;
  }

  /**
   * Hide the active tooltip immediately.
   */
  function hide() {
    if (_timer) {
      clearTimeout(_timer);
      _timer = null;
    }
    if (_activeEl) {
      if (_activeEl.parentNode) {
        _activeEl.parentNode.removeChild(_activeEl);
      }
      _activeEl = null;
      _anchorEl = null;
    }
  }

  /**
   * Initialize event delegation for [data-tooltip] elements.
   * Call once after DOM is ready.
   */
  function init() {
    // Use a shared hover timeout to avoid flicker
    document.addEventListener('mouseover', function (e) {
      var el = e.target.closest('[data-tooltip]');
      if (!el) return;
      // Already showing for this element — skip
      if (el === _anchorEl) return;

      var content = el.getAttribute('data-tooltip');
      if (!content) return;
      var position = el.getAttribute('data-tooltip-pos') || 'top';

      if (_timer) clearTimeout(_timer);
      _timer = setTimeout(function () {
        show(el, content, position);
      }, 300);
    });

    document.addEventListener('mouseout', function (e) {
      var el = e.target.closest('[data-tooltip]');
      var related = e.relatedTarget;

      // If we moved from one tooltip element to another within the same anchor, don't hide
      if (_anchorEl) {
        // Still inside anchor? keep showing
        if (_anchorEl.contains(related)) return;
        // Moving to tooltip itself? keep showing
        if (_activeEl && _activeEl.contains(related)) return;
      }

      // If we're still inside a [data-tooltip] element (nested), don't hide
      if (el && el.contains(related)) return;

      hide();
    });

    // Hide tooltip on scroll or resize
    document.addEventListener('scroll', function () { hide(); }, true);
    window.addEventListener('resize', function () { hide(); });
  }

  return {
    show: show,
    hide: hide,
    init: init
  };
})();
