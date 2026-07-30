const ContainersTemplates = (function() {
  'use strict';
  var _init=false, _cachedTemplates=null;
  var CC={web:'badge-blue',database:'badge-purple',cache:'badge-cyan',messaging:'badge-orange',monitoring:'badge-success',tool:'badge-default'};
  function _subNav(active) {
    return '<nav class="module-tabs">' +
      '<a href="#/containers" class="module-tab' + (active === 'containers' ? ' active' : '') + '">' + t('nav_containers') + '</a>' +
      '<a href="#/containers/images" class="module-tab' + (active === 'images' ? ' active' : '') + '">' + t('nav_containers_images') + '</a>' +
      '<a href="#/containers/templates" class="module-tab' + (active === 'templates' ? ' active' : '') + '">' + t('nav_containers_templates') + '</a>' +
      '<a href="#/containers/registries" class="module-tab' + (active === 'registries' ? ' active' : '') + '">' + t('nav_registries') + '</a>' +
      '</nav>';
  }

  function cardHtml(t) {
    var h=Helpers.escapeHtml,d=(t.description||'');if(d.length>60)d=d.substring(0,57)+'...';
    var pp=(t.ports||[]).map(function(p){return h(p.host_port+':'+p.container_port);}).join(', ');
    return '<div class="card card-hover anim-fade-in" style="padding:1.25rem;display:flex;flex-direction:column;gap:0.75rem">'+
      '<div class="flex items-center justify-between"><span class="text-sm font-semibold truncate" style="color:var(--color-text)">'+h(t.name)+'</span>'+
      '<span class="badge '+(CC[t.category]||'badge-default')+'">'+h(t.category||t('container_category_other'))+'</span></div>'+
      '<p class="text-xs" style="color:var(--color-text-tertiary);line-height:1.5">'+h(d)+'</p>'+
      '<div class="flex items-center gap-2 flex-wrap"><span class="text-xs font-mono" style="color:var(--color-text-secondary);background:var(--color-bg-secondary);padding:1px 6px;border-radius:var(--radius-sm)">'+h(t.image)+'</span>'+
      (pp?'<span class="text-xs" style="color:var(--color-text-tertiary)">'+pp+'</span>':'')+'</div>'+
      '<div class="mt-auto"><button class="btn btn-primary btn-sm w-full" data-deploy="'+t.id+'">'+t('container_deploy')+'</button></div></div>';
  }

  function renderGrid(templates) {
    var grid=document.getElementById('tpl-grid');if(!grid)return;
    if(!templates.length){grid.innerHTML='<div class="empty-state"><p class="text-sm" style="color:var(--color-text-tertiary)">'+t('cta_no_templates')+'</p><p class="text-xs" style="color:var(--color-text-quaternary);margin-top:0.5rem">'+t('cta_no_templates_desc')+'</p></div>';return;}
    var g={},o=[];templates.forEach(function(t){var c=t.category||'other';if(!g[c]){g[c]=[];o.push(c);}g[c].push(t);});
    var html='';o.forEach(function(cat){
      html+='<div class="mb-5"><h3 class="text-xs font-semibold uppercase tracking-wider mb-2" style="color:var(--color-text-tertiary)">'+Helpers.escapeHtml(cat)+'</h3><div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">';
      g[cat].forEach(function(t){html+=cardHtml(t);});html+='</div></div>';});
    grid.innerHTML=html;
  }

  function fld(id,lbl,val,ro) {
    var h=Helpers.escapeHtml,is='class="input" style="width:100%;margin-top:2px"';
    return '<div><label class="text-xs font-medium" style="color:var(--color-text-tertiary)">'+lbl+'</label>'+(ro?
      '<input id="'+id+'" '+is+' value="'+h(val||'')+'" readonly style="opacity:0.7">':
      '<textarea id="'+id+'" '+is+' rows="2">'+h(val||'')+'</textarea>')+'</div>';
  }

  function showDeployModal(tpl) {
    var h=Helpers.escapeHtml,
      pv=(tpl.ports||[]).map(function(p){return p.host_port+':'+p.container_port;}).join('\n'),
      ev=tpl.env?Object.keys(tpl.env).map(function(k){return k+'='+tpl.env[k];}).join('\n'):'';
    var bodyHtml='<div class="flex flex-col gap-3">'+
      fld('tpl-nm',t('container_name_label'),tpl.name)+fld('tpl-img',t('container_image_label'),tpl.image,1)+
      fld('tpl-ports',t('container_ports_label'),pv)+fld('tpl-env',t('container_env_label'),ev)+
      '</div><div class="flex justify-end gap-3 mt-4"><button id="tpl-no" class="btn btn-secondary">'+t('cancel')+'</button><button id="tpl-go" class="btn btn-primary">'+t('container_deploy')+'</button></div>';
    var modal = Components.createModal({
      title: t('container_deploy_title', {name: tpl.name}),
      bodyHtml: bodyHtml,
      size: '28rem',
      onMount: function(overlay) {
        overlay.querySelector('#tpl-no').addEventListener('click', modal.close);
        overlay.querySelector('#tpl-go').addEventListener('click', async function(){
          var nm=document.getElementById('tpl-nm').value.trim();
          if(!nm){Components.showToast(t('container_name_required'),'error');return;}
          var b={name:nm,image:tpl.image};if(tpl.command)b.command=tpl.command;if(tpl.restart_policy)b.restart_policy=tpl.restart_policy;
          var pts=document.getElementById('tpl-ports').value.trim();
          if(pts)b.ports=pts.split('\n').filter(function(l){return l.trim();}).map(function(l){var p=l.trim().split(':');return{host_port:parseInt(p[0])||0,container_port:parseInt(p[1]||p[0])||0,protocol:'tcp'};});
          var evs=document.getElementById('tpl-env').value.trim();
          if(evs){b.env={};evs.split('\n').filter(function(l){return l.trim();}).forEach(function(l){var i=l.indexOf('=');if(i>0)b.env[l.substring(0,i).trim()]=l.substring(i+1).trim();});}
          var r=await Api.post('/admin/containers',b);modal.close();if(r&&r.success)Components.showToast(tpl.name+' '+t('container_deployed'),'success');
        });
      }
    });
  }

  async function handleGridClick(e) {
    var btn=e.target.closest('[data-deploy]');if(!btn)return;
    var res=await Api.get('/admin/templates/'+btn.getAttribute('data-deploy'));
    if(res&&res.success&&res.data)showDeployModal(res.data);
  }

  async function loadData() {
    var grid=document.getElementById('tpl-grid');if(!grid)return;
    var res=await Api.get('/admin/templates');
    if(!res||!res.success){_init=true;return;}
    renderGrid(res.data||[]);_init=true;_cachedTemplates=res.data||[];
  }

  function render() {
    destroy();var mc=document.getElementById('main-content');if(!mc)return;
    var sk='<div class="card" style="padding:1.25rem"><div class="skeleton" style="height:1rem;width:50%;margin-bottom:0.5rem"></div><div class="skeleton" style="height:0.75rem;width:80%;margin-bottom:0.75rem"></div><div class="skeleton" style="height:2rem"></div></div>';
    mc.innerHTML='<div class="p-4 md:p-6 max-w-7xl mx-auto">'+
      _subNav('templates')+
      '<h2 class="text-base font-semibold mb-4" style="color:var(--color-text)">'+t('template_list')+'</h2>'+
      '<div id="tpl-grid"><div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3">'+
      (_cachedTemplates ? '' : sk.repeat(3)) +'</div></div></div>';
    var grid=document.getElementById('tpl-grid');if(grid)grid.addEventListener('click',handleGridClick);
    if (_cachedTemplates) { renderGrid(_cachedTemplates); _init = true; }
    loadData();
  }
  function destroy(){_init=false;}

  return {render:render,destroy:destroy};
})();
