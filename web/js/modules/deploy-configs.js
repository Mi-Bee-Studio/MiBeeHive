const DeployConfigs = (function () {
  'use strict';
  var html = PreactBridge.html;
  var preactRender = PreactBridge.render;
  var useState = PreactBridge.useState;
  var useEffect = PreactBridge.useEffect;
  var useRef = PreactBridge.useRef;
  var useCallback = PreactBridge.useCallback;
  var useMemo = PreactBridge.useMemo;

  // ── Constants ────────────────────────────────────────────────────────
  var TYPE_ORDER = ['debian', 'ubuntu', 'centos', 'rocky', 'alma', 'fedora', 'opensuse'];
  var OS_LABELS = { debian: 'Debian', ubuntu: 'Ubuntu', centos: 'CentOS', rocky: 'Rocky Linux', alma: 'AlmaLinux', fedora: 'Fedora', opensuse: 'openSUSE' };
  var OS_BADGES = { debian: 'badge-blue', ubuntu: 'badge-orange', centos: 'badge-purple', rocky: 'badge-blue', alma: 'badge-cyan', fedora: 'badge-purple', opensuse: 'badge-default' };
  var PXE_FMT = { debian: 'preseed', centos: 'kickstart', ubuntu: 'autoinstall', rocky: 'kickstart', alma: 'kickstart', fedora: 'kickstart', opensuse: 'autoinstall' };

  var ACCORDION_ID = 'configs-list-accordion';

  // ── Sub-navigation ───────────────────────────────────────────────────
  function _subNav(active) {
    return '<nav class="module-tabs">' +
      '<a href="#/deploy" class="module-tab' + (active === 'configs' ? ' active' : '') + '" data-tooltip="' + t('tooltip_deploy') + '">' + t('nav_deploy_configs') + '</a>' +
      '<a href="#/deploy/iso" class="module-tab' + (active === 'iso' ? ' active' : '') + '">' + t('nav_deploy_iso') + '</a></nav>';
  }

  // ── Config Row Component ─────────────────────────────────────────────
  function ConfigRow(props) {
    var c = props.config;
    var fmt = PXE_FMT[c.os_type] || 'preseed';
    var url = location.protocol + '//' + location.host + '/pxe/' + fmt + '/' + encodeURIComponent(c.config_name);
    var E = Helpers.escapeHtml;

    function handleCopyUrl() {
      Helpers.copyToClipboard(url).then(function () { showToast(t('osinstall_url_copied'), 'success'); });
    }
    function handleEdit() { props.onEdit(c); }
    function handleView() { props.onView(c); }
    function handleDelete() { props.onDelete(c); }

    return html`
      <tr>
        <td><div class="font-medium" style="color:var(--color-text)">${E(c.name)}</div></td>
        <td><span class="text-xs" style="color:var(--color-text-tertiary)">${E(c.config_name)}</span></td>
        <td><span class="badge ${c.enabled ? 'badge-success' : 'badge-default'}" style="font-size:0.6875rem">${c.enabled ? t('osinstall_enabled') : t('osinstall_disabled')}</span></td>
        <td>
          <div class="flex items-center gap-1">
            <span class="text-xs truncate" style="color:var(--color-text-tertiary);max-width:12rem" title="${E(url)}">${E(url)}</span>
            <button class="btn btn-ghost btn-sm" style="padding:0.125rem 0.375rem"
                    aria-label="${t('aria_copy_url')}" title="${t('osinstall_copy_url')}" onClick=${handleCopyUrl}>
              <svg aria-hidden="true" class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
            </button>
          </div>
        </td>
        <td style="text-align:right">
          <div class="flex justify-end gap-2">
            <button class="btn btn-ghost btn-sm" onClick=${handleEdit}>${t('osinstall_edit')}</button>
            <button class="btn btn-ghost btn-sm" onClick=${handleView}>${t('osinstall_text_view')}</button>
            <button class="btn btn-ghost btn-danger-outline btn-sm" onClick=${handleDelete}>${t('osinstall_delete')}</button>
          </div>
        </td>
      </tr>`;
  }

  // ── Config Section (grouped by os_type) ──────────────────────────────
  function ConfigSection(props) {
    var _open = useState(true);
    var open = _open[0], setOpen = _open[1];

    var configs = props.configs;
    var title = props.title;
    var count = configs.length;

    return html`
      <div class="accordion-section" style="border:1px solid var(--color-border);border-radius:var(--radius-md);margin-bottom:0.5rem;overflow:hidden">
        <button class="accordion-header" onClick=${function () { setOpen(!open); }}
          style="width:100%;display:flex;align-items:center;justify-content:space-between;padding:0.75rem 1rem;background:var(--color-bg-secondary);border:none;cursor:pointer;color:var(--color-text);font-size:0.875rem;font-weight:var(--font-weight-semibold)">
          <div class="flex items-center gap-2">
            <span class="badge ${OS_BADGES[props.type] || 'badge-default'}" style="font-size:0.6875rem">${title}</span>
            <span class="text-xs" style="color:var(--color-text-tertiary)">${count}</span>
          </div>
          <svg style="transform:rotate(${open ? '180' : '0'}deg);transition:transform 0.2s;width:1rem;height:1rem;color:var(--color-text-quaternary)" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24"><path d="M6 9l6 6 6-6"/></svg>
        </button>
        ${open ? html`
          <div class="table-wrap overflow-x-auto">
            <table>
              <thead><tr>
                <th>${t('osinstall_name')}</th><th>${t('osinstall_config_name')}</th>
                <th>${t('proj_enabled')}</th><th>${t('osinstall_pxe_url')}</th><th style="text-align:right">${t('actions')}</th>
              </tr></thead>
              <tbody>
                ${configs.map(function (c) {
                  return html`<${ConfigRow} key=${c.id} config=${c}
                    onEdit=${props.onEdit} onView=${props.onView} onDelete=${props.onDelete} />`;
                })}
              </tbody>
            </table>
          </div>` : null}
      </div>`;
  }

  // ── Text View Modal ──────────────────────────────────────────────────
  function TextViewModal(props) {
    var config = props.config;
    var obj = {};
    try { obj = JSON.parse(config.config || '{}'); } catch (e) { /* */ }
    var pretty = JSON.stringify(obj, null, 2);

    function handleCopy() {
      Helpers.copyToClipboard(pretty).then(function () { showToast(t('osinstall_config_copied'), 'success'); });
    }

    return html`
      <div ref=${props.overlayRef} class="modal-overlay" onClick=${props.handleOverlayClick} onKeyDown=${props.handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" style="max-width:40rem">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-base font-semibold" style="color:var(--color-text)">${Helpers.escapeHtml(config.name)}</h3>
            <button class="btn btn-ghost btn-sm" onClick=${handleCopy}>${t('osinstall_copy_config')}</button>
          </div>
          <pre style="max-height:60vh;overflow:auto;font-size:0.8125rem;padding:1rem;background:var(--color-bg-secondary);border-radius:var(--radius-md)">${pretty}</pre>
          <div class="flex justify-end mt-4">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('close')}</button>
          </div>
        </div>
      </div>`;
  }

  // ── Preview Modal ────────────────────────────────────────────────────
  function PreviewModal(props) {
    var content = props.content;

    return html`
      <div ref=${props.overlayRef} class="modal-overlay" onClick=${props.handleOverlayClick} onKeyDown=${props.handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" style="max-width:40rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${t('osinstall_preview_title')}</h3>
          <pre style="max-height:60vh;overflow:auto;font-size:0.8125rem;padding:1rem;background:var(--color-bg-secondary);border-radius:var(--radius-md)">${content}</pre>
          <div class="flex justify-end mt-4">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('close')}</button>
          </div>
        </div>
      </div>`;
  }

  // ── Config Editor Modal ──────────────────────────────────────────────
  function ConfigEditorModal(props) {
    var config = props.config;
    var isEdit = !!config;
    var params = {};
    if (isEdit) { try { params = JSON.parse(config.config || '{}'); } catch (e) { /* */ } }

    var _activeTab = useState('basic');
    var activeTab = _activeTab[0], setActiveTab = _activeTab[1];
    var _submitting = useState(false);
    var submitting = _submitting[0], setSubmitting = _submitting[1];
    var _previewing = useState(false);
    var previewing = _previewing[0], setPreviewing = _previewing[1];
    var _previewContent = useState(null);
    var previewContent = _previewContent[0], setPreviewContent = _previewContent[1];
    var _errors = useState({});
    var errors = _errors[0], setErrors = _errors[1];

    // Form state
    var _name = useState(isEdit ? config.name : '');
    var name = _name[0], setName = _name[1];
    var _configName = useState(isEdit ? (config.config_name || '') : '');
    var configName = _configName[0], setConfigName = _configName[1];
    var _osType = useState(isEdit ? config.os_type : '');
    var osType = _osType[0], setOsType = _osType[1];
    var _hostname = useState(params.hostname || '');
    var hostname = _hostname[0], setHostname = _hostname[1];
    var _timezone = useState(params.timezone || '');
    var timezone = _timezone[0], setTimezone = _timezone[1];
    var _language = useState(params.language || '');
    var language = _language[0], setLanguage = _language[1];
    var _keyboard = useState(params.keyboard_layout || '');
    var keyboard = _keyboard[0], setKeyboard = _keyboard[1];
    var _dhcp = useState(!params.ip_address);
    var dhcp = _dhcp[0], setDhcp = _dhcp[1];
    var _ip = useState(params.ip_address || '');
    var ip = _ip[0], setIp = _ip[1];
    var _netmask = useState(params.netmask || '');
    var netmask = _netmask[0], setNetmask = _netmask[1];
    var _gateway = useState(params.gateway || '');
    var gateway = _gateway[0], setGateway = _gateway[1];
    var _dns = useState(params.dns_servers || '');
    var dns = _dns[0], setDns = _dns[1];
    var _username = useState(params.username || '');
    var username = _username[0], setUsername = _username[1];
    var _password = useState(params.user_password || '');
    var password = _password[0], setPassword = _password[1];
    var _sshKey = useState(params.user_ssh_key || '');
    var sshKey = _sshKey[0], setSshKey = _sshKey[1];
    var _disk = useState(params.disk || '');
    var disk = _disk[0], setDisk = _disk[1];
    var _partition = useState(params.partition_scheme || 'whole_disk');
    var partition = _partition[0], setPartition = _partition[1];
    var _packages = useState(params.packages ? params.packages.join('\n') : '');
    var packages = _packages[0], setPackages = _packages[1];
    var _additional = useState(params.additional_config || '');
    var additional = _additional[0], setAdditional = _additional[1];

    function gatherParams() {
      var raw = packages.trim();
      var pkgs = raw ? raw.split(/[\n,]/).map(function (s) { return s.trim(); }).filter(Boolean) : [];
      return {
        hostname: hostname.trim(), timezone: timezone.trim(), language: language.trim(),
        keyboard_layout: keyboard.trim(),
        ip_address: dhcp ? '' : ip.trim(), netmask: dhcp ? '' : netmask.trim(),
        gateway: dhcp ? '' : gateway.trim(), dns_servers: dhcp ? '' : dns.trim(),
        username: username.trim(), user_password: password.trim(), user_ssh_key: sshKey.trim(),
        disk: disk.trim(), partition_scheme: partition,
        packages: pkgs, additional_config: additional.trim()
      };
    }

    function handleSubmit() {
      var newErrors = {};
      if (!name.trim()) newErrors.name = t('validation_required');
      if (!osType) newErrors.osType = t('osinstall_os_type_required');
      if (hostname.trim() && /\s/.test(hostname.trim())) newErrors.hostname = t('validation_hostname_format');
      if (!dhcp) {
        if (ip.trim() && !Helpers.validateIP(ip.trim())) newErrors.ip = t('validation_ip_format');
        if (netmask.trim() && !Helpers.validateNetmask(netmask.trim())) newErrors.netmask = t('validation_netmask_format');
        if (gateway.trim() && !Helpers.validateIP(gateway.trim())) newErrors.gateway = t('validation_ip_format');
        if (dns.trim() && !Helpers.validateIP(dns.trim())) newErrors.dns = t('validation_ip_format');
      }
      setErrors(newErrors);
      if (Object.keys(newErrors).length) return;

      setSubmitting(true);
      var body = { name: name.trim(), config_name: configName.trim() || name.trim(), os_type: osType, params: gatherParams() };
      var req = isEdit ? Api.put('/admin/os-install/configs/' + config.id, body) : Api.post('/admin/os-install/configs', body);
      req.then(function (r) {
        setSubmitting(false);
        if (!r || !r.success) { showToast((r && r.message) || t('error'), 'error'); return; }
        showToast(isEdit ? t('osinstall_updated') : t('osinstall_created'), 'success');
        props.onClose();
        props.onSaved();
      });
    }

    function handlePreview() {
      if (!osType) { showToast(t('osinstall_select_os_type'), 'error'); return; }
      setPreviewing(true);
      Api.post('/admin/os-install/configs/preview', { os_type: osType, params: gatherParams() }).then(function (r) {
        setPreviewing(false);
        if (!r || !r.success) { showToast(t('osinstall_preview_error') + ': ' + ((r && r.message) || t('error')), 'error'); return; }
        setPreviewContent((r.data || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;'));
      });
    }

    var TABS = [
      { id: 'basic', label: t('osi_tab_basic') },
      { id: 'network', label: t('osi_tab_network') },
      { id: 'user', label: t('osi_tab_user') },
      { id: 'disk', label: t('osi_tab_disk') },
      { id: 'packages', label: t('osi_tab_packages') },
      { id: 'additional', label: t('osi_tab_additional') }
    ];

    var lblStyle = 'color:var(--color-text-secondary)';
    var fz = 'font-size:0.8125rem';

    return html`
      <div ref=${props.overlayRef} class="modal-overlay" onClick=${props.handleOverlayClick} onKeyDown=${props.handleKeyDown}>
        <div class="modal-content" role="dialog" aria-modal="true" style="max-width:40rem">
          <h3 class="text-base font-semibold mb-4" style="color:var(--color-text)">${isEdit ? t('osinstall_edit') : t('osinstall_new')}</h3>

          <div class="osi-modal-tabs">
            ${TABS.map(function (tb) {
              return html`<button class="osi-modal-tab ${activeTab === tb.id ? 'osi-modal-tab-active' : ''}"
                onClick=${function () { setActiveTab(tb.id); }}>${tb.label}</button>`;
            })}
          </div>

          <div class="osi-tab-panels">
            ${activeTab === 'basic' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_name')}</label>
                      <input class="input" placeholder="e.g. my-server" value=${name} style="${fz}"
                        onInput=${function (e) { setName(e.target.value); }} />
                      ${errors.name ? html`<span class="text-xs" style="color:var(--color-error)">${errors.name}</span>` : null}
                    </div>
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_config_name')}</label>
                      <input class="input" placeholder="my-server" value=${configName} style="${fz}"
                        onInput=${function (e) { setConfigName(e.target.value); }} />
                    </div>
                  </div>
                  <div>
                    <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_os_type')}</label>
                    <select class="input select" style="${fz}" value=${osType}
                      onChange=${function (e) { setOsType(e.target.value); }}>
                      <option value="" disabled>--</option>
                      ${Object.keys(OS_LABELS).map(function (k) {
                        return html`<option value=${k}>${OS_LABELS[k]}</option>`;
                      })}
                    </select>
                    ${errors.osType ? html`<span class="text-xs" style="color:var(--color-error)">${errors.osType}</span>` : null}
                  </div>
                  <div class="grid grid-cols-3 gap-3">
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_hostname')}</label>
                      <input class="input" placeholder="server01" value=${hostname} style="${fz}"
                        onInput=${function (e) { setHostname(e.target.value); }} />
                      ${errors.hostname ? html`<span class="text-xs" style="color:var(--color-error)">${errors.hostname}</span>` : null}
                    </div>
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_timezone')}</label>
                      <input class="input" placeholder="Asia/Shanghai" value=${timezone} style="${fz}"
                        onInput=${function (e) { setTimezone(e.target.value); }} />
                    </div>
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_language')}</label>
                      <input class="input" placeholder="en_US" value=${language} style="${fz}"
                        onInput=${function (e) { setLanguage(e.target.value); }} />
                    </div>
                  </div>
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_keyboard')}</label>
                      <input class="input" placeholder="us" value=${keyboard} style="${fz}"
                        onInput=${function (e) { setKeyboard(e.target.value); }} />
                    </div>
                  </div>
                </div>
              </div>` : null}

            ${activeTab === 'network' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <div class="flex items-center gap-3 mb-2">
                    <label class="toggle-switch">
                      <input type="checkbox" role="switch" checked=${dhcp}
                        aria-checked="${dhcp ? 'true' : 'false'}"
                        onChange=${function () { setDhcp(!dhcp); }} />
                      <span class="toggle-slider"></span>
                    </label>
                    <span class="text-sm" style="${lblStyle}">${t('osinstall_dhcp')}</span>
                  </div>
                  <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_osi_dhcp')}</span>
                  ${!dhcp ? html`
                    <div class="grid gap-3">
                      <div class="grid grid-cols-2 gap-3">
                        <div>
                          <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_ip_address')}</label>
                          // Default placeholder IP for OS install config form (example value)
                          <input class="input" placeholder="192.168.1.100" value=${ip} style="${fz}"
                            onInput=${function (e) { setIp(e.target.value); }} />
                          ${errors.ip ? html`<span class="text-xs" style="color:var(--color-error)">${errors.ip}</span>` : null}
                        </div>
                        <div>
                          <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_netmask')}</label>
                          <input class="input" placeholder="255.255.255.0" value=${netmask} style="${fz}"
                            onInput=${function (e) { setNetmask(e.target.value); }} />
                          ${errors.netmask ? html`<span class="text-xs" style="color:var(--color-error)">${errors.netmask}</span>` : null}
                        </div>
                      </div>
                      <div class="grid grid-cols-2 gap-3">
                        <div>
                          <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_gateway')}</label>
                          <input class="input" placeholder="192.168.1.1" value=${gateway} style="${fz}"
                            onInput=${function (e) { setGateway(e.target.value); }} />
                          ${errors.gateway ? html`<span class="text-xs" style="color:var(--color-error)">${errors.gateway}</span>` : null}
                        </div>
                        <div>
                          <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_dns')}</label>
                          <input class="input" placeholder="8.8.8.8" value=${dns} style="${fz}"
                            onInput=${function (e) { setDns(e.target.value); }} />
                          ${errors.dns ? html`<span class="text-xs" style="color:var(--color-error)">${errors.dns}</span>` : null}
                        </div>
                      </div>
                      <span class="text-xs" style="color:var(--color-text-tertiary)">${t('help_osi_network')}</span>
                    </div>` : null}
                </div>
              </div>` : null}

            ${activeTab === 'user' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_username')}</label>
                      <input class="input" placeholder="admin" value=${username} style="${fz}"
                        onInput=${function (e) { setUsername(e.target.value); }} />
                    </div>
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_password')}</label>
                      <input type="password" class="input" placeholder="" value=${password} style="${fz}"
                        onInput=${function (e) { setPassword(e.target.value); }} />
                    </div>
                  </div>
                  <div>
                    <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_ssh_key')}</label>
                    <textarea class="input" rows="3" placeholder="ssh-rsa AAA..." style="${fz};resize:vertical"
                      onInput=${function (e) { setSshKey(e.target.value); }}>${sshKey}</textarea>
                  </div>
                </div>
              </div>` : null}

            ${activeTab === 'disk' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <div class="grid grid-cols-2 gap-3">
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_disk')}</label>
                      <input class="input" placeholder="/dev/sda" value=${disk} style="${fz}"
                        onInput=${function (e) { setDisk(e.target.value); }} />
                    </div>
                    <div>
                      <label class="text-xs font-medium" style="${lblStyle}">${t('osinstall_partition')}</label>
                      <select class="input select" style="${fz}" value=${partition}
                        onChange=${function (e) { setPartition(e.target.value); }}>
                        <option value="whole_disk">${t('osinstall_whole_disk')}</option>
                        <option value="manual">${t('osinstall_manual')}</option>
                      </select>
                    </div>
                  </div>
                </div>
              </div>` : null}

            ${activeTab === 'packages' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <textarea class="input" rows="6" placeholder="openssh-server, curl, wget" style="${fz};resize:vertical"
                    onInput=${function (e) { setPackages(e.target.value); }}>${packages}</textarea>
                </div>
              </div>` : null}

            ${activeTab === 'additional' ? html`
              <div class="osi-tab-panel" style="display:block">
                <div class="grid gap-4">
                  <textarea class="input" rows="4" placeholder="${t('osinstall_additional')}" style="${fz};resize:vertical"
                    onInput=${function (e) { setAdditional(e.target.value); }}>${additional}</textarea>
                </div>
              </div>` : null}
          </div>

          <div class="flex justify-end gap-3 mt-6">
            <button class="btn btn-secondary" onClick=${props.onClose}>${t('cancel')}</button>
            <button class="btn btn-ghost" disabled=${!osType || previewing} onClick=${handlePreview}>
              ${previewing ? '...' : t('osinstall_preview')}
            </button>
            <button class="btn btn-primary" onClick=${handleSubmit} disabled=${submitting}>
              ${submitting ? '...' : t('save')}
            </button>
          </div>
        </div>
      </div>

      ${previewContent !== null ? html`
        <${PreviewModal}
          content=${previewContent}
          overlayRef=${useRef(null)}
          onClose=${function () { setPreviewContent(null); }}
          handleOverlayClick=${function (e) { if (e.target === e.currentTarget) setPreviewContent(null); }}
          handleKeyDown=${function (e) { if (e.key === 'Escape') setPreviewContent(null); }}
        />` : null}`;
  }

  // ── Main Page Component ──────────────────────────────────────────────
  function ConfigsPage() {
    var _configs = useState([]);
    var configs = _configs[0], setConfigs = _configs[1];
    var _loading = useState(true);
    var loading = _loading[0], setLoading = _loading[1];
    var _error = useState(null);
    var error = _error[0], setError = _error[1];
    var _showModal = useState(false);
    var showModal = _showModal[0], setShowModal = _showModal[1];
    var _editConfig = useState(null);
    var editConfig = _editConfig[0], setEditConfig = _editConfig[1];
    var _viewConfig = useState(null);
    var viewConfig = _viewConfig[0], setViewConfig = _viewConfig[1];
    var mountedRef = useRef(true);
    var overlayRef = useRef(null);
    var previousFocusRef = useRef(null);

    function loadConfigs() {
      Api.get('/admin/os-install/configs').then(function (r) {
        if (!mountedRef.current) return;
        if (!r || !r.success) {
          setError(t('error_load_failed'));
          setLoading(false);
          return;
        }
        setConfigs(r.data || []);
        setError(null);
        setLoading(false);
      });
    }

    useEffect(function () {
      mountedRef.current = true;
      loadConfigs();
      return function () { mountedRef.current = false; };
    }, []);

    // Group configs by os_type
    var groups = useMemo(function () {
      var g = {};
      configs.forEach(function (c) {
        var type = c.os_type || 'other';
        if (!g[type]) g[type] = [];
        g[type].push(c);
      });
      var sections = [];
      // Known types first
      TYPE_ORDER.forEach(function (type) {
        if (g[type]) {
          sections.push({ type: type, title: OS_LABELS[type], configs: g[type] });
          delete g[type];
        }
      });
      // Remaining types
      Object.keys(g).sort().forEach(function (type) {
        sections.push({ type: type, title: type.charAt(0).toUpperCase() + type.slice(1), configs: g[type] });
      });
      return sections;
    }, [configs]);

    function handleEdit(c) {
      previousFocusRef.current = document.activeElement;
      setEditConfig(c);
    }
    function handleView(c) {
      previousFocusRef.current = document.activeElement;
      setViewConfig(c);
    }
    function handleDelete(c) {
      showConfirmModal(t('osinstall_delete_confirm', { name: c.name }), function (mb) {
        var o = mb.textContent; mb.disabled = true; mb.textContent = '...';
        Api.delete('/admin/os-install/configs/' + c.id).then(function (r) {
          if (!r || !r.success) { mb.disabled = false; mb.textContent = o; showToast((r && r.message) || t('error'), 'error'); return; }
          showToast(t('osinstall_deleted'), 'success');
          loadConfigs();
        });
      });
    }
    function handleAdd() {
      previousFocusRef.current = document.activeElement;
      setShowModal(true);
    }
    function closeModal() {
      setShowModal(false);
      setEditConfig(null);
      if (previousFocusRef.current && previousFocusRef.current.focus) previousFocusRef.current.focus();
    }
    function closeViewModal() {
      setViewConfig(null);
      if (previousFocusRef.current && previousFocusRef.current.focus) previousFocusRef.current.focus();
    }
    function handleOverlayClick(e) {
      if (e.target === e.currentTarget) closeModal();
    }
    function handleKeyDown(e) {
      if (e.key === 'Escape') closeModal();
    }
    function handleViewOverlayClick(e) {
      if (e.target === e.currentTarget) closeViewModal();
    }
    function handleViewKeyDown(e) {
      if (e.key === 'Escape') closeViewModal();
    }

    // ── Loading state ─────────────────────────────────────────────────
    if (loading) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _subNav('configs') }} />
          <div class="flex items-center justify-between mb-5">
            <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('osinstall_title')}</h2>
            <button class="btn btn-primary btn-sm" disabled>${t('osinstall_new')}</button>
          </div>
          <div dangerouslySetInnerHTML=${{ __html: Components.skeletonTable(4, 6) }} />
        </div>`;
    }

    // ── Error state ───────────────────────────────────────────────────
    if (error) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _subNav('configs') }} />
          <div class="anim-fade-in empty-state">
            <div style="color:var(--color-text-quaternary);margin-bottom:0.75rem"
              dangerouslySetInnerHTML=${{ __html: Helpers.ICONS.inbox }} />
            <p class="text-sm font-medium" style="color:var(--color-text-tertiary);margin-bottom:1rem">${error}</p>
            <button class="btn btn-primary btn-sm" onClick=${function () { setLoading(true); setError(null); loadConfigs(); }}>
              ${t('error_retry')}
            </button>
          </div>
        </div>`;
    }

    // ── Empty state ───────────────────────────────────────────────────
    if (!configs.length) {
      return html`
        <div class="p-4 md:p-6 max-w-7xl mx-auto">
          <div dangerouslySetInnerHTML=${{ __html: _subNav('configs') }} />
          <div class="flex items-center justify-between mb-5">
            <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('osinstall_title')}</h2>
            <button class="btn btn-primary btn-sm"
              onClick=${handleAdd}>${t('osinstall_new')}</button>
          </div>
          <div dangerouslySetInnerHTML=${{ __html: Components.emptyState({
            message: t('osinstall_empty'),
            description: t('cta_create_os_config_desc'),
            actionLabel: t('cta_create_os_config')
          }) }} />
          <p class="text-xs" style="color:var(--color-text-tertiary);margin-top:0.5rem">${t('deploy_empty_help')}</p>
        </div>`;
    }

    // ── Main content ──────────────────────────────────────────────────
    return html`
      <div class="p-4 md:p-6 max-w-7xl mx-auto">
        <div dangerouslySetInnerHTML=${{ __html: _subNav('configs') }} />
        <div class="flex items-center justify-between mb-5">
          <h2 class="text-base font-semibold" style="color:var(--color-text)">${t('osinstall_title')}</h2>
          <button class="btn btn-primary btn-sm"
            onClick=${handleAdd}>${t('osinstall_new')}</button>
        </div>
        <div>
          ${groups.map(function (g) {
            return html`<${ConfigSection} key=${g.type} type=${g.type} title=${g.title} configs=${g.configs}
              onEdit=${handleEdit} onView=${handleView} onDelete=${handleDelete} />`;
          })}
        </div>

        ${showModal ? html`
          <${ConfigEditorModal}
            overlayRef=${overlayRef}
            onClose=${closeModal}
            onSaved=${loadConfigs}
            handleOverlayClick=${handleOverlayClick}
            handleKeyDown=${handleKeyDown}
          />` : null}

        ${editConfig ? html`
          <${ConfigEditorModal}
            config=${editConfig}
            overlayRef=${overlayRef}
            onClose=${closeModal}
            onSaved=${loadConfigs}
            handleOverlayClick=${handleOverlayClick}
            handleKeyDown=${handleKeyDown}
          />` : null}

        ${viewConfig ? html`
          <${TextViewModal}
            config=${viewConfig}
            overlayRef=${useRef(null)}
            onClose=${closeViewModal}
            handleOverlayClick=${handleViewOverlayClick}
            handleKeyDown=${handleViewKeyDown}
          />` : null}
      </div>`;
  }

  // ── Public API ───────────────────────────────────────────────────────
  function render() {
    var app = document.getElementById('main-content');
    if (!app) return;
    preactRender(html`<${ConfigsPage} />`, app);
  }

  function destroy() {
    Components.Accordion.destroy(ACCORDION_ID);
    var app = document.getElementById('main-content');
    if (app) preactRender(null, app);
  }

  return { render: render, destroy: destroy, _subNav: _subNav };
})();
