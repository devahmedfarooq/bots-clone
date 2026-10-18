const $ = id => document.getElementById(id);
const fragment = new URLSearchParams(location.hash.slice(1));
if (fragment.has('token')) sessionStorage.setItem('local-bots-token', fragment.get('token'));
history.replaceState(null, '', location.pathname);
const token = sessionStorage.getItem('local-bots-token') || '';
async function api(path, data) {
  const response = await fetch('/api/' + path, {
    method: data === undefined ? 'GET' : 'POST',
    headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' },
    body: data === undefined ? undefined : JSON.stringify(data)
  });
  if (!response.ok) {
    const text = await response.text();
    let message = text;
    try { message = JSON.parse(text).error || text; } catch {}
    throw Error(response.status === 401 ? 'Open the dashboard link printed by bots serve to connect.' : message);
  }
  return response.json();
}
function node(tag, text, className) {
  const element = document.createElement(tag);
  if (text !== undefined) element.textContent = text;
  if (className) element.className = className;
  return element;
}
function report(error) { $('notice').textContent = error.message; }
let bots = [];
function selected() {
  const entry = bots.find(entry => entry.bot.id === $('bot').value);
  $('details').textContent = entry ? `${entry.bot.description} · ${entry.effective.context_size} context · ${entry.effective.max_tokens} output tokens` : 'Install a bot to get started.';
  $('submit').disabled = !entry;
}
async function loadBots() {
  bots = await api('bots');
  const previous = $('bot').value;
  $('bot').replaceChildren(...bots.map(({bot}) => {const option=node('option',bot.id);option.value=bot.id;return option;}));
  if (bots.some(entry => entry.bot.id === previous)) $('bot').value = previous;
  $('bots').replaceChildren(...bots.map(({bot,compatibility}) => {
    const card=node('div',undefined,'bot');
    card.append(node('strong',bot.id + ' · ' + bot.version),node('span',compatibility,'tag'),node('p','Tools: '+(bot.tools || []).join(', '),'muted'),node('p','Folders: '+(bot.context_sources || []).join(', '),'muted'));
    return card;
  }));
  selected();
}
function showRun(run) {
  const details=node('details');details.append(node('summary','Output and trace'));
  details.append(node('pre',run.output || run.error || 'No output.'));
  const trace=node('button','Inspect tool trace','secondary');
  trace.onclick=async()=>{try {details.append(node('pre',JSON.stringify(await api('runs/'+run.id+'/trace'),null,2)));trace.disabled=true;}catch(error){report(error);}};
  details.append(node('p',`${run.id} · ${run.duration_ms} ms · ${run.usage.total_tokens || 0} tokens`,'muted'),trace);
  return details;
}
let refreshing = false;
async function refresh() {
  if (refreshing) return;
  refreshing = true;
  try {
    const [jobs,runs]=await Promise.all([api('jobs'),api('runs')]);
    $('jobs').replaceChildren(...(jobs.length ? jobs.map(job=>{
      const card=node('div',undefined,'job');
      card.append(node('strong',job.bot),node('span',job.status,'tag '+job.status),node('p',new Date(job.created).toLocaleString(),'muted'));
      if(job.error)card.append(node('p',job.error));
      if(job.run)card.append(showRun(job.run));
      if(['queued','running'].includes(job.status)) {
        const cancel=node('button','Cancel task','secondary');
        cancel.onclick=async()=>{try{await api('jobs/'+job.id+'/cancel',{});await refresh();}catch(error){report(error);}};card.append(cancel);
      }
      return card;
    }):[node('p','No tasks yet.','muted')]));
    $('runs').replaceChildren(...(runs.length ? runs.slice(0,20).map(run=>{
      const card=node('div',undefined,'job');card.append(node('strong',run.bot+' · '+run.version),node('span',run.status,'tag '+run.status),showRun(run));return card;
    }):[node('p','Your saved runs will appear here.','muted')]));
  } catch(error){report(error);} finally {refreshing=false;}
}
function form(id, action) {
  $(id).onsubmit=async event=>{
    event.preventDefault();const button=event.currentTarget.querySelector('button');button.disabled=true;$('notice').textContent='';
    try{await action();}catch(error){report(error);}finally{button.disabled=false;selected();}
  };
}
$('bot').onchange=selected;
$('refresh').onclick=refresh;
form('run-form',async()=>{await api('jobs',{bot:$('bot').value,input:$('input').value});$('notice').textContent='Task submitted.';await refresh();});
form('install-form',async()=>{await api('packages',{directory:$('directory').value});await loadBots();$('notice').textContent='Bot installed. Review its tools and folders above.';});
loadBots().then(refresh).catch(report);
setInterval(()=>{if(!document.hidden)refresh();},2000);
