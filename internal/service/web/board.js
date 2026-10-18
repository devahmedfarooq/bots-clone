const fragment = new URLSearchParams(location.hash.slice(1));
if (fragment.has('token')) sessionStorage.setItem('local-bots-token', fragment.get('token'));
history.replaceState(null, '', location.pathname);
const $ = id => document.getElementById(id);
const escape = text => String(text ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const states = ['Inbox', 'Ready', 'In Progress', 'Review', 'Done', 'Blocked', 'Needs Your Approval'];
let board = {goals:[], cards:[]}, rendered = '', editing = '', refreshing = false;
function settingsFields() {
  const fields = [['priority','Priority (5 highest)',3,1,5],['token_limit','Total token budget',60000,1,10000000],['run_limit','Total runs (including planning/review)',30,1,1000],['run_tokens','Tokens per run',6000,1,131072],['seconds','Seconds per run',180,1,3600],['tool_calls','Tool calls per run',6,0,100]];
  return `<div class="fields">${fields.map(([name,label,value,min,max]) => `<label>${label}<input type="number" name="${name}" value="${value}" min="${min}" max="${max}" required></label>`).join('')}</div><div class="rules">${[['before_work','Approve all worker runs'],['before_done','Approve verified results'],['tools','Approve bots with tool access']].map(([name,label])=>`<label><input type="checkbox" name="${name}" ${name==='tools'?'checked':''}>${label}</label>`).join('')}</div>`;
}
$('create-settings').innerHTML = settingsFields(); $('edit-settings').innerHTML = settingsFields();
async function api(path, data) {
  const response = await fetch('/api/'+path, {headers:{Authorization:'Bearer '+(sessionStorage.getItem('local-bots-token')||''), ...(data?{'Content-Type':'application/json'}:{})}, ...(data?{method:'POST',body:JSON.stringify(data)}:{})});
  if (!response.ok) { const text=await response.text(); throw Error(response.status===401?'Open the dashboard link printed by bots serve to connect.':text); }
  return response.json();
}
function settings(form) {
  const data = new FormData(form), result={rules:{}};
  for(const name of ['priority','token_limit','run_limit','run_tokens','seconds','tool_calls']) result[name]=Number(data.get(name));
  for(const name of ['before_work','before_done','tools']) result.rules[name]=data.has(name);
  return result;
}
function cardMarkup(card) {
  const history = card.history.map(item => `<p>${escape(new Date(item.at).toLocaleString())} · ${escape(item.actor)}<br>${escape(item.text)}</p>`).join('');
  const button = (action, label) => `<button data-card="${card.id}" data-action="${action}">${label}</button>`;
  let actions = '';
  if (card.status === 'Needs Your Approval') actions = button('approve', 'Approve') + button('reject', 'Reject');
  if (card.status === 'Blocked') actions = button('retry', card.phase === 'verify' ? 'Retry review' : 'Retry task');
  return `<article class="card">
    <h3>${escape(card.title)}</h3>
    <p>${escape(card.owner)} · ${escape(card.phase)} · ${escape(card.id.slice(0,8))}</p>
    <details id="card-${card.id}">
      <summary>Requirements, output & history</summary>
      <p>Requirements</p><pre>${escape(card.requirements)}</pre>
      <p>Acceptance</p><pre>${escape(card.acceptance)}</pre>
      <p>Output</p><pre>${escape(card.output || 'Waiting for output.')}</pre>
      <p>Jobs: ${escape((card.jobs || []).join(', ') || card.job || 'Waiting')}</p>
      <div class="history">${history}</div>
    </details>${actions}
  </article>`;
}
function draw() {
  const selected=$('goal').value, goal=board.goals.find(g=>g.id===selected);
  const signature=JSON.stringify([selected,board]); if(signature===rendered)return; rendered=signature;
  const open=new Set([...document.querySelectorAll('#board details[open]')].map(d=>d.id));
  $('goal').innerHTML='<option value="">All goals</option>'+board.goals.map(g=>`<option value="${g.id}">${escape(g.requirements.slice(0,90))}</option>`).join('');$('goal').value=selected;
  $('board').innerHTML=states.map(status=>{
    const cards=board.cards.filter(c=>c.status===status && (!selected||c.goal===selected));
    return `<section class="column"><h2>${status} · ${cards.length}</h2>${cards.map(cardMarkup).join('') || '<p>No tasks</p>'}</section>`;
  }).join('');
  for(const id of open) if($(id)) $(id).open=true;
  $('goal-panel').hidden=!goal;
  $('usage').textContent=goal?`${goal.status} · Priority ${goal.priority} · Tokens used/reserved ${goal.tokens}/${goal.token_limit} · Runs ${goal.runs}/${goal.run_limit}`:'Select a goal to view its chat, budgets and approval rules.';
  if(goal) {
    if(editing!==goal.id) {
      const form=$('settings-form');for(const name of ['priority','token_limit','run_limit','run_tokens','seconds','tool_calls']) form.elements[name].value=goal[name];
      for(const name of ['before_work','before_done','tools']) form.elements[name].checked=goal.rules[name];editing=goal.id;
    }
    const chat=$('chat'),atBottom=chat.scrollHeight-chat.scrollTop-chat.clientHeight<40;
    chat.innerHTML=goal.chat.map(m=>`<p><strong>${escape(m.actor)}</strong> · ${escape(new Date(m.at).toLocaleString())}<br>${escape(m.text)}</p>`).join('');
    if(atBottom)chat.scrollTop=chat.scrollHeight;
  }
}
async function refresh() {
  if(refreshing)return;refreshing=true;
  try {const result=await api('board');board=result.board;draw();$('connection').textContent=result.error?'Storage error: '+result.error:'Connected · Updates every 2 seconds';}
  catch(error){$('connection').textContent='Disconnected';$('notice').textContent=error.message;}finally{refreshing=false;}
}
async function mutation(button,run) {button.disabled=true;try{$('notice').textContent='';await run();await refresh();}catch(error){$('notice').textContent=error.message;}finally{button.disabled=false;}}
$('goal-form').addEventListener('submit',event=>{event.preventDefault();const form=event.currentTarget;mutation(event.submitter,async()=>{const data=new FormData(form);const goal=await api('goals',{...settings(form),requirements:data.get('requirements'),manager:data.get('manager'),workers:data.getAll('workers')});board.goals.push(goal);rendered='';draw();$('goal').value=goal.id;draw();$('new-goal').open=false;});});
$('goal').addEventListener('change',draw);
$('settings-form').addEventListener('submit',event=>{event.preventDefault();mutation(event.submitter,async()=>{await api(`goals/${$('goal').value}/settings`,settings(event.currentTarget));editing='';});});
$('chat-form').addEventListener('submit',event=>{event.preventDefault();const form=event.currentTarget;mutation(event.submitter,async()=>{await api(`goals/${$('goal').value}/chat`,{text:new FormData(form).get('text')});form.reset();});});
$('board').addEventListener('click',event=>{const button=event.target.closest('button[data-card]');if(!button)return;const note=prompt(button.dataset.action==='retry'?'Retry can repeat interrupted tool actions. Inspect the card history and job trace first. Add an instruction or reason:':'Decision note (optional):');if(note===null)return;mutation(button,()=>api(`cards/${button.dataset.card}/${button.dataset.action}`,{note}));});
(async()=>{try{const bots=await api('bots');const options=bots.filter(b=>b.compatibility==='compatible (prompt-only)').map(b=>`<option value="${escape(b.bot.id)}">${escape(b.bot.id)} — ${escape(b.bot.description)}</option>`).join('');$('manager').innerHTML=options;$('workers').innerHTML=options;if(bots.some(b=>b.bot.id==='manager'))$('manager').value='manager';if(!options)$('notice').textContent='Install manager and worker bot packages from the Local Bots dashboard first.';await refresh();setInterval(refresh,2000);}catch(error){$('notice').textContent=error.message;}})();
