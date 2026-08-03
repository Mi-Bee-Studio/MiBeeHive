const Settings = (function () {
  'use strict';

  var html = PreactBridge.html;
  var render = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;
  var Fragment = PreactBridge.Fragment;

  // ── Password strength ─────────────────────────────────────────────────

  function getPasswordStrength(pw) {
    if (!pw || pw.length < 8) return { score: 0, level: 'weak', width: '0%', checks: {} };
    var checks = { length: pw.length >= 8, upper: /[A-Z]/.test(pw), lower: /[a-z]/.test(pw), digit: /\d/.test(pw), special: /[^a-zA-Z0-9]/.test(pw) };
    var score = (checks.length ? 1 : 0) + (checks.lower ? 1 : 0) + (checks.upper ? 1 : 0) + (checks.digit ? 1 : 0) + (checks.special ? 1 : 0);
    var level, width;
    if (score <= 2) { level = 'weak'; width = '20%'; }
    else if (score === 3) { level = 'medium'; width = '40%'; }
    else if (score === 4) { level = 'strong'; width = '70%'; }
    else { level = 'strongest'; width = '100%'; }
    return { score: score, level: level, width: width, checks: checks };
  }

  function strengthColor(l) { return l === 'weak' ? 'var(--color-error)' : l === 'medium' ? 'var(--color-warning)' : 'var(--color-success)'; }
  function strengthLabel(l) { return l === 'weak' ? t('validation_password_strength_weak') : l === 'medium' ? t('validation_password_strength_medium') : (l === 'strong' || l === 'strongest') ? t('validation_password_strength_strong') : ''; }

  var CHECK_LABELS = { length: '>= 8', upper: 'A-Z', lower: 'a-z', digit: '0-9', special: '!@#...' };

  function maskToken(token) {
    if (!token) return '';
    if (token.length <= 4) return Helpers.escapeHtml(token);
    return '*'.repeat(Math.min(token.length - 4, 12)) + token.slice(-4);
  }

  // ── Sub-components ────────────────────────────────────────────────────

  function InfoRow(props) {
    return html`
      <div class="flex justify-between items-center" style="padding:0.5rem 0">
        <span class="text-sm" style="color:var(--color-text-tertiary)">${props.label}</span>
        <span class="text-sm" style="font-weight:var(--font-weight-medium);color:var(--color-text)">${props.value}</span>
      </div>
    `;
  }

  function Divider() {
    return html`<div class="divider" style="margin:1rem 0"></div>`;
  }

  function SectionCard(props) {
    return html`
      <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">
        <h2 class="text-base" style="font-weight:var(--font-weight-semibold);color:var(--color-text);margin-bottom:${props.noPad ? '0' : '1'}rem">${props.title}</h2>
        ${props.children}
      </div>
    `;
  }

  // ── Password Warning Banner ───────────────────────────────────────────

  function PasswordWarningBanner() {
    return html`
      <div class="card" style="padding:1rem;margin-bottom:1rem;border-left:4px solid var(--color-warning);background:var(--color-surface-secondary)">
        <div class="flex items-center gap-2" style="margin-bottom:0.25rem">
          <span style="font-size:0.875rem;font-weight:var(--font-weight-medium);color:var(--color-warning)">${t('password_change_required')}</span>
        </div>
        <p style="font-size:0.8125rem;color:var(--color-text-secondary)">${t('password_default_warning')}</p>
      </div>
    `;
  }

  // ── System Info Card ──────────────────────────────────────────────────

  function SystemInfoCard(props) {
    var info = props.info;
    if (!info) return null;
    var dp = info.disk_total > 0 ? Math.round((info.disk_used / info.disk_total) * 100) : 0;

    return html`
      <${SectionCard} title=${t('system_info')}>
        <div style="display:flex;flex-direction:column;gap:0">
          <${InfoRow} label=${t('app_version')} value=${Helpers.escapeHtml(info.version || 'dev')} />
          <${InfoRow} label=${t('project_count')} value=${String(info.project_count)} />
          <${InfoRow} label=${t('file_count')} value=${info.file_count.toLocaleString()} />
        </div>
        <${Divider} />
        <${InfoRow} label=${t('disk_usage')} value=${Helpers.formatBytes(info.disk_used) + ' / ' + Helpers.formatBytes(info.disk_total)} />
        <div style="margin-top:0.75rem;margin-bottom:0.25rem">
          <div class="progress-bar" role="status">
            <div class="progress-bar-fill" style=${{ width: dp + '%' }}></div>
          </div>
          <div class="flex justify-between" style="margin-top:0.25rem">
            <span class="text-xs" style="color:var(--color-text-tertiary)">${dp}% ${t('disk_usage')}</span>
            <span class="text-xs" style="color:var(--color-text-tertiary)">${t('disk_available')}: ${Helpers.formatBytes(info.disk_avail)}</span>
          </div>
        </div>
        <${Divider} />
        <${InfoRow} label=${t('last_crawl')} value=${Helpers.formatTime(info.last_crawl_at)} />
      <//>
    `;
  }

  // ── Token List ────────────────────────────────────────────────────────

  function TokenList() {
    var state = useState({ creds: [], loading: true, error: null });
    var data = state[0], setData = state[1];

    // edit states keyed by source_type
    var editState = useState({});
    var edits = editState[0], setEdits = editState[1];
    var editValues = useState({});
    var values = editValues[0], setValues = editValues[1];
    var editVis = useState({});
    var vis = editVis[0], setVis = editVis[1];
    var savingState = useState({});
    var saving = savingState[0], setSaving = savingState[1];

    function loadTokens() {
      Api.get('/admin/credentials').then(function (res) {
        if (!res || !res.success) {
          setData({ creds: [], loading: false, error: (res && res.message) || t('error') });
          return;
        }
        setData({ creds: res.data || [], loading: false, error: null });
      });
    }

    useEffect(function () { loadTokens(); }, []);

    function startEdit(src) {
      setEdits(Object.assign({}, edits, { [src]: true }));
      setValues(Object.assign({}, values, { [src]: '' }));
      setVis(Object.assign({}, vis, { [src]: false }));
    }

    function cancelEdit(src) {
      var ne = Object.assign({}, edits); delete ne[src];
      var nv = Object.assign({}, values); delete nv[src];
      var nvi = Object.assign({}, vis); delete nvi[src];
      setEdits(ne); setValues(nv); setVis(nvi);
    }

    function toggleVis(src) {
      setVis(Object.assign({}, vis, { [src]: !vis[src] }));
    }

    function saveToken(src) {
      var val = (values[src] || '').trim();
      if (!val) return;
      setSaving(Object.assign({}, saving, { [src]: true }));
      Api.put('/admin/credentials', { source_type: src, token: val }).then(function (r) {
        setSaving(Object.assign({}, saving, { [src]: false }));
        if (r && r.success) {
          Components.showToast(t('tokens_saved'), 'success');
          cancelEdit(src);
          loadTokens();
        } else {
          Components.showToast((r && r.message) || t('error'), 'error');
        }
      });
    }

    if (data.loading) {
      return html`<div class="flex-center" style="padding:1rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;
    }

    if (data.error) {
      return html`<p class="text-sm" style="color:var(--color-text-tertiary)">${data.error}</p>`;
    }

    if (!data.creds.length) {
      return html`<p class="text-sm" style="color:var(--color-text-tertiary);padding:0.5rem 0">${t('tokens_empty')}</p>`;
    }

    return html`
      <div style="display:flex;flex-direction:column;gap:0.75rem">
        ${data.creds.map(function (c) {
          var src = c.source_type;
          var isEditing = edits[src];
          var isSaving = saving[src];
          var isPassword = !vis[src];

          return html`
            <div key=${src} style="padding:0.5rem 0">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span dangerouslySetInnerHTML=${{ __html: Helpers.sourceTypeBadge(c.source_type) }}></span>
                  <span class="text-sm" style="font-weight:var(--font-weight-medium);color:var(--color-text)">${Helpers.escapeHtml(src)}</span>
                </div>
                <div class="flex items-center gap-2">
                  ${!isEditing ? html`
                    <span class="text-sm" style="color:var(--color-text-quaternary);letter-spacing:2px">${maskToken(c.token_masked)}</span>
                    <button class="btn btn-secondary btn-sm" onClick=${function () { startEdit(src); }}>${t('tokens_edit')}</button>
                  ` : null}
                </div>
              </div>
              ${isEditing ? html`
                <div class="anim-fade-in" style="margin-top:0.5rem">
                  <div style="position:relative;display:flex;gap:0.5rem;align-items:center">
                    <input type=${isPassword ? 'password' : 'text'} class="input" placeholder=${t('tokens_placeholder')}
                      style="font-size:0.8125rem;padding:0.375rem 0.75rem;flex:1"
                      value=${values[src] || ''} onInput=${function (e) { setValues(Object.assign({}, values, { [src]: e.target.value })); }} />
                    <button type="button" class="btn btn-ghost" style="padding:0.25rem" aria-label=${t('aria_toggle_visibility')} onClick=${function () { toggleVis(src); }}>
                      <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.eye }} style=${{ display: isPassword ? '' : 'none' }}></span>
                      <span dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.eyeOff }} style=${{ display: isPassword ? 'none' : '' }}></span>
                    </button>
                  </div>
                  <div class="flex gap-2" style="margin-top:0.5rem">
                    <button class="btn btn-primary btn-sm" disabled=${isSaving} onClick=${function () { saveToken(src); }}>${isSaving ? '...' : t('tokens_save')}</button>
                    <button class="btn btn-secondary btn-sm" onClick=${function () { cancelEdit(src); }}>${t('tokens_cancel')}</button>
                  </div>
                </div>
              ` : null}
            </div>
          `;
        })}
      </div>
    `;
  }

  // ── Password Change Form ──────────────────────────────────────────────

  function PasswordForm() {
    var formState = useState({ oldPw: '', newPw: '', confirmPw: '' });
    var form = formState[0], setForm = formState[1];
    var errState = useState({ old: '', new: '', confirm: '', form: '' });
    var errors = errState[0], setErrors = errState[1];
    var submitting = useState(false);
    var isSubmitting = submitting[0], setSubmitting = submitting[1];
    var countdown = useState(0);
    var countdownVal = countdown[0], setCountdown = countdown[1];
    var timerRef = useRef(null);

    var strength = useMemo(function () {
      return getPasswordStrength(form.newPw);
    }, [form.newPw]);

    function updateField(key, val) {
      setForm(Object.assign({}, form, { [key]: val }));
    }

    function validate() {
      var e = { old: '', new: '', confirm: '', form: '' };
      var ok = true;
      if (!form.oldPw.trim()) { e.old = t('validation_required'); ok = false; }
      if (!form.newPw.trim()) { e.new = t('validation_required'); ok = false; }
      else if (form.newPw.length < 8) { e.new = t('validation_password_min'); ok = false; }
      if (!form.confirmPw.trim()) { e.confirm = t('validation_required'); ok = false; }
      else if (form.confirmPw !== form.newPw) { e.confirm = t('validation_password_mismatch'); ok = false; }
      setErrors(e);
      return ok;
    }

    function handleSubmit() {
      if (!validate()) return;
      setSubmitting(true);
      Api.post('/admin/password', { old_password: form.oldPw, new_password: form.newPw }).then(function (r) {
        if (!r || !r.success || (r.data && r.data.status === 401)) {
          setErrors({ old: '', new: '', confirm: '', form: t('password_error') });
          setSubmitting(false);
          return;
        }
        if (r && r.success) {
          Components.showToast(t('password_changed_logout'), 'success');
          setSubmitting(true);
          setCountdown(3);
          timerRef.current = setInterval(function () {
            setCountdown(function (prev) {
              if (prev <= 1) {
                clearInterval(timerRef.current);
                Auth.logout();
                window.location.hash = '#/login';
                return 0;
              }
              return prev - 1;
            });
          }, 1000);
        } else {
          setErrors({ old: '', new: '', confirm: '', form: (r && r.message) || t('password_error') });
          setSubmitting(false);
        }
      }).catch(function () {
        setErrors({ old: '', new: '', confirm: '', form: t('password_error') });
        setSubmitting(false);
      });
    }

    useEffect(function () {
      return function () {
        if (timerRef.current) clearInterval(timerRef.current);
      };
    }, []);

    var lblStyle = 'display:block;margin-bottom:0.25rem;font-size:0.8125rem;color:var(--color-text-secondary)';
    var errStyle = 'color:var(--color-error);font-size:0.75rem;margin-top:0.25rem';
    var checkKeys = ['length', 'upper', 'lower', 'digit', 'special'];

    return html`
      <div style="display:flex;flex-direction:column;gap:1rem">
        <div>
          <label class="form-label" style=${lblStyle}>${t('old_password')}</label>
          <input type="password" class="input" autocomplete="current-password"
            style=${{ borderColor: errors.old ? 'var(--color-error)' : '' }}
            value=${form.oldPw} onInput=${function (e) { updateField('oldPw', e.target.value); }}
            onKeyPress=${function (e) { if (e.key === 'Enter') handleSubmit(); }} />
          ${errors.old ? html`<p style=${errStyle}>${errors.old}</p>` : null}
        </div>

        <div>
          <label class="form-label" style=${lblStyle}>${t('new_password')}</label>
          <input type="password" class="input" autocomplete="new-password"
            style=${{ borderColor: errors.new ? 'var(--color-error)' : '' }}
            value=${form.newPw} onInput=${function (e) { updateField('newPw', e.target.value); }}
            onKeyPress=${function (e) { if (e.key === 'Enter') handleSubmit(); }} />
          <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_password_req')}</span>
          ${form.newPw.length > 0 ? html`
            <div style="margin-top:0.5rem">
              <div style="height:4px;background:var(--color-border);border-radius:2px;overflow:hidden">
                <div style=${{ height: '100%', width: strength.width, borderRadius: '2px', background: strengthColor(strength.level), transition: 'width 0.2s, background 0.2s' }}></div>
              </div>
              <span class="text-xs" style=${{ color: strengthColor(strength.level) }}>${strengthLabel(strength.level)}</span>
              <div style="margin-top:0.5rem;display:flex;flex-direction:column;gap:0.25rem">
                ${checkKeys.map(function (key) {
                  return html`
                    <div key=${key} class="flex items-center" style="gap:0.375rem">
                      <span style=${{ width: '6px', height: '6px', borderRadius: '50%', background: strength.checks[key] ? 'var(--color-success)' : 'var(--color-border)', display: 'inline-block', transition: 'background 0.2s' }}></span>
                      <span class="text-xs" style="color:var(--color-text-tertiary)">${CHECK_LABELS[key]}</span>
                    </div>
                  `;
                })}
              </div>
            </div>
          ` : null}
          ${errors.new ? html`<p style=${errStyle}>${errors.new}</p>` : null}
        </div>

        <div>
          <label class="form-label" style=${lblStyle}>${t('confirm_password')}</label>
          <input type="password" class="input" autocomplete="new-password"
            style=${{ borderColor: errors.confirm ? 'var(--color-error)' : '' }}
            value=${form.confirmPw} onInput=${function (e) { updateField('confirmPw', e.target.value); }}
            onKeyPress=${function (e) { if (e.key === 'Enter') handleSubmit(); }} />
          ${errors.confirm ? html`<p style=${errStyle}>${errors.confirm}</p>` : null}
        </div>

        ${errors.form ? html`
          <div style="font-size:0.8125rem;padding:0.5rem 0.75rem;border-radius:var(--radius-md);background:var(--color-error-light);color:var(--color-error)">${errors.form}</div>
        ` : null}

        <button class="btn btn-primary" disabled=${isSubmitting} onClick=${handleSubmit}>
          ${isSubmitting && countdownVal === 0 ? html`<div class="spinner" style="width:1rem;height:1rem;border-width:2px"></div>` : null}
          ${countdownVal > 0 ? t('password_change_countdown', { count: countdownVal }) : t('password_change')}
        </button>
      </div>
    `;
  }

  // ── Disk Threshold Card ───────────────────────────────────────────────

  function DiskThresholdCard() {
    var cfgState = useState({ warn: 90, crit: 95, enabled: true });
    var cfg = cfgState[0], setCfg = cfgState[1];
    var loadState = useState(true);
    var loading = loadState[0], setLoading = loadState[1];
    var savingState = useState(false);
    var isSaving = savingState[0], setSaving = savingState[1];

    useEffect(function () {
      Api.get('/admin/config/monitor').then(function (res) {
        setLoading(false);
        if (res && res.success && res.data) {
          setCfg({
            warn: res.data.disk_warning_percent || 90,
            crit: res.data.disk_critical_percent || 95,
            enabled: res.data.disk_check_enabled !== false
          });
        }
      });
    }, []);

    function handleSave() {
      var w = parseInt(cfg.warn, 10);
      var c = parseInt(cfg.crit, 10);
      if (isNaN(w) || isNaN(c) || w < 1 || w > 100 || c < 1 || c > 100) {
        Components.showToast(t('settings_disk_invalid'), 'error');
        return;
      }
      if (w >= c) {
        Components.showToast(t('settings_disk_invalid'), 'error');
        return;
      }
      setSaving(true);
      Api.put('/admin/config/monitor', {
        disk_warning_percent: w,
        disk_critical_percent: c,
        disk_check_enabled: cfg.enabled
      }).then(function (r) {
        setSaving(false);
        if (r && r.success) {
          Components.showToast(t('settings_saved'), 'success');
        } else {
          Components.showToast((r && r.message) || t('error'), 'error');
        }
      });
    }

    if (loading) {
      return html`<div class="flex-center" style="padding:1rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;
    }

    var lblStyle = 'display:block;margin-bottom:0.25rem;font-size:0.8125rem;color:var(--color-text-secondary)';
    var descStyle = 'font-size:0.75rem;color:var(--color-text-tertiary);margin-top:0.25rem';

    return html`
      <div style="display:flex;flex-direction:column;gap:1rem">
        <div>
          <label class="form-label" style=${lblStyle}>${t('settings_disk_warning_label')}</label>
          <input type="number" class="input" min="1" max="100" value=${cfg.warn} style="max-width:120px"
            onInput=${function (e) { setCfg(Object.assign({}, cfg, { warn: e.target.value })); }} />
          <p style=${descStyle}>${t('settings_disk_warning_desc')}</p>
        </div>
        <div>
          <label class="form-label" style=${lblStyle}>${t('settings_disk_critical_label')}</label>
          <input type="number" class="input" min="1" max="100" value=${cfg.crit} style="max-width:120px"
            onInput=${function (e) { setCfg(Object.assign({}, cfg, { crit: e.target.value })); }} />
          <p style=${descStyle}>${t('settings_disk_critical_desc')}</p>
        </div>
        <div class="flex items-center" style="gap:0.5rem">
          <input type="checkbox" checked=${cfg.enabled} style="width:1rem;height:1rem;accent-color:var(--color-primary)"
            onChange=${function (e) { setCfg(Object.assign({}, cfg, { enabled: e.target.checked })); }} />
          <label class="form-label" style=${lblStyle.replace('margin-bottom:0.25rem', 'margin-bottom:0')}>${t('settings_disk_check_label')}</label>
        </div>
        <button class="btn btn-primary" disabled=${isSaving} onClick=${handleSave}>
          ${isSaving ? html`<div class="spinner" style="width:1rem;height:1rem;border-width:2px"></div>` : null}
          ${t('save')}
        </button>
      </div>
    `;
  }

  // ── Storage Paths Card ────────────────────────────────────────────────

  function StoragePathsCard() {
    var pathsState = useState({ oss: '', os_install: '', iso: '' });
    var paths = pathsState[0], setPaths = pathsState[1];
    var origState = useState({ oss: '', os_install: '', iso: '' });
    var origPaths = origState[0], setOrigPaths = origState[1];
    var loadState = useState(true);
    var loading = loadState[0], setLoading = loadState[1];
    var savingState = useState(false);
    var isSaving = savingState[0], setSaving = savingState[1];
    var editState = useState(false);
    var isEditing = editState[0], setEditing = editState[1];

    useEffect(function () {
      Api.get('/admin/config/storage').then(function (res) {
        setLoading(false);
        if (res && res.success && res.data) {
          var p = { oss: res.data.oss || '', os_install: res.data.os_install || '', iso: res.data.iso || '' };
          setPaths(p);
          setOrigPaths(p);
        }
      });
    }, []);

    function handleSave() {
      var changed = {};
      if (paths.oss && paths.oss !== origPaths.oss) changed.oss = paths.oss;
      if (paths.os_install && paths.os_install !== origPaths.os_install) changed.os_install = paths.os_install;
      if (paths.iso && paths.iso !== origPaths.iso) changed.iso = paths.iso;
      if (!Object.keys(changed).length) {
        Components.showToast(t('storage_no_change'), 'warning');
        return;
      }
      for (var k in changed) {
        if (changed[k] && changed[k][0] !== '/') {
          Components.showToast(t('storage_invalid_path'), 'error');
          return;
        }
      }
      Components.showConfirmModal(t('storage_confirm'), function () {
        setSaving(true);
        Api.put('/admin/config/storage', changed).then(function (r) {
          setSaving(false);
          if (r && r.success) {
            setOrigPaths(Object.assign({}, origPaths, changed));
            setEditing(false);
            Components.showToast(t('storage_saved'), 'success', { duration: 6000 });
          } else {
            Components.showToast((r && r.message) || t('error'), 'error');
          }
        });
      });
    }

    if (loading) {
      return html`<div class="flex-center" style="padding:1rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;
    }

    var lblStyle = 'display:block;margin-bottom:0.25rem;font-size:0.8125rem;color:var(--color-text-secondary)';
    var fields = [
      { key: 'oss', label: t('storage_oss') },
      { key: 'os_install', label: t('storage_os_install') },
      { key: 'iso', label: t('storage_iso') }
    ];

    return html`
      <div style="display:flex;flex-direction:column;gap:1rem">
        ${fields.map(function (f) {
          return html`
            <div key=${f.key}>
              <label class="form-label" style=${lblStyle}>${f.label}</label>
              <div class="flex items-center" style="gap:0.5rem">
                <input type="text" class="input" readonly=${!isEditing}
                  style=${{ flex: '1', fontSize: '0.8125rem', opacity: isEditing ? '1' : '0.8', background: isEditing ? '' : 'var(--color-bg-secondary)' }}
                  value=${paths[f.key] || ''}
                  onInput=${function (e) { setPaths(Object.assign({}, paths, { [f.key]: e.target.value })); }} />
              </div>
            </div>
          `;
        })}
        <div style="font-size:0.75rem;color:var(--color-warning);border-left:3px solid var(--color-warning);padding:0.375rem 0.625rem;background:var(--color-warning-light);border-radius:0 var(--radius-md) var(--radius-md) 0">
          ${t('storage_warning')}
        </div>
        <div class="flex gap-2">
          ${!isEditing ? html`
            <button class="btn btn-secondary" onClick=${function () { setEditing(true); }}>${t('storage_edit')}</button>
          ` : html`
            <button class="btn btn-primary" disabled=${isSaving} onClick=${handleSave}>
              ${isSaving ? html`<div class="spinner" style="width:1rem;height:1rem;border-width:2px"></div>` : null}
              ${t('storage_save')}
            </button>
            <button class="btn btn-secondary" disabled=${isSaving} onClick=${function () { setPaths(Object.assign({}, origPaths)); setEditing(false); }}>${t('storage_cancel')}</button>
          `}
        </div>
      </div>
    `;
  }

  // ── Backup List ───────────────────────────────────────────────────────

  function BackupList() {
    var dataState = useState({ backups: [], loading: true, error: null });
    var data = dataState[0], setData = dataState[1];
    var restoring = useState(null);
    var restoringFile = restoring[0], setRestoring = restoring[1];
    var restoreSuccess = useState(false);
    var isSuccess = restoreSuccess[0], setSuccess = restoreSuccess[1];

    function loadBackups() {
      Api.get('/admin/backups').then(function (res) {
        if (!res || !res.success) {
          setData({ backups: [], loading: false, error: t('error') });
          return;
        }
        setData({ backups: res.data || [], loading: false, error: null });
      });
    }

    useEffect(function () { loadBackups(); }, []);

    function handleRestore(filename) {
      Components.showConfirmModal(t('backup_restore_confirm_msg', { filename: filename }), function (btn) {
        setRestoring(filename);
        Api.post('/admin/backups/restore', { filename: filename }).then(function (r) {
          if (r && r.success) {
            Components.showToast(t('backup_restore_success'), 'success');
            setSuccess(true);
          } else {
            Components.showToast((r && r.message) || t('error'), 'error');
            setRestoring(null);
          }
        }).catch(function () {
          Components.showToast(t('backup_restore_failed') || t('error'), 'error');
          setRestoring(null);
        });
      });
    }

    if (isSuccess) {
      return html`
        <div class="empty-state">
          <div class="spinner" style="width:2rem;height:2rem;border-width:3px;margin:0 auto 1rem"></div>
          <p class="text-sm" style="color:var(--color-text-secondary)">${t('backup_restore_success')}</p>
        </div>
      `;
    }

    if (data.loading) {
      return html`<div class="flex-center" style="padding:1rem"><div class="spinner" style="width:1.25rem;height:1.25rem;border-width:2px"></div></div>`;
    }

    if (data.error) {
      return html`<p class="text-sm" style="color:var(--color-text-tertiary)">${data.error}</p>`;
    }

    if (!data.backups.length) {
      return html`<p class="text-sm" style="color:var(--color-text-tertiary);padding:0.5rem 0">${t('backup_restore_empty')}</p>`;
    }

    return html`
      <div>
        <div class="flex text-xs" style="font-weight:var(--font-weight-medium);color:var(--color-text-tertiary);padding:0 0 0.25rem">
          <span style="flex:1">${t('backup_restore_filename')}</span>
          <span style="width:80px;text-align:right">${t('backup_restore_size')}</span>
          <span style="width:130px;text-align:right;margin-right:60px">${t('backup_restore_date')}</span>
        </div>
        <div style="display:flex;flex-direction:column">
          ${data.backups.map(function (b) {
            var isRestoring = restoringFile === b.filename;
            return html`
              <div key=${b.filename} class="flex items-center" style="padding:0.5rem 0;border-bottom:1px solid var(--color-border-secondary)">
                <span class="text-sm" style="flex:1;color:var(--color-text);word-break:break-all">${Helpers.escapeHtml(b.filename)}</span>
                <span class="text-xs" style="width:80px;text-align:right;color:var(--color-text-tertiary)">${Helpers.formatBytes(b.size)}</span>
                <span class="text-xs" style="width:130px;text-align:right;color:var(--color-text-tertiary)">${Helpers.escapeHtml(b.mod_time)}</span>
                <button class="btn btn-secondary btn-sm" disabled=${isRestoring} style="margin-left:0.75rem"
                  onClick=${function () { handleRestore(b.filename); }}>
                  ${isRestoring ? html`<div class="spinner" style="width:0.875rem;height:0.875rem;border-width:2px"></div>` : null}
                  ${t('backup_restore_btn')}
                </button>
              </div>
            `;
          })}
        </div>
      </div>
    `;
  }

  // ── Main Settings Page ────────────────────────────────────────────────

  function SettingsPage() {
    var infoState = useState(null);
    var sysInfo = infoState[0], setSysInfo = infoState[1];
    var pwStatusState = useState(false);
    var isDefaultPw = pwStatusState[0], setDefaultPw = pwStatusState[1];
    var errState = useState(null);
    var error = errState[0], setError = errState[1];
    var loadingState = useState(true);
    var isLoading = loadingState[0], setLoading = loadingState[1];

    useEffect(function () {
      Promise.all([
        Api.get('/system/info'),
        Api.get('/auth/password-status')
      ]).then(function (results) {
        setLoading(false);
        var infoRes = results[0], pwRes = results[1];

        if (!infoRes || !infoRes.success) {
          setError((infoRes && infoRes.message) || t('error'));
          return;
        }
        setSysInfo(infoRes.data);
        if (pwRes && pwRes.success && pwRes.data && pwRes.data.is_default) {
          setDefaultPw(true);
        }
      });
    }, []);

    function handleLogout() {
      Components.showConfirmModal(t('confirm_logout'), function () {
        Auth.logout();
        window.location.hash = '#/login';
      });
    }

    if (isLoading) {
      return html`
        <div class="p-4 md:p-6" style="max-width:48rem;margin:0 auto">
          ${Components.skeletonHeading('40%')}
          <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">${Components.skeletonText(4)}</div>
          <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">${Components.skeletonText(3)}</div>
          <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">${Components.skeletonText(5)}</div>
          <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">${Components.skeletonText(3)}</div>
        </div>
      `;
    }

    if (error) {
      return html`<div dangerouslySetInnerHTML=${{ __html: Helpers.errorMessage(error) }}></div>`;
    }

    return html`
      ${isDefaultPw ? html`<${PasswordWarningBanner} />` : null}
      <div class="p-4 md:p-6 anim-fade-in" style="max-width:48rem;margin:0 auto">
        <h1 class="text-xl" style="font-weight:var(--font-weight-bold);color:var(--color-text);letter-spacing:-0.025em;margin-bottom:1.5rem" data-tooltip=${t('tooltip_settings')}>${t('settings_title')}</h1>

        <${SystemInfoCard} info=${sysInfo} />

        <${SectionCard} title=${t('tokens_title')} noPad=${true}>
          <${TokenList} />
        <//>

        <${SectionCard} title=${t('password_change')} noPad=${true}>
          <${PasswordForm} />
        <//>

        <${SectionCard} title=${t('about')}>
          <p class="text-sm" style="color:var(--color-text-secondary)">${t('about_desc')}</p>
          <p class="text-xs" style="margin-top:0.5rem;color:var(--color-text-tertiary)">${t('about_sub')}</p>
        <//>

        <${SectionCard} title=${t('settings_disk_threshold')} noPad=${true}>
          <${DiskThresholdCard} />
        <//>

        <${SectionCard} title=${t('storage_paths_title')} noPad=${true}>
          <${StoragePathsCard} />
        <//>

        <${SectionCard} title=${t('nav.external_services')}>
          <div class="flex items-center justify-between">
            <div>
              <p class="text-sm" style="color:var(--color-text-secondary)">${t('webdav_title')}</p>
              <p class="text-xs" style="margin-top:0.25rem;color:var(--color-text-tertiary)">${t('webdav_auth_note')}</p>
            </div>
            <span class="badge badge-default">${t('webdav_disabled')}</span>
          </div>
        <//>

        <${SectionCard} title=${t('backup_restore_title')}>
          <p class="text-xs" style="margin-bottom:1rem;color:var(--color-text-tertiary)">${t('backup_restore_desc')}</p>
          <${BackupList} />
        <//>

        <button class="btn btn-danger" style="width:100%;padding:0.75rem" onClick=${handleLogout}>${t('nav_logout')}</button>
      </div>
    `;
  }

  // ── Module API ────────────────────────────────────────────────────────

  var _mounted = false;
  var _rootEl = null;

  function renderPage() {
    var el = document.getElementById('main-content');
    if (!el) return;
    _rootEl = el;
    _mounted = true;
    render(html`<${SettingsPage} />`, el);
  }

  function cleanup() {
    if (_mounted && _rootEl) {
      render(null, _rootEl);
      _mounted = false;
      _rootEl = null;
    }
  }

  return {
    render: renderPage,
    cleanup: cleanup
  };
})();
