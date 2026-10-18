// Regression: a goal whose first task card has not loaded yet must render a waiting state, not crash.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const file = process.argv[2] || 'internal/service/web/board.js';
const elements = {};
const element = id => elements[id] ??= {id, hidden: false, open: false, disabled: false, value: '', innerHTML: '', textContent: '', className: '',
  elements: new Proxy({}, {get: () => ({})}), setAttribute() {}, addEventListener() {}, requestSubmit() {}, closest: () => null};
const now = new Date().toISOString();
const goal = {id: 'g1', requirements: 'Write a poem\nabout the sea', direct: false, workers: ['writer'], status: 'Inbox', tokens: 0, token_limit: 100, runs: 0, run_limit: 5,
  priority: 3, run_tokens: 1, seconds: 1, tool_calls: 0, rules: {}};
const doneCard = {id: 'c1', goal: 'g1', title: 'Poem', owner: 'writer', kind: 'task', status: 'Done', phase: 'work', output: 'Roses', history: [{at: now, actor: 'writer', text: 'ok'}]};

async function run(cards) {
  const board = {goals: [goal], cards};
  const sandbox = {
    document: {getElementById: element, querySelectorAll: () => [], createElement: () => element('t' + Math.random()), body: {appendChild() {}}},
    location: {hash: '', pathname: '/board'}, history: {replaceState() {}},
    sessionStorage: {getItem: () => null, setItem() {}}, localStorage: {getItem: () => null, setItem() {}},
    URLSearchParams, FormData: class { get() { return ''; } has() { return false; } }, setInterval() {}, setTimeout, console, JSON, Object, Error,
    fetch: async path => ({ok: true, json: async () => path.endsWith('board') ? {board, error: ''} : [{bot: {id: 'writer'}, compatibility: 'compatible (prompt-only)'}]})
  };
  vm.createContext(sandbox);
  vm.runInContext(fs.readFileSync(file, 'utf8'), sandbox);
  await new Promise(resolve => setTimeout(resolve, 50));
  return {connection: element('connection').textContent, notice: element('notice').textContent, title: element('reader-title').textContent, body: element('reader-formatted').innerHTML};
}

const empty = await run([]);
assert.equal(empty.connection, 'Connected', 'page must not go offline: ' + empty.notice);
assert.equal(empty.title, 'Write a poem', 'goal title shown while waiting');
assert.match(empty.body, /Starting/, 'waiting message shown');

const normal = await run([doneCard]);
assert.equal(normal.connection, 'Connected', normal.notice);
assert.match(normal.body, /Roses/, 'finished result still renders');
console.log('board empty-goal regression: ok');
