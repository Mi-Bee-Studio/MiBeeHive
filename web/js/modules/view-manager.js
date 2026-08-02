// Module: modules/view-manager — View Manager with CRUD operations
//
// Manages virtual views with hierarchical node trees (folders, file references,
// and rule_folder nodes with match criteria). Left panel shows the node tree,
// right panel shows node details/edit form.
const ViewManager = (function () {
  'use strict';

  var html = PreactBridge.html;
  var h = PreactBridge.h;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  // Node types
  var NODE_TYPES = ['folder', 'file_ref', 'rule_folder'];

  // Rule fields for rule_folder
  var RULE_FIELDS = ['os', 'arch', 'source_type', 'category', 'version'];

  // ── Tree Node Component ─────────────────────────────────────────────────────
  function TreeNode(props) {
    var node = props.node;
    var level = props.level || 0;
    var isSelected = props.isSelected;
    var onSelect = props.onSelect;
    var expanded = props.expanded || false;
    var onToggle = props.onToggle;

    var hasChildren = node.children && node.children.length > 0;
    var isFolder = node.type === 'folder' || node.type === 'rule_folder';
    var icon = isFolder 
      ? (node.type === 'rule_folder' 
          ? '<span style="color:var(--color-accent)">⚙</span>' 
          : '<span style="color:var(--color-primary)">📁</span>')
      : '<span style="color:var(--color-text-quaternary)">📄</span>';

    var handleClick = function (e) {
      e.stopPropagation();
      onSelect(node);
    };

    var handleToggle = function (e) {
      e.stopPropagation();
      if (hasChildren) onToggle(node);
    };

    var chevron = hasChildren
      ? html`<button type="button" class="tree-chevron" 
                   onClick=${handleToggle}
                   aria-expanded=${expanded ? 'true' : 'false'}
                   aria-label="${expanded ? 'Collapse' : 'Expand'}">
        ${expanded ? '▼' : '▶'}
      </button>`
      : html`<span class="tree-chevron-placeholder"></span>`;

    return html`
      <div class="tree-node ${isSelected ? 'selected' : ''}" 
           style="padding-left:${level * 1.5}rem"
           onClick=${handleClick}
           data-node-id=${node.id}
           role="treeitem"
           aria-selected=${isSelected ? 'true' : 'false'}>
        ${chevron}
        <span class="tree-icon" dangerouslySetInnerHTML=${{ __html: icon }} />
        <span class="tree-label">${node.name}</span>
        ${node.type === 'rule_folder' && node.materialized_count !== undefined 
          ? html`<span class="tree-count">(${node.materialized_count})</span>` 
          : null}
      </div>
      ${hasChildren && expanded 
        ? html`<div class="tree-children">
          ${node.children.map(function (child) {
            return html`<${TreeNode} key=${child.id} node=${child} 
                                    level=${level + 1}
                                    isSelected=${false}
                                    onSelect=${onSelect}
                                    expanded=${false}
                                    onToggle=${onToggle} />`;
          })}
        </div>` 
        : null}
    `;
  }

  // ── Rule Config Form Component ───────────────────────────────────────────────
  function RuleConfigForm(props) {
    var config = props.config || {};
    var onChange = props.onChange;

    var conditions = useMemo(function () {
      var conds = [];
      for (var field in config) {
        if (config.hasOwnProperty(field) && RULE_FIELDS.indexOf(field) !== -1) {
          conds.push({ field: field, value: config[field] });
        }
      }
      return conds;
    }, [config]);

    var handleAddCondition = function () {
      var newConfig = Object.assign({}, config);
      var availableField = RULE_FIELDS.find(function (f) { return !newConfig[f]; });
      if (availableField) {
        newConfig[availableField] = '';
        onChange(newConfig);
      }
    };

    var handleRemoveCondition = function (field) {
      var newConfig = Object.assign({}, config);
      delete newConfig[field];
      onChange(newConfig);
    };

    var handleFieldChange = function (index, newField) {
      var condsCopy = conditions.slice();
      var oldField = condsCopy[index].field;
      var newConfig = Object.assign({}, config);
      var value = newConfig[oldField];
      delete newConfig[oldField];
      newConfig[newField] = value;
      onChange(newConfig);
    };

    var handleValueChange = function (field, newValue) {
      var newConfig = Object.assign({}, config);
      newConfig[field] = newValue;
      onChange(newConfig);
    };

    return html`
      <div class="rule-config-form">
        ${conditions.map(function (cond, i) {
          return html`
            <div key=${cond.field} class="rule-condition-row">
              <select class="select rule-field-select"
                      value=${cond.field}
                      onChange=${function (e) { handleFieldChange(i, e.target.value); }}>
                ${RULE_FIELDS.map(function (f) {
                  return html`<option key=${f} value=${f}>${f}</option>`;
                })}
              </select>
              <input class="input rule-value-input"
                     type="text"
                     value={cond.value || ''}
                     placeholder="${t('view_manager.rule_value')}"
                     onInput=${function (e) { handleValueChange(cond.field, e.target.value); }} />
              <button type="button" class="btn btn-icon btn-sm rule-remove-btn"
                      onClick=${function () { handleRemoveCondition(cond.field); }}
                      aria-label="${t('view_manager.delete')}">
                ×
              </button>
            </div>
          `;
        })}
        <button type="button" class="btn btn-secondary btn-sm rule-add-btn"
                onClick=${handleAddCondition}
                disabled=${conditions.length >= RULE_FIELDS.length}>
          ${t('view_manager.add_condition')}
        </button>
      </div>
    `;
  }

  // ── Main Component ───────────────────────────────────────────────────────────
  function ViewManagerComponent(props) {
    var signal = props.signal;

    // Data state
    var _views = useState([]);
    var views = _views[0], setViews = _views[1];
    var _selectedViewId = useState(null);
    var selectedViewId = _selectedViewId[0], setSelectedViewId = _selectedViewId[1];
    var _tree = useState(null);
    var tree = _tree[0], setTree = _tree[1];
    var _selectedNode = useState(null);
    var selectedNode = _selectedNode[0], setSelectedNode = _selectedNode[1];
    var _expandedNodes = useState({});
    var expandedNodes = _expandedNodes[0], setExpandedNodes = _expandedNodes[1];

    // Loading/error state
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];

    // Modal state
    var _modalOpen = useState(false);
    var modalOpen = _modalOpen[0], setModalOpen = _modalOpen[1];
    var _modalType = useState(null);
    var modalType = _modalType[0], setModalType = _modalType[1];
    var _editingItem = useState(null);
    var editingItem = _editingItem[0], setEditingItem = _editingItem[1];

    // Form state
    var _formData = useState({});
    var formData = _formData[0], setFormData = _formData[1];

    var mountedRef = useRef(true);

    // ── API Calls ─────────────────────────────────────────────────────────────
    function loadViews() {
      setLoading(true);
      setError(null);
      Api.get('/admin/views', { signal: signal, silent: true }).then(function (res) {
        if (!mountedRef.current) return;
        if (res && res.success) {
          setViews(res.data || []);
          if (res.data && res.data.length > 0 && !selectedViewId) {
            setSelectedViewId(res.data[0].id);
          }
          setLoading(false);
        } else {
          setError(t('view_manager.no_views'));
          setLoading(false);
        }
      }).catch(function (e) {
        if (e && e.name === 'AbortError') return;
        if (mountedRef.current) { setError(t('view_manager.no_views')); setLoading(false); }
      });
    }

    function loadTree(viewId) {
      if (!viewId) return;
      Api.get('/admin/views/' + viewId + '/tree', { signal: signal, silent: true }).then(function (res) {
        if (!mountedRef.current) return;
        if (res && res.success) {
          setTree(res.data || null);
        }
      }).catch(function (e) {
        if (e && e.name === 'AbortError') return;
        console.error('Failed to load tree:', e);
      });
    }

    function createView(data) {
      return Api.post('/admin/views', data, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('view_manager.create_success'), 'success');
          loadViews();
          return true;
        }
        Components.showToast(t('view_manager.create_error') || 'Failed to create view', 'error');
        return false;
      });
    }

    function deleteView(viewId) {
      return Api.delete('/admin/views/' + viewId, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('view_manager.delete_success') || 'View deleted', 'success');
          if (selectedViewId === viewId) {
            setSelectedViewId(null);
            setTree(null);
            setSelectedNode(null);
          }
          loadViews();
          return true;
        }
        Components.showToast(t('view_manager.delete_error') || 'Failed to delete view', 'error');
        return false;
      });
    }

    function createNode(viewId, data) {
      return Api.post('/admin/views/' + viewId + '/nodes', data, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('view_manager.create_success'), 'success');
          loadTree(viewId);
          return res.data;
        }
        Components.showToast(t('view_manager.create_error') || 'Failed to create node', 'error');
        return null;
      });
    }

    function updateNode(viewId, nodeId, data) {
      return Api.put('/admin/views/' + viewId + '/nodes/' + nodeId, data, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('view_manager.update_success') || 'Node updated', 'success');
          loadTree(viewId);
          return true;
        }
        Components.showToast(t('view_manager.update_error') || 'Failed to update node', 'error');
        return false;
      });
    }

    function deleteNode(viewId, nodeId) {
      return Api.delete('/admin/views/' + viewId + '/nodes/' + nodeId, { signal: signal }).then(function (res) {
        if (res && res.success) {
          Components.showToast(t('view_manager.delete_success') || 'Node deleted', 'success');
          setSelectedNode(null);
          loadTree(viewId);
          return true;
        }
        Components.showToast(t('view_manager.delete_error') || 'Failed to delete node', 'error');
        return false;
      });
    }

    // ── Event Handlers ─────────────────────────────────────────────────────────
    function handleViewChange(viewId) {
      setSelectedViewId(Number(viewId));
      setTree(null);
      setSelectedNode(null);
      setExpandedNodes({});
    }

    function handleNodeSelect(node) {
      setSelectedNode(node);
    }

    function handleNodeToggle(node) {
      var newExpanded = Object.assign({}, expandedNodes);
      newExpanded[node.id] = !newExpanded[node.id];
      setExpandedNodes(newExpanded);
    }

    function openNewViewModal() {
      setModalType('new-view');
      setEditingItem(null);
      setFormData({ name: '', slug: '', channel_id: '' });
      setModalOpen(true);
    }

    function openNewNodeModal() {
      setModalType('new-node');
      setEditingItem(null);
      setFormData({ type: 'folder', name: '', parent_id: null, file_id: '', rule_config: {} });
      setModalOpen(true);
    }

    function openEditNodeModal(node) {
      setModalType('edit-node');
      setEditingItem(node);
      setFormData({ name: node.name, parent_id: node.parent_id || null });
      if (node.type === 'rule_folder') {
        setFormData(function (prev) { 
          return Object.assign({}, prev, { rule_config: node.rule_config || {} }); 
        });
      }
      setModalOpen(true);
    }

    function handleDeleteView(viewId) {
      Components.showConfirmModal(t('view_manager.delete_confirm')).then(function (confirmed) {
        if (confirmed) deleteView(viewId);
      });
    }

    function handleDeleteNode(nodeId) {
      if (!selectedViewId) return;
      Components.showConfirmModal(t('view_manager.delete_confirm')).then(function (confirmed) {
        if (confirmed) deleteNode(selectedViewId, nodeId);
      });
    }

    function handleModalSubmit(e) {
      e.preventDefault();
      if (modalType === 'new-view') {
        if (!formData.name || !formData.slug) return;
        createView({
          name: formData.name,
          slug: formData.slug,
          channel_id: formData.channel_id ? Number(formData.channel_id) : null
        }).then(function (success) {
          if (success) setModalOpen(false);
        });
      } else if (modalType === 'new-node') {
        if (!selectedViewId || !formData.name) return;
        var data = {
          type: formData.type,
          name: formData.name,
          parent_id: formData.parent_id
        };
        if (formData.type === 'file_ref') {
          data.file_id = formData.file_id;
        } else if (formData.type === 'rule_folder') {
          data.rule_config = formData.rule_config;
        }
        createNode(selectedViewId, data).then(function (node) {
          if (node) setModalOpen(false);
        });
      } else if (modalType === 'edit-node') {
        if (!selectedViewId || !editingItem) return;
        var data = {
          name: formData.name,
          parent_id: formData.parent_id
        };
        if (editingItem.type === 'rule_folder') {
          data.rule_config = formData.rule_config;
        }
        updateNode(selectedViewId, editingItem.id, data).then(function (success) {
          if (success) setModalOpen(false);
        });
      }
    }

    function handleModalClose() {
      setModalOpen(false);
      setFormData({});
      setEditingItem(null);
      setModalType(null);
    }

    // ── Effects ───────────────────────────────────────────────────────────────
    useEffect(function () {
      mountedRef.current = true;
      loadViews();
      return function () { mountedRef.current = false; };
    }, []);

    useEffect(function () {
      if (selectedViewId) loadTree(selectedViewId);
    }, [selectedViewId]);

    // ── Render Helpers ────────────────────────────────────────────────────────
    function renderModal() {
      if (!modalOpen) return null;

      var title = '';
      var body = '';

      if (modalType === 'new-view') {
        title = t('view_manager.new_view');
        body = `
          <form id="view-form" class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.view_name')} *</label>
              <input type="text" name="name" class="input w-full" value="${formData.name || ''}" required />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.view_slug')} *</label>
              <input type="text" name="slug" class="input w-full" value="${formData.slug || ''}" required />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.channel')}</label>
              <input type="text" name="channel_id" class="input w-full" value="${formData.channel_id || ''}" />
            </div>
          </form>
        `;
      } else if (modalType === 'new-node') {
        title = t('view_manager.new_node');
        body = `
          <form id="node-form" class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.node_type')} *</label>
              <select name="type" class="select w-full" required>
                ${NODE_TYPES.map(function (t) {
                  return '<option value="' + t + '" ' + (formData.type === t ? 'selected' : '') + '>' + 
                         t('view_manager.type_' + t) + '</option>';
                }).join('')}
              </select>
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.node_name')} *</label>
              <input type="text" name="name" class="input w-full" value="${formData.name || ''}" required />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">Parent ID</label>
              <input type="text" name="parent_id" class="input w-full" value="${formData.parent_id || ''}" />
            </div>
          </form>
          <div id="rule-config-container"></div>
          <div id="file-ref-container"></div>
        `;
      } else if (modalType === 'edit-node') {
        title = t('view_manager.edit');
        body = `
          <form id="node-form" class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-1">${t('view_manager.node_name')} *</label>
              <input type="text" name="name" class="input w-full" value="${formData.name || ''}" required />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">Parent ID</label>
              <input type="text" name="parent_id" class="input w-full" value="${formData.parent_id || ''}" />
            </div>
          </form>
          <div id="rule-config-container"></div>
        `;
      }

      var modal;
      setTimeout(function () {
        modal = Components.createModal({
          title: title,
          bodyHtml: body,
          onMount: function (overlay) {
            var form = overlay.querySelector('form');
            if (form) {
              form.addEventListener('submit', handleModalSubmit);
              
              // Bind input changes
              var inputs = form.querySelectorAll('input, select');
              inputs.forEach(function (input) {
                input.addEventListener('input', function (e) {
                  var newFormData = Object.assign({}, formData);
                  newFormData[e.target.name] = e.target.value;
                  setFormData(newFormData);
                });
              });

              // Handle type change for new node modal
              var typeSelect = form.querySelector('select[name="type"]');
              if (typeSelect) {
                typeSelect.addEventListener('change', function (e) {
                  var newFormData = Object.assign({}, formData);
                  newFormData.type = e.target.value;
                  setFormData(newFormData);
                });
              }
            }

            // Render rule config form if needed
            if (modalType === 'new-node' && formData.type === 'rule_folder') {
              var ruleContainer = document.getElementById('rule-config-container');
              if (ruleContainer) {
                render(html`<${RuleConfigForm} config=${formData.rule_config || {}} 
                                       onChange=${function (c) { 
                                         setFormData(function (prev) { 
                                           return Object.assign({}, prev, { rule_config: c }); 
                                         }); 
                                       }} />`, ruleContainer);
              }
            }

            // Render rule config form for edit node
            if (modalType === 'edit-node' && editingItem && editingItem.type === 'rule_folder') {
              var ruleContainer = document.getElementById('rule-config-container');
              if (ruleContainer) {
                render(html`<${RuleConfigForm} config=${formData.rule_config || {}} 
                                       onChange=${function (c) { 
                                         setFormData(function (prev) { 
                                           return Object.assign({}, prev, { rule_config: c }); 
                                         }); 
                                       }} />`, ruleContainer);
              }
            }
          }
        });
      }, 0);

      return null;
    }

    function renderTreePanel() {
      if (loading) {
        return html`<div class="card p-4"><div dangerouslySetInnerHTML=${{ __html: Components.skeletonTree(5) }} /></div>`;
      }
      if (error) {
        return html`<div class="card p-6"><p class="text-sm" style="color:var(--color-error)">${error}</p></div>`;
      }
      if (!tree || !tree.length) {
        return html`
          <div class="card p-6">
            <div class="empty-state">
              <p class="text-sm" style="color:var(--color-text-tertiary)">${t('view_manager.no_views')}</p>
            </div>
          </div>`;
      }

      return html`
        <div class="card p-2">
          ${tree.map(function (node) {
            return html`<${TreeNode} key=${node.id} node=${node} level=${0}
                                    isSelected={selectedNode && selectedNode.id === node.id}
                                    onSelect=${handleNodeSelect}
                                    expanded={expandedNodes[node.id] || false}
                                    onToggle=${handleNodeToggle} />`;
          })}
        </div>`;
    }

    function renderDetailsPanel() {
      if (!selectedNode) {
        return html`
          <div class="card p-6">
            <div class="empty-state">
              <p class="text-sm" style="color:var(--color-text-tertiary)">${t('view_manager.no_views')}</p>
            </div>
          </div>`;
      }

      var isRuleFolder = selectedNode.type === 'rule_folder';

      return html`
        <div class="card p-6">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-base font-semibold">${selectedNode.name}</h3>
            <div class="flex gap-2">
              ${isRuleFolder ? null : html`
                <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { openEditNodeModal(selectedNode); }}>
                  ${t('view_manager.edit')}
                </button>
              `}
              <button type="button" class="btn btn-ghost btn-sm" onClick=${function () { handleDeleteNode(selectedNode.id); }}>
                ${t('view_manager.delete')}
              </button>
            </div>
          </div>
          <div class="space-y-3 text-sm">
            <div>
              <span class="text-xs" style="color:var(--color-text-tertiary)">${t('view_manager.node_type')}:</span>
              <span class="ml-2">${t('view_manager.type_' + selectedNode.type)}</span>
            </div>
            <div>
              <span class="text-xs" style="color:var(--color-text-tertiary)">ID:</span>
              <span class="ml-2">${selectedNode.id}</span>
            </div>
            ${selectedNode.parent_id ? html`
              <div>
                <span class="text-xs" style="color:var(--color-text-tertiary)">Parent ID:</span>
                <span class="ml-2">${selectedNode.parent_id}</span>
              </div>` : null}
            ${isRuleFolder && selectedNode.rule_config ? html`
              <div>
                <span class="text-xs" style="color:var(--color-text-tertiary)">${t('view_manager.rule_config')}:</span>
                <pre class="mt-1 p-2 text-xs" style="background:var(--color-bg-secondary);border-radius:var(--radius-sm)">${JSON.stringify(selectedNode.rule_config, null, 2)}</pre>
              </div>` : null}
            ${isRuleFolder && selectedNode.materialized_count !== undefined ? html`
              <div>
                <span class="text-xs" style="color:var(--color-text-tertiary)">${t('view_manager.materialized')}:</span>
                <span class="ml-2">${selectedNode.materialized_count}</span>
              </div>` : null}
          </div>
        </div>`;
    }

    // ── Main Layout ────────────────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-4">
          <h1 class="text-lg font-semibold">${t('view_manager.title')}</h1>
          <div class="flex gap-2">
            <button type="button" class="btn btn-primary" onClick=${openNewViewModal}>
              ${t('view_manager.new_view')}
            </button>
            ${selectedViewId ? html`
              <button type="button" class="btn btn-secondary" onClick=${openNewNodeModal}>
                ${t('view_manager.new_node')}
              </button>` : null}
          </div>
        </div>

        <div class="flex gap-4">
          {/* Left Panel - View Selector + Tree */}
          <div class="w-64 shrink-0">
            <div class="mb-3">
              <label class="text-xs font-medium mb-1 block" style="color:var(--color-text-secondary)">
                ${t('view_manager.title')}
              </label>
              <select class="select w-full" value={selectedViewId || ''} onChange=${function (e) { handleViewChange(e.target.value); }}>
                ${views.length === 0 ? html`<option value="">${t('view_manager.no_views')}</option>` : null}
                ${views.map(function (v) {
                  return html`<option key=${v.id} value=${v.id}>${v.name}</option>`;
                })}
              </select>
            </div>
            ${selectedViewId ? renderTreePanel() : html`
              <div class="card p-6">
                <p class="text-sm" style="color:var(--color-text-tertiary)">${t('view_manager.no_views')}</p>
              </div>`}
            ${selectedViewId ? html`
              <button type="button" class="btn btn-ghost btn-sm mt-2 w-full text-danger"
                      onClick=${function () { handleDeleteView(selectedViewId); }}>
                ${t('view_manager.delete')}
              </button>` : null}
          </div>

          {/* Right Panel - Details */}
          <div class="flex-1">
            ${renderDetailsPanel()}
          </div>
        </div>

        ${renderModal()}
      </div>
    `;
  }

  function renderFn(params, query, signal) {
    var app = document.getElementById('main-content');
    if (!app) return;
    render(html`<${ViewManagerComponent} signal=${signal} />`, app);
  }

  function destroyFn() {
    var app = document.getElementById('main-content');
    if (app) render(null, app);
  }

  return { render: renderFn, destroy: destroyFn };
})();