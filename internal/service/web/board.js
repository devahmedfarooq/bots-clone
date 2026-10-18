const store = {
  get: key => { try { return localStorage.getItem(key); } catch (_) { return null; } },
  set: (key, value) => { try { localStorage.setItem(key, value); } catch (_) {} }
};
const $ = id => document.getElementById(id);
const escape = text => String(text ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

const states = ['Inbox', 'Ready', 'In Progress', 'Review', 'Done', 'Blocked', 'Needs Your Approval'];
const statusLabels = {
  'Done': ['Done', 'delivered'], 'Review': ['Checking', 'review'], 'Needs Your Approval': ['Needs your OK', 'review'],
  'In Progress': ['Working', 'running'], 'Blocked': ['Needs attention', 'blocked'], 'Ready': ['Queued', 'waiting'], 'Inbox': ['Queued', 'waiting']
};
const assistants = {
  'auto': {label: 'Manager decides', about: 'Splits the goal, assigns bots, reviews', hint: 'Describe your goal:', placeholder: 'Write a short poem about the sea. Or: build a landing page for a coffee shop and write its announcement post.'},
  'landing-page': {label: 'Landing page', about: 'Web pages (HTML/CSS)', hint: 'Describe the page you want:', placeholder: 'A landing page for a coffee roastery with a hero banner, product cards and testimonials.'},
  'code-writer': {label: 'Code', about: 'Scripts and programs', hint: 'Describe the program you want:', placeholder: 'A Python script that reads a CSV of prices and writes a moving-average report.'},
  'math-solver': {label: 'Math', about: 'Step-by-step solutions', hint: 'Enter your math problem:', placeholder: 'Solve 2x + 3y = 12 and 5x - y = 13 step by step.'},
  'qa-assistant': {label: 'QA', about: 'Reviews and tests', hint: 'Describe what to review or test:', placeholder: 'Review a user registration flow for security edge cases and list test cases.'},
  'writer': {label: 'Writer', about: 'Articles, copy, poems', hint: 'Describe what to write:', placeholder: 'A short poem about the sea, or a product announcement post.'}
};
const settingFields = [['priority','Priority (5 highest)',3,1,5],['token_limit','Total token budget',120000,1,10000000],['run_limit','Max runs',30,1,1000],['run_tokens','Tokens per run',24000,1,131072],['seconds','Seconds per run',180,1,3600],['tool_calls','Tool calls per run',6,0,100]];
const settingRules = [['before_work','Ask me before any assistant runs'],['before_done','Ask me to sign off on results'],['tools','Ask me before file/tool use']];

let board = {goals: [], cards: []}, goalId = store.get('studio-goal') || '', cardId = '', botId = '';
let workerIds = [], hasManager = false;
let rendered = '', editing = '', refreshing = false, botsLoaded = false, view = 'formatted', connectionError = false;

const settingsFields = () => `<div class="fields">${settingFields.map(([name,label,value,min,max]) => `<label>${label}<input type="number" name="${name}" value="${value}" min="${min}" max="${max}" required></label>`).join('')}</div><div class="rules">${settingRules.map(([name,label]) => `<label><input type="checkbox" name="${name}"> ${label}</label>`).join('')}</div>`;
$('create-settings').innerHTML = settingsFields();
$('edit-settings').innerHTML = settingsFields();

function notify(text, isConnection = false) { $('notice').textContent = text; connectionError = isConnection && !!text; }

async function api(path, data) {
  const response = await fetch('/api/' + path, {
    headers: data ? {'Content-Type': 'application/json'} : {},
    ...(data ? {method: 'POST', body: JSON.stringify(data)} : {})
  });
  if (!response.ok) {
    const text = (await response.text()).trim();
    throw Error(text || 'Request failed (' + response.status + ').');
  }
  return response.json();
}

function settings(form) {
  const data = new FormData(form), result = {rules: {}};
  for (const [name] of settingFields) result[name] = Number(data.get(name));
  for (const [name] of settingRules) result.rules[name] = data.has(name);
  return result;
}

function badge(status) {
  const [label, cls] = statusLabels[status] || ['Queued', 'waiting'];
  return `<span class="status-badge badge-${cls}">${label}</span>`;
}
const statusBadge = card => badge(card.status);

function extractResult(card) {
  const raw = (card.output || '').trim();
  if (!raw) return {text: '', raw: ''};
  try {
    const parsed = JSON.parse(raw.replace(/^```(?:json)?\s*/, '').replace(/```$/, '').trim());
    if (parsed && typeof parsed === 'object') return {text: typeof parsed.output === 'string' ? parsed.output : JSON.stringify(parsed, null, 2), raw};
  } catch (_) {}
  return {text: raw, raw};
}

function formatContent(text) {
  const parts = [], codeBlock = /```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g;
  let last = 0, match;
  while ((match = codeBlock.exec(text)) !== null) {
    if (match.index > last) parts.push({code: false, content: text.slice(last, match.index)});
    parts.push({code: true, content: match[2]});
    last = match.index + match[0].length;
  }
  if (last < text.length) parts.push({code: false, content: text.slice(last)});
  const list = (p, bullet, tag) => `<${tag}>${p.split('\n').filter(l => bullet.test(l)).map(l => `<li>${escape(l.replace(bullet, ''))}</li>`).join('')}</${tag}>`;
  return parts.map(part => {
    if (part.code) return `<pre><code>${escape(part.content.trim())}</code></pre>`;
    return part.content.split(/\n\s*\n/).map(p => p.trim()).filter(Boolean).map(p => {
      const lines = p.split('\n');
      if (/^###\s+/.test(p)) return `<h3>${escape(p.replace(/^###\s+/, ''))}</h3>`;
      if (/^##\s+/.test(p)) return `<h2>${escape(p.replace(/^##\s+/, ''))}</h2>`;
      if (/^#\s+/.test(p)) return `<h1>${escape(p.replace(/^#\s+/, ''))}</h1>`;
      if (/^>\s+/.test(p)) return `<blockquote>${lines.map(l => escape(l.replace(/^>\s*/, ''))).join('<br>')}</blockquote>`;
      if (/^[*-]\s+/.test(p)) return list(p, /^[*-]\s+/, 'ul');
      if (/^\d+\.\s+/.test(p)) return list(p, /^\d+\.\s+/, 'ol');
      if (lines.length >= 2 && p.includes('|') && lines[1].includes('---')) {
        const cells = r => r.split('|').slice(1, -1).map(c => c.trim());
        return `<table><thead><tr>${cells(lines[0]).map(c => `<th>${escape(c)}</th>`).join('')}</tr></thead><tbody>${lines.slice(2).map(r => `<tr>${cells(r).map(c => `<td>${escape(c)}</td>`).join('')}</tr>`).join('')}</tbody></table>`;
      }
      return `<p>${lines.map(escape).join('<br>')}</p>`;
    }).join('');
  }).join('');
}

// Exact text to copy/download, its file type and any renderable HTML.
function deliverable(card, raw) {
  const res = extractResult(card), text = raw ? (res.raw || res.text) : (res.text || res.raw);
  const whole = /^```([\w-]*)\n([\s\S]*?)\n?```$/.exec(text.trim());
  const inner = /```html\n([\s\S]*?)```/i.exec(text);
  const body = whole ? whole[2] : text;
  const html = inner ? inner[1] : (/<!DOCTYPE html|<html[\s>]|<body[\s>]/i.test(body) ? body : '');
  const byLang = {python: 'py', py: 'py', go: 'go', javascript: 'js', js: 'js', ts: 'ts', css: 'css', json: 'json', html: 'html'};
  const ext = html && whole ? 'html' : byLang[(whole?.[1] || '').toLowerCase()] || (html ? 'html' : card.owner === 'writer' ? 'md' : 'txt');
  return {body: ext === 'html' && html ? html : body, html, ext};
}

const goalTitle = goal => goal.requirements.trim().split('\n')[0].slice(0, 80);
const goalTasks = goal => board.cards.filter(c => c.goal === goal.id && c.kind === 'task');
function finalCard(goal, tasks) {
  if (tasks.length < 2 || !tasks.every(c => c.status === 'Done')) return null;
  const last = tasks[tasks.length - 1];
  return {id: 'final', goal: goal.id, title: 'Final result', owner: last.owner, kind: 'final', status: 'Done', phase: 'work', history: last.history,
    output: tasks.map(c => '## ' + c.title + '\n\n' + extractResult(c).text).join('\n\n')};
}
const goalCards = goal => {
  const tasks = goalTasks(goal), final = finalCard(goal, tasks);
  if (final) return [final, ...tasks];
  return tasks.length ? tasks : board.cards.filter(c => c.goal === goal.id);
};
const currentGoal = () => board.goals.find(g => g.id === goalId);
function currentCard(goal) {
  const cards = goalCards(goal);
  if (!cards.length) return undefined;
  return cards.find(c => c.id === cardId) || (cards[0].kind === 'final' ? cards[0] : [...cards].reverse().find(c => c.output) || cards[0]);
}

function emptyMessage(card) {
  return {
    'In Progress': 'The assistant is working on this. The result appears here when it finishes.',
    'Needs Your Approval': 'Waiting for your approval (see buttons above).',
    'Blocked': 'No result yet. See the message above.',
    'Review': 'Result is being checked.'
  }[card.status] || 'Queued. The assistant picks this up shortly.';
}

function cardNotice(card) {
  const last = card.history.length ? card.history[card.history.length - 1].text : '';
  if (card.status === 'Blocked') return 'Needs attention: ' + last;
  if (card.status === 'Needs Your Approval') return card.gate === 'done' ? (card.review_shrink ? 'The manager could only review an excerpt of this long result. Read it in full, then approve or reject it.' : card.manual_signoff ? 'The manager could not back its approval with evidence from this result. Read it, then approve or reject it.' : 'Review the result below, then approve or reject it.') : 'This task is waiting for your approval before it runs.';
  return '';
}

function cardButtons(card) {
  const button = (action, label, cls) => `<button type="button" class="${cls}" data-card="${card.id}" data-action="${action}">${label}</button>`;
  if (card.status === 'Needs Your Approval') return button('approve', 'Approve', 'btn-approve') + button('reject', 'Reject', 'btn-reject');
  if (card.status === 'Blocked') return button('retry', card.phase === 'verify' ? 'Retry review' : 'Try again', 'btn-retry');
  return '';
}

function renderWaiting(goal) {
  $('reader-title').textContent = goalTitle(goal);
  $('reader-meta').innerHTML = badge('Inbox') + '<span>Waiting for the manager to pick this up…</span>';
  $('task-tabs').innerHTML = $('card-actions').innerHTML = '';
  $('card-alert').hidden = $('reader-preview').hidden = $('reader-iframe').hidden = $('reader-raw').hidden = $('chat-form').hidden = true;
  $('reader-copy').disabled = $('reader-download').disabled = true;
  $('reader-formatted').hidden = false;
  $('reader-formatted').innerHTML = '<p>Starting. The result appears here when the assistant finishes.</p>';
  $('usage').textContent = $('workspace').textContent = '';
  $('history').innerHTML = '';
}

function renderResult() {
  const goal = currentGoal();
  $('result-empty').hidden = !!goal;
  $('result-view').hidden = !goal;
  if (!goal) return;
  const cards = goalCards(goal), card = currentCard(goal);
  if (!card) return renderWaiting(goal);
  const res = extractResult(card), file = deliverable(card, view === 'raw');
  const worker = card.owner, bot = assistants[worker]?.label || worker;
  const updated = card.history.length ? new Date(card.history[card.history.length - 1].at).toLocaleString() : '';
  const tasks = goalTasks(goal), finished = tasks.filter(c => c.status === 'Done').length;
  const progress = goal.direct ? '' : tasks.length ? `<span>${finished} of ${tasks.length} subtasks done</span>` : '<span>Manager is planning…</span>';
  $('reader-title').textContent = goalTitle(goal);
  $('reader-meta').innerHTML = `<strong>${escape(card.kind === 'final' ? 'Manager + team' : bot)}</strong>${statusBadge(card)}${progress}<span>Updated ${escape(updated)}</span>`;
  $('task-tabs').innerHTML = cards.length > 1 ? cards.map(c => `<button type="button" data-tab="${c.id}" aria-current="${c.id === card.id}">${escape(c.title)} ${statusBadge(c)}</button>`).join('') : '';
  const notice = cardNotice(card);
  $('card-alert').hidden = !notice;
  $('card-alert').textContent = notice;
  $('card-actions').innerHTML = cardButtons(card);
  $('reader-preview').hidden = !file.html;
  $('reader-preview').textContent = view === 'preview' ? 'Show text' : 'Live preview';
  $('reader-toggle-raw').textContent = view === 'raw' ? 'Formatted' : 'Diagnostics';
  $('reader-copy').disabled = $('reader-download').disabled = !res.text;
  $('chat-form').hidden = !(card.status === 'Done' || card.status === 'Blocked');
  $('reader-iframe').hidden = view !== 'preview' || !file.html;
  $('reader-raw').hidden = view !== 'raw';
  $('reader-formatted').hidden = view !== 'formatted';
  if (!$('reader-iframe').hidden) $('reader-iframe').srcdoc = file.html;
  $('reader-raw').textContent = res.raw || 'No raw output recorded.';
  $('reader-formatted').innerHTML = res.text ? formatContent(res.text) : `<p>${escape(emptyMessage(card))}</p>`;
  $('usage').textContent = `Tokens ${goal.tokens}/${goal.token_limit} · Runs ${goal.runs}/${goal.run_limit}`;
  $('workspace').textContent = goal.workspace ? 'Files saved in: ' + goal.workspace : '';
  $('history').innerHTML = card.history.map(item => `<p>${escape(new Date(item.at).toLocaleString())} · ${escape(item.actor)}<br>${escape(item.text)}</p>`).join('');
  if (editing !== goal.id) {
    const form = $('settings-form');
    for (const [name] of settingFields) form.elements[name].value = goal[name];
    for (const [name] of settingRules) form.elements[name].checked = goal.rules[name];
    editing = goal.id;
  }
}

function renderList() {
  $('work-list').innerHTML = [...board.goals].reverse().map(g => {
    const worker = g.workers[0], owner = g.direct ? assistants[worker]?.label || worker : 'Manager + team';
    return `<button type="button" class="work-item" data-goal="${g.id}" aria-current="${g.id === goalId}"><span>${escape(goalTitle(g))}<small>${escape(owner)}</small></span>${badge(g.status)}</button>`;
  }).join('') || '<p>Nothing yet.</p>';
}

function renderBoard() {
  if (!$('board-view').open) return;
  $('board').innerHTML = states.map(status => {
    const cards = board.cards.filter(c => c.status === status);
    return `<section class="column"><h3>${status} · ${cards.length}</h3>${cards.map(c => `<article class="card"><strong>${escape(c.title)}</strong><br><small>${escape(c.owner)}</small><button type="button" data-open="${c.id}" data-goal="${c.goal}">Open</button></article>`).join('')}</section>`;
  }).join('');
}

function draw() {
  const signature = JSON.stringify([goalId, cardId, view, board, $('board-view').open]);
  if (signature === rendered) return;
  rendered = signature;
  renderList();
  renderBoard();
  renderResult();
}

function select(id, card = '') {
  goalId = id; cardId = card; view = 'formatted'; editing = ''; rendered = '';
  store.set('studio-goal', id);
  draw();
}

async function refresh() {
  if (refreshing) return;
  refreshing = true;
  try {
    const result = await api('board');
    board = result.board;
    if (!currentGoal() && board.goals.length) goalId = board.goals[board.goals.length - 1].id;
    if (!botsLoaded) await loadAssistants();
    if (connectionError) notify('');
    $('connection').className = '';
    $('connection').textContent = result.error ? 'Storage error: ' + result.error : 'Connected';
    draw();
  } catch (error) {
    $('connection').className = 'bad';
    $('connection').textContent = 'Offline, retrying…';
    notify(error.message, true);
  } finally { refreshing = false; }
}

async function mutation(button, run) {
  if (button) button.disabled = true;
  try {
    notify('');
    await run();
    await refresh();
  } catch (error) { notify(error.message); }
  finally { if (button) button.disabled = false; }
}

function chooseBot(id) {
  botId = id;
  document.querySelectorAll('#assistants button').forEach(b => b.setAttribute('aria-pressed', b.dataset.bot === id));
  $('prompt-hint').textContent = assistants[id]?.hint || 'Describe what you need:';
  $('prompt-input').placeholder = assistants[id]?.placeholder || '';
}

async function loadAssistants() {
  const all = (await api('bots')).filter(b => b.compatibility === 'compatible (prompt-only)').map(b => b.bot.id);
  workerIds = all.filter(id => id !== 'manager');
  hasManager = all.includes('manager');
  const bots = hasManager ? ['auto', ...workerIds] : workerIds;
  $('assistants').innerHTML = bots.map(id => `<button type="button" data-bot="${escape(id)}" aria-pressed="false">${escape(assistants[id]?.label || id)}<small>${escape(assistants[id]?.about || '')}</small></button>`).join('');
  botsLoaded = true;
  chooseBot(bots[0]);
}

$('assistants').addEventListener('click', e => { const b = e.target.closest('button[data-bot]'); if (b) chooseBot(b.dataset.bot); });

$('goal-form').addEventListener('submit', event => {
  event.preventDefault();
  const form = event.currentTarget, auto = botId === 'auto';
  mutation(event.submitter, async () => {
    if (!botId) throw Error('No assistant is available yet. Install a bot package first.');
    if (!workerIds.length) throw Error('No worker bots are installed.');
    const goal = await api('goals', {...settings(form), requirements: new FormData(form).get('requirements'), direct: !auto, manager: auto ? 'manager' : '', workers: auto ? workerIds : [botId]});
    $('prompt-input').value = '';
    board.goals.push(goal);
    select(goal.id);
  });
});
$('prompt-input').addEventListener('keydown', e => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) $('goal-form').requestSubmit(); });

