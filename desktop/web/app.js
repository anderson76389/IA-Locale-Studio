'use strict';const $=id=>document.getElementById(id);let token='',kind='texte',status={},job={},timer=null,threads={},pending=null;const config={texte:['Créer du texte','Discutez avec un modèle local via Ollama.'],code:['Créer du code','Programmez avec un modèle local via Ollama.'],image:['Créer une image','Utilisez un workflow ComfyUI compatible.'],video:['Créer une vidéo','Utilisez un workflow vidéo ComfyUI compatible.'],musique:['Créer de la musique','Utilisez un workflow audio ou musical ComfyUI compatible.']};const textMode=()=>kind==='texte'||kind==='code';const busy=()=>job.state==='running'||job.state==='stopping';async function api(path,body){const r=await fetch('/api/'+path,{method:body===undefined?'GET':'POST',headers:{'X-Studio-Token':token,...(body===undefined?{}:{'Content-Type':'application/json'})},...(body===undefined?{}:{body:JSON.stringify(body)})});const v=await r.json();if(!r.ok)throw Error(v.error||'HTTP '+r.status);return v}function msg(v){$('feedback').textContent=v}function controls(){const available=textMode()?(!status.ollama_error&&!!$('models').value):(!status.comfy_error&&!!status.comfy_stats);$('generate').disabled=busy()||!available;$('stop').disabled=!busy()||job.state==='stopping'||!(Object.keys(config).includes(job.kind)||job.kind==='pull');$('benchmark').disabled=busy()||!!status.ollama_error||!(status.models||[]).length;$('backup').disabled=busy();$('restore').disabled=busy()||!$('backup-list').value;$('clear').disabled=busy();$('prompt').disabled=busy();$('models').disabled=busy();$('workflow').disabled=busy();$('trusted').disabled=busy();$('pull').disabled=busy()||!!status.ollama_error}
function selectKind(k){kind=k;document.body.dataset.kind=k;for(const b of document.querySelectorAll('[data-kind]'))if(b.tagName==='BUTTON'){b.classList.toggle('selected',b.dataset.kind===k);b.setAttribute('aria-pressed',String(b.dataset.kind===k))}$('heading').textContent=config[k][0];$('subtitle').textContent=config[k][1];$('text-settings').hidden=!textMode();$('media-settings').hidden=textMode();$('models').required=textMode();$('attachments').hidden=!textMode();$('attachment').value='';$('prompt').value='';$('transcript').textContent='';if(token)outputs();$('workflow').value='';$('trusted').checked=false;msg('');controls()}
const appHeader = document.querySelector('header');
new ResizeObserver(() => {
  document.body.style.setProperty('--header-height', appHeader.offsetHeight + 'px');
}).observe(appHeader);
for (const button of $('tabs').querySelectorAll('button')) {
  button.onclick = () => {
    if (preparing || button.dataset.kind === kind) return;
    selectKind(button.dataset.kind);
    const headingTop = $('heading').getBoundingClientRect().top;
    const chromeBottom = Math.max(
      appHeader.getBoundingClientRect().bottom,
      matchMedia('(max-width: 760px)').matches ? 0 : $('tabs').getBoundingClientRect().bottom,
    );
    if (headingTop < chromeBottom) window.scrollTo(0, 0);
  };
}
$('models').onchange=controls;$('clear').onclick=()=>{delete threads[kind];$('transcript').textContent='';$('attachment').value='';$('prompt').value='';$('files').replaceChildren();$('result').textContent='Votre création apparaîtra ici.';msg('Nouvelle discussion : le contexte précédent est effacé.')};async function refresh(){try{status=await api('status');const model=$('models'),old=model.value;model.replaceChildren();const models=status.models||[];if(!models.length)model.append(new Option('Aucun modèle local détecté',''));for(const m of models)model.append(new Option((m.name||m.model)+' · '+(m.size/1e9).toFixed(1)+' Go',m.name||m.model));if([...model.options].some(o=>o.value===old))model.value=old;$('engines').textContent='Ollama '+(status.ollama_error?'indisponible':'joignable')+' · ComfyUI '+(status.comfy_error?'indisponible':'joignable');$('root').textContent='Dossier : '+status.root;controls()}catch(e){$('engines').textContent='Serveur inaccessible';msg(e.message)}}
async function observer(){try{const v=await api('observer');const checks=v.controles||[];$('hardware').textContent=checks.some(x=>x.etat==='OK')?'Matériel · '+checks.filter(x=>x.etat==='OK').length+' mesure(s)':'Matériel non vérifié';$('details').textContent=checks.map(x=>x.controle+' : '+x.detail+' ['+x.etat+']').join('\n')}catch(e){$('hardware').textContent='Mesures indisponibles';$('details').textContent=e.message}}
async function download(j,name){const a=document.createElement('a');a.href='/api/file?'+new URLSearchParams({job:j,name});a.download=name;document.body.append(a);a.click();a.remove()}function entry(parent,j,name){const line=document.createElement('div');line.className='entry';const s=document.createElement('span');s.textContent=name;const b=document.createElement('button');b.textContent='Enregistrer';b.onclick=async()=>{b.disabled=true;try{await download(j,name)}catch(e){msg(e.message)}finally{b.disabled=false}};line.append(s,b);parent.append(line)}let listVersion=0, preparing=false;
async function outputs(){
 const version=++listVersion, category=kind;
 try {
  const [rows, files]=await Promise.all([api('history'),api('outputs')]);
  if(version!==listVersion)return;
  const target=$('outputs');target.replaceChildren();
  $('history-title').textContent='Historique · '+document.querySelector('#tabs [data-kind="'+category+'"]').textContent.slice(1);
  const groups=new Map();
  for(const row of rows.filter(r=>r.kind===category).reverse()){
   if(!groups.has(row.thread))groups.set(row.thread,[]);
   groups.get(row.thread).push(row);
  }
  if(!groups.size)target.textContent='Aucune création dans cet atelier.';
  for(const [thread,turns] of groups){
   const box=document.createElement('details'),title=document.createElement('summary');
   title.textContent=new Date(turns[0].date).toLocaleString('fr-FR')+' · '+turns.at(-1).prompt.slice(0,75);
   box.append(title);
   const resume=document.createElement('button');resume.textContent='Ouvrir et reprendre';
   resume.onclick=async()=>{if(busy())return;try{
    let transcript=turns.slice().reverse().map(r=>r.prompt).join('\n\n');
    if(textMode()){
     const lines=[];
     for(const row of turns.slice().reverse()){
      const result=await api('file?'+new URLSearchParams({job:row.id,name:'conversation.json'}));
      const messages=result.messages.filter(m=>m.role!=='system');
      for(const m of (lines.length?messages.slice(-2):messages))lines.push((m.role==='user'?'Vous':'Studio')+' · '+m.content);
     }
     transcript=lines.join('\n\n');
    }
    if(version!==listVersion)return;
    threads[category]=thread;$('transcript').textContent=transcript;$('prompt').value='';msg('Discussion ouverte.');$('prompt').focus();
   }catch(e){msg(e.message)}};
   const remove=document.createElement('button');remove.textContent='Supprimer définitivement';
   remove.onclick=async()=>{if(busy()||!confirm('Supprimer cette discussion et toutes ses créations du Studio ?'))return;
    try{await api('history/delete',{thread});if(threads[category]===thread){delete threads[category];$('transcript').textContent=''}render(await api('job'));await outputs()}catch(e){msg(e.message)}};
   box.append(resume,remove);
   for(const row of turns)for(const f of files.filter(f=>f.job===row.id))entry(box,row.id,f.name);
   target.append(box);
  }
 }catch(e){if(version===listVersion)$('outputs').textContent=e.message}
}
function render(j){const changed=j.id!==job.id||j.state!==job.state||JSON.stringify(j.files)!==JSON.stringify(job.files);job=j;const labels={running:'En cours',stopping:'Arrêt demandé',done:'Terminé',error:'Erreur',stopped:'Attente arrêtée'};$('state').textContent=labels[j.state]||'Prêt';if(changed||busy())$('result').textContent=[j.text,j.error].filter(Boolean).join('\n\n')||'Votre création apparaîtra ici.';$('rate').textContent=j.progress||(j.rate?j.rate.toFixed(1)+' tokens/s mesurés':'');if(changed){$('files').replaceChildren();for(const f of j.files||[])entry($('files'),j.id,f)}if(['backup','benchmark','restore'].includes(j.kind))$('maintenance').textContent=[labels[j.state],j.text,j.error].filter(Boolean).join('\n');if(changed&&j.kind==='backup'&&!busy())backupList();if(changed&&j.kind==='pull'&&!busy()){refresh();if(j.state==='error'||j.state==='stopped')networkNotice(j.error||'Téléchargement interrompu.');else networkNotice(j.text)}if(pending&&j.id===pending.id&&!busy()){if(j.state==='done'){threads[j.kind]=pending.thread||j.id;if(kind===j.kind){$('attachment').value='';$('transcript').textContent+=[pending.prompt,j.text].join('\n')+'\n\n'}}pending=null;outputs()}controls()}
async function poll(){try{render(await api('job'))}catch(e){msg(e.message)}finally{timer=setTimeout(poll,busy()?800:3500)}}async function start(path,body){try{msg('');const j=await api(path,body);render(j);return j}catch(e){msg(e.message);if(path!=='generate')$('maintenance').textContent=e.message}}
$('generate-form').onsubmit=async e=>{e.preventDefault();if(busy()||preparing)return;preparing=true;try{const p=$('prompt').value,q={kind,prompt:p,model:$('models').value,context:Number($('context').value),tokens:Number($('tokens').value),trusted:$('trusted').checked,thread:threads[kind]||''};if(textMode()){try{await attachFiles(q)}catch(err){msg(err.message);return}}if(!textMode()){const file=$('workflow').files[0];if(!file||file.size>3*1024*1024){msg('Sélectionnez un workflow API JSON de moins de 3 Mio.');return}try{q.workflow=JSON.parse(await file.text())}catch{msg('JSON invalide.');return}}const j=await start('generate',q);if(j)pending={id:j.id,prompt:p,thread:q.thread}}finally{preparing=false}};$('stop').onclick=async()=>{try{await api('stop',{});msg('Arrêt demandé. Le moteur externe peut encore travailler.')}catch(e){msg(e.message)}};$('refresh').onclick=refresh;$('outputs-refresh').onclick=outputs;async function backupList(){try{const rows=await api('backups');const el=$('backup-list'),old=el.value;el.replaceChildren(new Option('Sélectionner une archive',''));for(const name of rows)el.append(new Option(name,name));if(rows.includes(old))el.value=old;controls()}catch(e){$('maintenance').textContent=e.message}}$('backup-list').onchange=controls;$('restore').onclick=()=>{const name=$('backup-list').value;if(name&&confirm('Restaurer dans un nouveau dossier sans écraser les données ?'))start('restore',{name})};$('settings').onclick=()=>{$('dialog').showModal();backupList()};$('close').onclick=()=>$('dialog').close();$('diag-refresh').onclick=observer;$('benchmark').onclick=()=>{if(confirm('Lancer un benchmark réel avec un modèle local installé ?'))start('benchmark',{})};$('backup').onclick=()=>{if(confirm('Sauvegarder data/ dans une archive vérifiée ?'))start('backup',{})};$('search-form').onsubmit=async e=>{e.preventDefault();if(!allowNetwork())return;const target=$('library-results');target.textContent='Recherche…';try{const rows=await api('library?'+new URLSearchParams({kind,q:$('search').value}));networkNotice('Hugging Face joignable.');target.replaceChildren();if(!rows.length)target.textContent='Aucun résultat.';for(const m of rows){if(!/^[\w.-]+\/[\w.-]+$/.test(m.id))continue;const line=document.createElement('div');line.className='entry';const a=document.createElement('a');a.href='https://huggingface.co/'+m.id;a.target='_blank';a.rel='noopener noreferrer';a.textContent=m.id;a.onclick=async e=>{e.preventDefault();if(!allowNetwork())return;const tab=window.open('about:blank','_blank');if(tab)tab.opener=null;try{await api('connectivity');networkNotice('Hugging Face joignable.');if(tab)tab.location.href=a.href;else window.location.assign(a.href)}catch(err){if(tab)tab.close();networkNotice(err.message)}};const s=document.createElement('span');s.className='hint';s.textContent=Number(m.downloads||0).toLocaleString('fr-FR')+' téléchargements';line.append(a,s);target.append(line)}}catch(e){target.textContent='Recherche indisponible : '+e.message;networkNotice(e.message)}};
const offlineText='Vous êtes hors ligne. Connectez-vous à Internet pour télécharger des modèles. Les modèles déjà installés restent utilisables.';function networkNotice(t){$('network-message').textContent=t;if(t==='Hugging Face joignable.')$('network').textContent='Hugging Face : accessible';else if(navigator.onLine!==false&&/Internet indisponible|service inaccessible|Hugging Face indisponible/.test(t))$('network').textContent='Internet : accès non confirmé'}function updateNetwork(){const offline=navigator.onLine===false;$('network').textContent=offline?'Hors ligne':'Internet : non vérifié';$('network').dataset.offline=String(offline);if(offline)networkNotice(offlineText);else networkNotice('Connexion réseau détectée. L’accès Internet sera vérifié à la demande.')}function allowNetwork(){if(navigator.onLine===false){updateNetwork();return false}return true}window.addEventListener('online',updateNetwork);window.addEventListener('offline',updateNetwork);$('pull-form').onsubmit=async e=>{e.preventDefault();if(!allowNetwork()||busy())return;const model=$('pull-model').value.trim();if(!model)return;if(!confirm('Télécharger '+model+' via Ollama ? Le modèle peut occuper plusieurs Go.'))return;networkNotice('Vérification du registre Ollama…');const j=await start('pull',{model});if(!j)networkNotice($('feedback').textContent)};updateNetwork();(async()=>{controls();try{token=(await api('session')).token;selectKind('texte');await Promise.all([refresh(),outputs()]);observer();poll();setInterval(()=>{if(!document.hidden){refresh();observer()}},20000)}catch(e){msg('Démarrage impossible : '+e.message)}})();

