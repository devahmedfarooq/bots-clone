const fragment = new URLSearchParams(location.hash.slice(1));
if (fragment.has('token')) {
  sessionStorage.setItem('local-bots-token', fragment.get('token'));
  try { localStorage.setItem('local-bots-token', fragment.get('token')); } catch (_) {}
}
history.replaceState(null, '', location.pathname);
const getToken = () => sessionStorage.getItem('local-bots-token') || localStorage.getItem('local-bots-token') || '';
const $ = id => document.getElementById(id);
const escape = text => String(text ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const states = ['Inbox', 'Ready', 'In Progress', 'Review', 'Done', 'Blocked', 'Needs Your Approval'];
let board = {goals:[], cards:[]}, rendered = '', editing = '', refreshing = false;
let activeCardId = null, showingRaw = false, showingPreview = false, lastReaderSig = '';

const assistantHints = {
  'landing-page': {
    hint: 'Describe your landing page or web component:',
    placeholder: 'e.g. Build a modern landing page for a coffee roastery with hero banner, product cards, testimonials, and clean dark aesthetic.'
  },
  'code-writer': {
    hint: 'Describe the script, program, or feature to code:',
    placeholder: 'e.g. Write a Python script to fetch stock prices from an API, calculate moving averages, and save a CSV report.'
  },
  'math-solver': {
    hint: 'Enter your math, algebra, or calculus problem:',
    placeholder: 'e.g. Solve the system of equations 2x + 3y = 12 and 5x - y = 13 step-by-step and verify the solution.'
  },
  'qa-assistant': {
    hint: 'Describe the code or feature to review and test:',
    placeholder: 'e.g. Review this user registration workflow for security edge cases, race conditions, and write a full test suite.'
  },
  'writer': {
    hint: 'Describe the article, guide, or copy to write:',
    placeholder: 'e.g. Write a high-converting announcement post for a new developer tool, highlighting local privacy and speed.'
  }
};

function settingsFields() {
  const fields = [['priority','Priority (5 highest)',3,1,5],['token_limit','Total token budget',60000,1,10000000],['run_limit','Total runs (including planning/review)',30,1,1000],['run_tokens','Tokens per run',6000,1,131072],['seconds','Seconds per run',180,1,3600],['tool_calls','Tool calls per run',6,0,100]];
  return `<div class="fields">${fields.map(([name,label,value,min,max]) => `<label>${label}<input type="number" name="${name}" value="${value}" min="${min}" max="${max}" required></label>`).join('')}</div><div class="rules">${[['before_work','Manual approval for all worker runs'],['before_done','Manual sign-off on final results'],['tools','Manual approval for file/tool operations']].map(([name,label])=>`<label><input type="checkbox" name="${name}"> ${label}</label>`).join('')}</div>`;
}
$('create-settings').innerHTML = settingsFields(); $('edit-settings').innerHTML = settingsFields();

async function api(path, data) {
  const token = getToken();
  const response = await fetch('/api/'+path, {
    headers: {
      Authorization: 'Bearer ' + token,
      ...(data ? {'Content-Type': 'application/json'} : {})
    },
    ...(data ? {method: 'POST', body: JSON.stringify(data)} : {})
  });
  if (!response.ok) {
    const text = await response.text();
    throw Error(response.status === 401 ? 'Authentication required. Reopen the dashboard link printed by bots serve to connect.' : text);
  }
  return response.json();
}

function settings(form) {
  const data = new FormData(form), result={rules:{}};
  for(const name of ['priority','token_limit','run_limit','run_tokens','seconds','tool_calls']) result[name]=Number(data.get(name));
  for(const name of ['before_work','before_done','tools']) result.rules[name]=data.has(name);
  return result;
}

function statusBadge(card) {
  switch (card.status) {
    case 'Done': return '<span class="status-badge badge-delivered">Verified Deliverable</span>';
    case 'Review': return '<span class="status-badge badge-review">Awaiting Review</span>';
    case 'Needs Your Approval': return '<span class="status-badge badge-review">Needs Approval</span>';
    case 'In Progress': return '<span class="status-badge badge-running">Running</span>';
    case 'Blocked': return '<span class="status-badge badge-blocked">Blocked</span>';
    default: return '<span class="status-badge badge-waiting">Waiting</span>';
  }
}

function extractResult(card) {
  const raw = (card.output || '').trim();
  if (!raw) return { text: '', isJson: false, raw: '' };
  try {
    let clean = raw;
    if (clean.startsWith('```json')) clean = clean.slice(7);
    if (clean.startsWith('```')) clean = clean.slice(3);
    if (clean.endsWith('```')) clean = clean.slice(0, -3);
    clean = clean.trim();
    const parsed = JSON.parse(clean);
    if (parsed && typeof parsed === 'object') {
      if (typeof parsed.output === 'string') return { text: parsed.output, isJson: true, raw, parsed };
      return { text: JSON.stringify(parsed, null, 2), isJson: true, raw, parsed };
    }
  } catch (_) {}
  return { text: raw, isJson: false, raw };
}

function formatContent(text) {
  if (!text) return '';
  const parts = [];
  const codeBlockRegex = /```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g;
  let lastIndex = 0, match;
  while ((match = codeBlockRegex.exec(text)) !== null) {
    if (match.index > lastIndex) parts.push({ type: 'text', content: text.slice(lastIndex, match.index) });
    parts.push({ type: 'code', lang: match[1], content: match[2] });
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < text.length) parts.push({ type: 'text', content: text.slice(lastIndex) });

  let html = '';
  for (const part of parts) {
    if (part.type === 'code') {
      html += `<pre><code>${escape(part.content.trim())}</code></pre>`;
      continue;
    }
    const paragraphs = part.content.split(/\n\s*\n/);
    for (let p of paragraphs) {
      p = p.trim();
      if (!p) continue;
      if (/^###\s+/.test(p)) html += `<h3>${escape(p.replace(/^###\s+/, ''))}</h3>`;
      else if (/^##\s+/.test(p)) html += `<h2>${escape(p.replace(/^##\s+/, ''))}</h2>`;
      else if (/^#\s+/.test(p)) html += `<h1>${escape(p.replace(/^#\s+/, ''))}</h1>`;
      else if (/^>\s+/.test(p)) {
        const quoteLines = p.split('\n').map(l => l.replace(/^>\s*/, ''));
        html += `<blockquote>${quoteLines.map(escape).join('<br>')}</blockquote>`;
      } else if (/^(\*|-)\s+/.test(p)) {
        const items = p.split('\n').filter(l => /^(\*|-)\s+/.test(l));
        html += `<ul>${items.map(l => `<li>${escape(l.replace(/^(\*|-)\s+/, ''))}</li>`).join('')}</ul>`;
      } else if (/^\d+\.\s+/.test(p)) {
        const items = p.split('\n').filter(l => /^\d+\.\s+/.test(l));
        html += `<ol>${items.map(l => `<li>${escape(l.replace(/^\d+\.\s+/, ''))}</li>`).join('')}</ol>`;
      } else if (p.includes('|') && p.split('\n').length >= 2 && p.split('\n')[1].includes('---')) {
        const rows = p.split('\n').map(r => r.trim()).filter(Boolean);
        const parseRow = r => r.split('|').slice(1, -1).map(c => c.trim());
        const header = parseRow(rows[0]);
        const bodyRows = rows.slice(2).map(parseRow);
        html += `<table><thead><tr>${header.map(c => `<th>${escape(c)}</th>`).join('')}</tr></thead><tbody>${bodyRows.map(r => `<tr>${r.map(c => `<td>${escape(c)}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
      } else {
        const lines = p.split('\n').map(escape);
        html += `<p>${lines.join('<br>')}</p>`;
      }
    }
  }
  return html;
}

function updateReader() {
  const card = board.cards.find(c => c.id === activeCardId);
  if (!card) { $('reader-title').textContent = 'Task Not Found'; return; }
  $('reader-title').textContent = card.title || 'Deliverable Result';
  const lastAt = card.history.length ? new Date(card.history[card.history.length-1].at).toLocaleTimeString() : '';
  $('reader-meta').innerHTML = `<span><strong>${escape(card.owner)}</strong></span>${statusBadge(card)}<span>Updated ${escape(lastAt)}</span><span>Job ${escape(card.job || (card.jobs && card.jobs.slice(-1)[0]) || 'none')}</span>`;
  const res = extractResult(card);

  // Check if content has HTML
  const hasHtml = /<!DOCTYPE html|<html[\s>]|<body[\s>]|<div[\s>]|<head[\s>]/i.test(res.text) || (card.owner === 'landing-page' && res.text.includes('<') && res.text.includes('>'));
  $('reader-preview').hidden = !hasHtml;

  if (showingPreview && hasHtml) {
    $('reader-iframe').hidden = false;
    $('reader-raw').hidden = true;
    $('reader-formatted').hidden = true;
    $('reader-preview').textContent = '📝 Show Code';
    let htmlCode = res.text;
    const match = /```(?:html)?\n([\s\S]*?)```/i.exec(res.text);
    if (match) htmlCode = match[1];
    $('reader-iframe').srcdoc = htmlCode;
  } else if (showingRaw) {
    $('reader-iframe').hidden = true;
    $('reader-raw').hidden = false;
    $('reader-formatted').hidden = true;
    $('reader-preview').textContent = '🌐 Live Preview';
    $('reader-raw').textContent = res.raw || 'No raw output recorded.';
  } else {
    $('reader-iframe').hidden = true;
    $('reader-raw').hidden = true;
    $('reader-formatted').hidden = false;
    $('reader-preview').textContent = '🌐 Live Preview';
    if (!res.text) {
      let emptyMsg = 'Waiting for worker output.';
      if (card.status === 'In Progress') emptyMsg = 'Assistant is currently working on this task…';
      else if (card.status === 'Blocked') emptyMsg = 'Task is blocked. No completed output available.';
      else if (card.status === 'Needs Your Approval') emptyMsg = 'Task awaiting approval before dispatch.';
      $('reader-formatted').innerHTML = `<div class="empty-result">${escape(emptyMsg)}</div>`;
    } else {
      $('reader-formatted').innerHTML = formatContent(res.text);
    }
  }
}

function openReader(cardId) {
  activeCardId = cardId;
  showingRaw = false;
  showingPreview = false;
  $('reader-toggle-raw').textContent = 'Diagnostics';
  $('reader-preview').textContent = '🌐 Live Preview';
  $('reader-drawer').hidden = false;
  lastReaderSig = '';
  updateReader();
}

function closeReader() {
  activeCardId = null;
  lastReaderSig = '';
  $('reader-drawer').hidden = true;
}

$('reader-close').addEventListener('click', closeReader);
$('reader-backdrop').addEventListener('click', closeReader);
window.addEventListener('keydown', e => { if (e.key === 'Escape' && !$('reader-drawer').hidden) closeReader(); });

$('reader-toggle-raw').addEventListener('click', () => {
  showingRaw = !showingRaw;
  showingPreview = false;
  $('reader-toggle-raw').textContent = showingRaw ? 'Formatted' : 'Diagnostics';
  lastReaderSig = '';
  updateReader();
});

$('reader-preview').addEventListener('click', () => {
  showingPreview = !showingPreview;
  showingRaw = false;
  $('reader-toggle-raw').textContent = 'Diagnostics';
  lastReaderSig = '';
  updateReader();
});

$('reader-copy').addEventListener('click', async () => {
  const card = board.cards.find(c => c.id === activeCardId);
  if (!card) return;
  const res = extractResult(card);
  let content = showingRaw ? (res.raw || res.text) : (res.text || res.raw);
  const match = /```(?:html|python|go|javascript|ts)?\n([\s\S]*?)```/i.exec(content);
  if (match) content = match[1];
  const btn = $('reader-copy');
  try {
    await navigator.clipboard.writeText(content);
    btn.textContent = 'Copied!';
    setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
  } catch (_) {
    const ta = document.createElement('textarea');
    ta.value = content;
    document.body.appendChild(ta);
    ta.select();
    document.execCommand('copy');
    document.body.removeChild(ta);
    btn.textContent = 'Copied!';
    setTimeout(() => { btn.textContent = 'Copy'; }, 1500);
  }
});

$('reader-download').addEventListener('click', () => {
  const card = board.cards.find(c => c.id === activeCardId);
  if (!card) return;
  const res = extractResult(card);
  let content = showingRaw ? (res.raw || res.text) : (res.text || res.raw);
  let ext = 'txt';
  if (card.owner === 'landing-page' || /<!DOCTYPE html|<html[\s>]/i.test(content)) ext = 'html';
  else if (card.owner === 'code-writer') {
    if (/import (?:sys|os|json|requests)|def |print\(/i.test(content)) ext = 'py';
    else if (/package |func main\(/i.test(content)) ext = 'go';
    else if (/function |const |let |console\.log/i.test(content)) ext = 'js';
  } else if (card.owner === 'writer') ext = 'md';

  const match = new RegExp('```(?:' + ext + ')?\n([\\s\\S]*?)```', 'i').exec(content);
  if (match) content = match[1];

  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  const titleSlug = (card.title || 'deliverable').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'deliverable';
  a.href = url;
  a.download = `${titleSlug}.${ext}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
});

function cardMarkup(card) {
  const history = card.history.map(item => `<p>${escape(new Date(item.at).toLocaleString())} · ${escape(item.actor)}<br>${escape(item.text)}</p>`).join('');
  const button = (action, label, cls) => `<button type="button" class="${cls}" data-card="${card.id}" data-action="${action}">${label}</button>`;
  let actions = '';
  if (card.status === 'Needs Your Approval') {
    actions = `<div class="card-action-row">${button('approve', '✓ Approve', 'btn-approve')}${button('reject', '✕ Reject', 'btn-reject')}</div>`;
  } else if (card.status === 'Blocked') {
    actions = `<div class="card-action-row">${button('retry', '↺ ' + (card.phase === 'verify' ? 'Retry Review' : 'Retry Task'), 'btn-retry')}</div>`;
  }
  return `<article class="card">
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:.4rem">
      <h3 style="margin:0">${escape(card.title)}</h3>
      ${statusBadge(card)}
    </div>
    <p>Assistant: <strong>${escape(card.owner)}</strong> · ${escape(card.phase)}</p>
    <button type="button" class="btn-view-result" data-view-result="${card.id}">👁️ View Deliverable</button>
    <details id="card-${card.id}">
      <summary>Requirements &amp; History</summary>
      <p>Requirements</p><pre>${escape(card.requirements)}</pre>
      <p>Acceptance</p><pre>${escape(card.acceptance)}</pre>
      <p>Jobs: ${escape((card.jobs || []).join(', ') || card.job || 'Waiting')}</p>
      <div class="history">${history}</div>
    </details>${actions}
  </article>`;
}

function draw() {
  const selected=$('goal').value, goal=board.goals.find(g=>g.id===selected);
  const signature=JSON.stringify([selected,board]);
  if(signature===rendered) {
    if (!$('reader-drawer').hidden && activeCardId) {
      const card = board.cards.find(c => c.id === activeCardId);
      if (card) {
        const sig = card.id + '|' + (card.output || '') + '|' + card.status + '|' + card.phase + '|' + showingRaw + '|' + showingPreview;
        if (sig !== lastReaderSig) { lastReaderSig = sig; updateReader(); }
      }
    }
    return;
  }
  rendered=signature;
  const open=new Set([...document.querySelectorAll('#board details[open]')].map(d=>d.id));
  $('goal').innerHTML='<option value="">All projects</option>'+board.goals.map(g=>`<option value="${g.id}">${escape(g.requirements.slice(0,90))}</option>`).join('');$('goal').value=selected;
  $('board').innerHTML=states.map(status=>{
    const cards=board.cards.filter(c=>c.status===status && (!selected||c.goal===selected));
    return `<section class="column"><h2>${status} · ${cards.length}</h2>${cards.map(cardMarkup).join('') || '<p style="color:#5c6f84;font-size:0.85rem">No tasks</p>'}</section>`;
  }).join('');
  for(const id of open) if($(id)) $(id).open=true;
  $('goal-panel').hidden=!goal;
  $('usage').textContent=goal?`${goal.status} · Priority ${goal.priority} · Tokens ${goal.tokens}/${goal.token_limit} · Runs ${goal.runs}/${goal.run_limit}`:'Select a project to inspect history and chat.';
  if(goal) {
    if(editing!==goal.id) {
      const form=$('settings-form');for(const name of ['priority','token_limit','run_limit','run_tokens','seconds','tool_calls']) form.elements[name].value=goal[name];
      for(const name of ['before_work','before_done','tools']) form.elements[name].checked=goal.rules[name];editing=goal.id;
    }
    const chat=$('chat'),atBottom=chat.scrollHeight-chat.scrollTop-chat.clientHeight<40;
    chat.innerHTML=goal.chat.map(m=>`<p><strong>${escape(m.actor)}</strong> · ${escape(new Date(m.at).toLocaleString())}<br>${escape(m.text)}</p>`).join('');
    if(atBottom)chat.scrollTop=chat.scrollHeight;
  }
  if (!$('reader-drawer').hidden && activeCardId) {
    const card = board.cards.find(c => c.id === activeCardId);
    if (card) {
      const sig = card.id + '|' + (card.output || '') + '|' + card.status + '|' + card.phase + '|' + showingRaw + '|' + showingPreview;
      if (sig !== lastReaderSig) { lastReaderSig = sig; updateReader(); }
    }
  }
}

async function refresh() {
  if(refreshing)return;refreshing=true;
  try {const result=await api('board');board=result.board;draw();$('connection').textContent=result.error?'Storage error: '+result.error:'Connected · Ready';}
  catch(error){$('connection').textContent='Disconnected';$('notice').textContent=error.message;}finally{refreshing=false;}
}

async function mutation(button,run) {
  if (button) button.disabled=true;
  try{
    $('notice').textContent='';
    await run();
    await refresh();
  }catch(error){
    $('notice').textContent='⚠️ ' + error.message;
    alert('Action error: ' + error.message);
  }finally{
    if (button) button.disabled=false;
  }
}

$('goal-form').addEventListener('submit',event=>{
  event.preventDefault();
  const form=event.currentTarget;
  mutation(event.submitter,async()=>{
    const data=new FormData(form);
    const goal=await api('goals',{...settings(form),requirements:data.get('requirements'),manager:data.get('manager'),workers:data.getAll('workers')});
    board.goals.push(goal);
    rendered='';
    draw();
    $('goal').value=goal.id;
    draw();
  });
});

$('goal').addEventListener('change',draw);
$('settings-form').addEventListener('submit',event=>{event.preventDefault();mutation(event.submitter,async()=>{await api(`goals/${$('goal').value}/settings`,settings(event.currentTarget));editing='';});});
$('chat-form').addEventListener('submit',event=>{
  event.preventDefault();
  const form=event.currentTarget;
  mutation(event.submitter,async()=>{
    await api(`goals/${$('goal').value}/chat`,{text:new FormData(form).get('text')});
    form.reset();
  });
});

$('board').addEventListener('click',event=>{
  const viewBtn = event.target.closest('button[data-view-result]');
  if (viewBtn) { openReader(viewBtn.dataset.viewResult); return; }
  const button=event.target.closest('button[data-card]');
  if(!button)return;
  const action = button.dataset.action;
  const cardId = button.dataset.card;
  const originalLabel = button.textContent;
  mutation(button, async () => {
    button.textContent = action === 'approve' ? 'Approving…' : (action === 'reject' ? 'Rejecting…' : 'Retrying…');
    try {
      await api(`cards/${cardId}/${action}`, { note: action + ' by user' });
    } catch (err) {
      button.textContent = originalLabel;
      throw err;
    }
  });
});

// Assistant pill click handler
document.addEventListener('click', event => {
  const pill = event.target.closest('.assistant-pill');
  if (!pill) return;
  document.querySelectorAll('.assistant-pill').forEach(p => p.classList.remove('active'));
  pill.classList.add('active');
  const botId = pill.dataset.bot;
  if ($('workers')) {
    Array.from($('workers').options).forEach(opt => {
      opt.selected = (opt.value === botId);
    });
  }
  if (assistantHints[botId]) {
    $('prompt-hint').textContent = assistantHints[botId].hint;
    $('prompt-input').placeholder = assistantHints[botId].placeholder;
  }
});

(async()=>{
  try {
    const bots=await api('bots');
    const options=bots.filter(b=>b.compatibility==='compatible (prompt-only)').map(b=>`<option value="${escape(b.bot.id)}">${escape(b.bot.id)}</option>`).join('');
    $('manager').innerHTML=options;
    $('workers').innerHTML=options;
    if(bots.some(b=>b.bot.id==='manager')) $('manager').value='manager';

    // Preselect landing-page if available, otherwise first worker
    if(bots.some(b=>b.bot.id==='landing-page')) {
      const landingOpt = Array.from($('workers').options).find(o => o.value === 'landing-page');
      if (landingOpt) landingOpt.selected = true;
    } else if ($('workers').options.length) {
      $('workers').options[0].selected = true;
    }

    await refresh();
    setInterval(refresh,2000);
  } catch(error) {
    $('notice').textContent=error.message;
  }
})();