$('work-list').addEventListener('click', e => { const b = e.target.closest('button[data-goal]'); if (b) select(b.dataset.goal); });
$('task-tabs').addEventListener('click', e => { const b = e.target.closest('button[data-tab]'); if (b) { cardId = b.dataset.tab; view = 'formatted'; rendered = ''; draw(); } });
$('board').addEventListener('click', e => { const b = e.target.closest('button[data-open]'); if (b) { select(b.dataset.goal, b.dataset.open); $('result-panel').scrollIntoView({behavior: 'smooth'}); } });
$('board-view').addEventListener('toggle', () => { rendered = ''; draw(); });

for (const [id, mode] of [['reader-preview', 'preview'], ['reader-toggle-raw', 'raw']]) {
  $(id).addEventListener('click', () => { view = view === mode ? 'formatted' : mode; rendered = ''; draw(); });
}

$('reader-copy').addEventListener('click', async () => {
  const card = currentCard(currentGoal()), button = $('reader-copy');
  if (!card) return;
  const {body} = deliverable(card, view === 'raw');
  try { await navigator.clipboard.writeText(body); } catch (_) {
    const area = Object.assign(document.createElement('textarea'), {value: body});
    document.body.appendChild(area); area.select(); document.execCommand('copy'); area.remove();
  }
  button.textContent = 'Copied!';
  setTimeout(() => { button.textContent = 'Copy'; }, 1500);
});

