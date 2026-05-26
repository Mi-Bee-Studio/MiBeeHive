const RegistriesCleanup = (function () {
  'use strict';
  var html = PreactBridge.html;
  var pRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;

  function _regName(regs, id) {
    var r = regs.find(function (x) { return x.id === id; });
    return r ? Helpers.escapeHtml(r.name) : '#' + id;
  }

  function _ruleLabel(p) {
    if (p.keep_days > 0) return t('reg_keep_last_days', { count: p.keep_days });
    if (p.keep_count > 0) return t('reg_keep_last_tags', { count: p.keep_count });
    if (p.keep_pattern) return t('reg_keep_matching_pattern', { pattern: Helpers.escapeHtml(p.keep_pattern) });
    return t('reg_no_rule');
  }

  // ── Policy Modal (portal) ─────────────────────────────────────────────
  function PolicyModal(props) {
    var policy = props.policy;
    var isEditing = !!policy;
    var regs = props.registries;
    var activeRule = isEditing && policy.keep_days > 0 ? 'days'
      : isEditing && policy.keep_count > 0 ? 'count'
      : isEditing && policy.keep_pattern ? 'pattern' : 'days';

    var formState = useState({
      registry_id: isEditing ? policy.registry_id : (regs.length ? regs[0].id : 0),
      repo_pattern: isEditing ? policy.repo_pattern : '*',
      rule_type: activeRule,
      keep_days: isEditing && policy.keep_days > 0 ? String(policy.keep_days) : '',
      keep_count: isEditing && policy.keep_count > 0 ? String(policy.keep_count) : '',
      keep_pattern: isEditing && policy.keep_pattern ? policy.keep_pattern : '',
      enabled: !isEditing || policy.enabled
    });
    var form = formState[0], setForm = formState[1];
    var ref = useRef(null);

    useEffect(function () {
      var el = ref.current && ref.current.querySelector('input, select');
      if (el) el.focus();
    }, []);

    function up(k, v) { setForm(Object.assign({}, form, { [k]: v })); }

    function handleSave() {
      var data = {
        registry_id: parseInt(form.registry_id, 10),
        repo_pattern: form.repo_pattern || '*',
        keep_days: form.rule_type === 'days' ? (parseInt(form.keep_days, 10) || 0) : 0,
        keep_count: form.rule_type === 'count' ? (parseInt(form.keep_count, 10) || 0) : 0,
        keep_pattern: form.rule_type === 'pattern' ? form.keep_pattern : '',
        enabled: form.enabled
      };
      var req = isEditing
        ? Api.put('/admin/retention/' + policy.id, data)
        : Api.post('/admin/retention', data);
      req.then(function (r) {
        if (r && r.success) {
          Components.showToast(isEditing ? t('reg_updated') : t('reg_created'), 'success');
          props.onSave();
          props.onClose();
        } else {
          Components.showToast((r && r.message) || t('reg_failed'), 'error');
        }
      });
    }

    function handleKey(e) {
      if (e.key === 'Escape') { props.onClose(); return; }
      if (e.key === 'Tab') {
        var els = ref.current.querySelectorAll('button, input, select, textarea');
        if (!els.length) return;
        if (e.shiftKey && document.activeElement === els[0]) { e.preventDefault(); els[els.length - 1].focus(); }
        else if (!e.shiftKey && document.activeElement === els[els.length - 1]) { e.preventDefault(); els[0].focus(); }
      }
    }

    var lbl = 'display:block;margin-bottom:0.25rem;font-size:0.75rem;font-weight:var(--font-weight-medium);color:var(--color-text-tertiary)';
    var inp = { width: '100%', marginTop: '2px' };

    return html`
      <div ref=${ref} class="modal-overlay" onClick=${function (e) { if (e.target === e.currentTarget) props.onClose(); }} onKeyDown=${handleKey}>
        <div class="modal-content" role="dialog" aria-modal="true">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${isEditing ? t('common_edit') + ' ' + t('reg_policy') : t('reg_create_policy')}</h3>
          <div class="flex flex-col gap-3">
            <div>
              <label style=${lbl}>${t('reg_registry_label')}</label>
              <select class="input" style=${inp} value=${String(form.registry_id)}
                onChange=${function (e) { up('registry_id', parseInt(e.target.value, 10)); }}>
                ${regs.map(function (r) { return html`<option key=${r.id} value=${String(r.id)}>${Helpers.escapeHtml(r.name)}</option>`; })}
              </select>
            </div>
            <div>
              <label style=${lbl}>${t('reg_repo_pattern')}</label>
              <input class="input" style=${inp} placeholder="* for all" value=${form.repo_pattern}
                onInput=${function (e) { up('repo_pattern', e.target.value); }} />
            </div>
            <div>
              <label style=${lbl}>${t('reg_rule_type_label')}</label>
              <div style="display:flex;gap:1rem;margin-top:4px">
                ${['days', 'count', 'pattern'].map(function (r) {
                  return html`<label key=${r} style="font-size:0.8125rem;display:flex;align-items:center;gap:4px;cursor:pointer">
                    <input type="radio" name="rc-rule" value=${r} checked=${form.rule_type === r}
                      onChange=${function () { up('rule_type', r); }} /> ${t('reg_rule_' + r)}
                  </label>`;
                })}
              </div>
            </div>
            ${form.rule_type === 'days' ? html`
              <div><label style=${lbl}>${t('reg_keep_days')}</label>
              <input type="number" min="1" class="input" style=${inp} value=${form.keep_days}
                onInput=${function (e) { up('keep_days', e.target.value); }} /></div>` : null}
            ${form.rule_type === 'count' ? html`
              <div><label style=${lbl}>${t('reg_keep_count')}</label>
              <input type="number" min="1" class="input" style=${inp} value=${form.keep_count}
                onInput=${function (e) { up('keep_count', e.target.value); }} /></div>` : null}
            ${form.rule_type === 'pattern' ? html`
              <div><label style=${lbl}>${t('reg_keep_pattern')}</label>
              <input class="input" style=${inp} value=${form.keep_pattern}
                onInput=${function (e) { up('keep_pattern', e.target.value); }} /></div>` : null}
            <div class="flex items-center gap-2">
              <input type="checkbox" id="rc-enabled" style="width:1rem;height:1rem;accent-color:var(--color-primary)"
                checked=${form.enabled} onChange=${function (e) { up('enabled', e.target.checked); }} />
              <label for="rc-enabled" style="font-size:0.8125rem;color:var(--color-text-secondary)">${t('enabled')}</label>
            </div>
          </div>
          <div class="flex justify-end gap-3 mt-4">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('common_cancel')}</button>
            <button class="btn btn-primary" onClick=${handleSave}>${t('common_save')}</button>
          </div>
        </div>
      </div>`;
  }

  function _showModal(policy, regs, onSave) {
    var root = document.createElement('div');
    document.body.appendChild(root);
    var prev = document.activeElement;
    function close() { pRender(null, root); root.remove(); if (prev && prev.focus) prev.focus(); }
    pRender(html`<${PolicyModal} policy=${policy} registries=${regs} onClose=${close} onSave=${function () { onSave(); close(); }} />`, root);
  }

  // ── Cleanup Component ──────────────────────────────────────────────────
  function Component() {
    var dataState = useState({ policies: [], registries: [], loading: true });
    var data = dataState[0], setData = dataState[1];

    function loadData() {
      Promise.all([Api.get('/admin/retention'), Api.get('/admin/registries')]).then(function (results) {
        if (!results[0] || !results[0].success) {
          Components.showToast((results[0] && results[0].message) || t('reg_failed_to_load'), 'error');
          setData({ policies: [], registries: [], loading: false });
          return;
        }
        setData({
          policies: results[0].data || [],
          registries: (results[1] && results[1].success) ? (results[1].data || []) : [],
          loading: false
        });
      });
    }

    useEffect(function () { loadData(); }, []);

    function togglePolicy(p) {
      Api.put('/admin/retention/' + p.id, {
        registry_id: p.registry_id, repo_pattern: p.repo_pattern,
        keep_days: p.keep_days, keep_count: p.keep_count,
        keep_pattern: p.keep_pattern, enabled: !p.enabled
      }).then(function (r) {
        if (r && r.success) loadData();
        else Components.showToast((r && r.message) || t('reg_failed'), 'error');
      });
    }

    function deletePolicy(p) {
      Components.showConfirmModal(
        t('reg_delete_policy_confirm', { name: _regName(data.registries, p.registry_id) }),
        function () {
          Api.delete('/admin/retention/' + p.id).then(function (r) {
            if (r && r.success) { Components.showToast(t('reg_deleted'), 'success'); loadData(); }
            else Components.showToast((r && r.message) || t('reg_failed'), 'error');
          });
        });
    }

    function runPolicy(p) {
      Api.post('/admin/retention/' + p.id + '/execute').then(function (r) {
        if (r && r.success) { Components.showToast(t('reg_deleted_tags_count', { count: r.data || 0 }), 'success'); loadData(); }
        else Components.showToast((r && r.message) || t('reg_execution_failed'), 'error');
      });
    }

    if (data.loading) return html`<div class="flex-center" style="padding:1rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;

    return html`
      <div>
        <div style="display:flex;justify-content:flex-end;margin-bottom:0.75rem">
          <button class="btn btn-primary btn-sm" onClick=${function () { _showModal(null, data.registries, loadData); }}>+ ${t('reg_create_policy')}</button>
        </div>
        ${!data.policies.length ? html`
          <p class="text-sm" style="color:var(--color-text-tertiary);text-align:center;padding:1rem">${t('reg_no_rule')}</p>
        ` : data.policies.map(function (p) {
          var on = p.enabled;
          return html`
            <div key=${p.id} class="queue-item">
              <div class="flex-1 min-w-0 flex flex-col gap-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-medium truncate" style="color:var(--color-text)">${_regName(data.registries, p.registry_id)}</span>
                  <span class="text-xs" style="color:var(--color-text-tertiary)">${Helpers.escapeHtml(p.repo_pattern)}</span>
                </div>
                <span class="text-xs" style="color:var(--color-primary)">${_ruleLabel(p)}</span>
                ${p.last_executed_at ? html`<span class="text-xs" style="color:var(--color-text-tertiary)">${t('reg_last_executed')}: ${Helpers.formatTime(p.last_executed_at)}</span>` : null}
              </div>
              <div class="flex items-center gap-1 shrink-0">
                <button class="btn btn-sm" style=${{ background: on ? 'var(--color-success-light)' : 'var(--color-bg-tertiary)', color: on ? 'var(--color-success)' : 'var(--color-text-tertiary)' }}
                  onClick=${function () { togglePolicy(p); }}>${on ? t('reg_on') : t('reg_off')}</button>
                <button class="btn btn-secondary btn-sm" onClick=${function () { runPolicy(p); }} aria-label=${t('reg_run_now')}>${t('reg_run_now')}</button>
                <button class="btn btn-secondary btn-sm" onClick=${function () { _showModal(p, data.registries, loadData); }}>${t('common_edit')}</button>
                <button class="btn btn-secondary btn-sm" style="color:var(--color-error);border-color:var(--color-error)"
                  onClick=${function () { deletePolicy(p); }} aria-label=${t('common_delete')}>${t('common_delete')}</button>
              </div>
            </div>`;
        })}
      </div>`;
  }

  return Component;
})();