async function attachFiles(q){
 let imageBytes=0;
 for(const file of Array.from($('attachment').files)){
  if(['image/jpeg','image/png','image/webp'].includes(file.type)){
   imageBytes+=file.size;if(imageBytes>2*1024*1024)throw Error('Les images dépassent 2 Mio au total.');
   const encoded=await new Promise((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(reader.result.split(',')[1]);reader.onerror=()=>reject(Error('Image illisible'));reader.readAsDataURL(file)});
   (q.images??=[]).push(encoded);
  }else{
   if(file.size>32*1024*1024)throw Error('Fichier supérieur à 32 Mio.');
   const r=await fetch('/api/attachment?'+new URLSearchParams({name:file.name}),{method:'POST',headers:{'X-Studio-Token':token},body:file});
   const v=await r.json();if(!r.ok)throw Error(v.error);
   q.prompt+='\n\n'+file.name+'\n'+v.text;
  }
 }
}
$('offline-test').onclick=async()=>{
 if(busy()||preparing)return;
 const q={kind,model:$('models').value,prompt:'Réponds simplement OK.',tokens:64,context:2048};
 if(!textMode()){
  if(!$('trusted').checked||!$('workflow').files.length){msg('Sélectionnez et approuvez le workflow à tester.');return}
  try{q.workflow=JSON.parse(await $('workflow').files[0].text());q.trusted=true}catch{msg('Workflow JSON invalide.');return}
 }
 const j=await start('generate',q);if(j)pending={id:j.id,prompt:q.prompt};
};