$('reader-download').addEventListener('click', () => {
  const goal = currentGoal(), card = currentCard(goal);
  if (!card) return;
  const {body, ext} = deliverable(card, view === 'raw');
  const link = Object.assign(document.createElement('a'), {
    href: URL.createObjectURL(new Blob([body], {type: 'text/plain;charset=utf-8'})),
    download: (goalTitle(goal).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'result') + '.' + ext
  });
  document.body.appendChild(link); link.click(); link.remove();
  URL.revokeObjectURL(link.href);
});

$('card-actions').addEventListener('click', event => {
  const button = event.target.closest('button[data-card]');
  if (button) mutation(button, () => api(`cards/${button.dataset.card}/${button.dataset.action}`, {note: button.dataset.action + ' by user'}));
});

$('chat-form').addEventListener('submit', event => {
  event.preventDefault();
  const form = event.currentTarget, goal = currentGoal(), card = currentCard(goal);
  if (!card) return;
  mutation(event.submitter, async () => {
    await api(`goals/${goal.id}/chat`, {text: new FormData(form).get('text')});
    if (card.status === 'Blocked' || (goal.direct && card.status === 'Done')) await api(`cards/${card.id}/retry`, {note: 'changes requested'});
    form.reset();
    cardId = card.id;
  });
});

$('settings-form').addEventListener('submit', event => {
  event.preventDefault();
  mutation(event.submitter, async () => { await api(`goals/${goalId}/settings`, settings(event.currentTarget)); editing = ''; });
});

refresh();
setInterval(refresh, 2000);
