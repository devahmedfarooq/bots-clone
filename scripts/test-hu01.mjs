import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import vm from 'node:vm';

const projectRoot = path.resolve('.');
const boardJsPath = path.join(projectRoot, 'internal/service/web/board.js');
const boardJsCode = fs.readFileSync(boardJsPath, 'utf8');

// Create sandbox with DOM stubs
const domElements = {};
const getOrCreate = id => {
  if (!domElements[id]) {
    domElements[id] = {
      id,
      innerHTML: '',
      textContent: '',
      hidden: false,
      style: {},
      addEventListener: () => {},
      elements: {},
      querySelector: () => null,
      querySelectorAll: () => []
    };
  }
  return domElements[id];
};

const sandbox = {
  location: { hash: '#token=test-token-123', pathname: '/board' },
  history: { replaceState: () => {} },
  sessionStorage: {
    data: {},
    getItem(k) { return this.data[k] || null; },
    setItem(k, v) { this.data[k] = String(v); }
  },
  document: {
    getElementById: getOrCreate,
    querySelectorAll: () => [],
    addEventListener: () => {}
  },
  window: { addEventListener: () => {} },
  setInterval: () => {},
  fetch: async () => ({ ok: true, json: async () => ({ board: { goals: [], cards: [] } }) }),
  URLSearchParams,
  console
};

vm.createContext(sandbox);
vm.runInContext(boardJsCode, sandbox);

console.log('--- Testing extractResult ---');
// 1. Raw text
const rawCard = { output: 'Simple poem line 1\nLine 2' };
const res1 = sandbox.extractResult(rawCard);
assert.equal(res1.text, 'Simple poem line 1\nLine 2');
assert.equal(res1.isJson, false);

// 2. JSON envelope
const jsonCard = { output: JSON.stringify({ status: 'result', output: 'The woods are lovely, dark and deep,\nBut I have promises to keep.', message: 'Done' }) };
const res2 = sandbox.extractResult(jsonCard);
assert.equal(res2.isJson, true);
assert.equal(res2.text, 'The woods are lovely, dark and deep,\nBut I have promises to keep.');

// 3. JSON codeblock envelope
const codeblockCard = { output: '```json\n{"status":"result","output":"Stanza 1\\nStanza 2"}\n```' };
const res3 = sandbox.extractResult(codeblockCard);
assert.equal(res3.isJson, true);
assert.equal(res3.text, 'Stanza 1\nStanza 2');

console.log('--- Testing formatContent (Poem line breaks & paragraphs) ---');
const poem = 'Whose woods these are I think I know.\nHis house is in the village though;\n\nHe will not see me stopping here\nTo watch his woods fill up with snow.';
const poemHtml = sandbox.formatContent(poem);
assert(poemHtml.includes('<p>Whose woods these are I think I know.<br>His house is in the village though;</p>'), 'Preserves stanza 1 line breaks with <br>');
assert(poemHtml.includes('<p>He will not see me stopping here<br>To watch his woods fill up with snow.</p>'), 'Preserves stanza 2 line breaks with <br>');

console.log('--- Testing formatContent (Markdown code blocks & tables) ---');
const mdText = 'Intro paragraph\n\n```python\ndef hello():\n    return 42\n```\n\n| Item | Count |\n|---|---|\n| Apple | 5 |\n| Orange | 10 |';
const mdHtml = sandbox.formatContent(mdText);
assert(mdHtml.includes('<pre><code>def hello():'), 'Formats code block');
assert(mdHtml.includes('<table><thead><tr><th>Item</th><th>Count</th></tr></thead>'), 'Formats table header');
assert(mdHtml.includes('<td>Apple</td><td>5</td>'), 'Formats table cells');

console.log('--- Testing formatContent (XSS and hostile injection safety) ---');
const hostile = '<script>alert("pwned")</script><img src=x onerror=evil()>';
const safeHtml = sandbox.formatContent(hostile);
assert(!safeHtml.includes('<script>'), 'Must not contain raw <script>');
assert(!safeHtml.includes('<img'), 'Must not contain raw <img');
assert(safeHtml.includes('&lt;script&gt;'), 'Must escape <script>');
assert(safeHtml.includes('&lt;img'), 'Must escape <img');

console.log('--- Testing statusBadge and review states ---');
const statesToCheck = [
  { status: 'Done', expected: 'Verified Deliverable', badgeClass: 'badge-delivered' },
  { status: 'Review', expected: 'Awaiting Review', badgeClass: 'badge-review' },
  { status: 'In Progress', expected: 'Running', badgeClass: 'badge-running' },
  { status: 'Blocked', expected: 'Blocked', badgeClass: 'badge-blocked' },
  { status: 'Needs Your Approval', expected: 'Needs Approval', badgeClass: 'badge-review' },
  { status: 'Ready', expected: 'Waiting', badgeClass: 'badge-waiting' }
];

for (const s of statesToCheck) {
  const badge = sandbox.statusBadge({ status: s.status });
  assert(badge.includes(s.expected), `Expected ${s.expected} in badge for ${s.status}`);
  assert(badge.includes(s.badgeClass), `Expected ${s.badgeClass} for ${s.status}`);
  if (s.status !== 'Done') {
    assert(!badge.includes('Verified Deliverable'), `Must not label ${s.status} as Verified Deliverable`);
  }
}

console.log('All HU-01 unit and contract assertions passed successfully!');
