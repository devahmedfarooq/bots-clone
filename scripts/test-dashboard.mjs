import { spawn } from 'node:child_process';
import { readFile, writeFile, mkdir, cp } from 'node:fs/promises';
import path from 'node:path';
import assert from 'node:assert/strict';

const project = process.cwd();
const session = JSON.parse(await readFile('.local/service-smoke-session.json', 'utf8').then(text => text.replace(/^\uFEFF/, '')));
const localRoot = path.resolve(project, '.local') + path.sep;
assert(path.resolve(session.root).startsWith(localRoot), 'Test state must be inside .local');
const log = await readFile(path.join(session.root, 'stderr.log'), 'utf8');
const token = log.match(/#token=([a-f0-9]{64})/)?.[1];
assert(token, 'Start scripts/test-service.ps1 -KeepRunning first');
const url = `http://127.0.0.1:${session.port}`;
const profile = path.join(session.root, 'ui-test-profile');
await mkdir(profile, { recursive: true });
const browser = spawn('C:/Program Files/Google/Chrome/Application/chrome.exe', [
  '--headless', '--disable-gpu', '--no-first-run', '--disable-background-networking',
  `--user-data-dir=${profile}`, '--remote-debugging-port=0', '--window-size=1280,1000'
], { stdio: 'ignore', windowsHide: true });
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
async function until(check, timeout = 15000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) { const value = await check(); if (value) return value; await sleep(100); }
  throw Error('Dashboard check timed out');
}
let socket;
const pending = new Map();
let sequence = 0;
function send(method, params = {}, sessionId) {
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timer = setTimeout(() => { pending.delete(id); reject(Error(`Timed out: ${method}`)); }, 15000);
    pending.set(id, { resolve, reject, timer });
    socket.send(JSON.stringify({ id, method, params, ...(sessionId ? { sessionId } : {}) }));
  });
}
try {
  const active = await until(async () => { try { return await readFile(path.join(profile, 'DevToolsActivePort'), 'utf8'); } catch { return false; } });
  const [port, endpoint] = active.trim().split(/\r?\n/);
  socket = new WebSocket(`ws://127.0.0.1:${port}${endpoint}`);
  await new Promise((resolve, reject) => { socket.onopen = resolve; socket.onerror = reject; });
  socket.onmessage = event => {
    const message = JSON.parse(event.data);
    const entry = pending.get(message.id);
    if (!entry) return;
    clearTimeout(entry.timer); pending.delete(message.id);
    if (message.error) entry.reject(Error(message.error.message)); else entry.resolve(message.result);
  };
  const target = await send('Target.createTarget', { url: `${url}/#token=${token}` });
  const attached = await send('Target.attachToTarget', { targetId: target.targetId, flatten: true });
  async function evaluate(expression) {
    const result = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true }, attached.sessionId);
    if (result.exceptionDetails) throw Error(result.exceptionDetails.text);
    return result.result.value;
  }
  await until(() => evaluate("document.getElementById('bot')?.options.length === 2"));
  assert.equal(await evaluate('location.hash'), '', 'Bootstrap token must leave address bar');
  const botDirectory = path.join(session.root, 'ui-extract');
  await cp(path.join(project, 'bots/extract'), botDirectory, { recursive: true });
  const definition = JSON.parse(await readFile(path.join(botDirectory, 'bot.json'), 'utf8'));
  definition.id = 'ui-extract';
  await writeFile(path.join(botDirectory, 'bot.json'), JSON.stringify(definition));
  await evaluate(`document.getElementById('directory').value=${JSON.stringify(botDirectory)};document.getElementById('install-form').requestSubmit();`);
  await until(() => evaluate("document.getElementById('bot').options.length === 3"));
  await until(() => evaluate("!document.querySelector('.htmx-settling, .htmx-swapping, .htmx-request')"));
  assert.equal(await evaluate("document.querySelectorAll('#bot').length"), 1, 'Picker refresh must target the form rather than the status area');
  await evaluate("document.getElementById('bot').value='ui-extract';document.getElementById('bot').dispatchEvent(new Event('change'));document.getElementById('input').value='Invoice INV-205 from Pine Goods. Total EUR 73.25.';document.getElementById('run-form').requestSubmit();");
  await until(() => evaluate("Array.from(document.querySelectorAll('#jobs .job')).some(card=>card.querySelector('strong')?.textContent==='ui-extract' && card.querySelector('.tag')?.textContent==='completed')"), 240000);
  const output = await evaluate("Array.from(document.querySelectorAll('#jobs .job')).find(card=>card.querySelector('strong')?.textContent==='ui-extract').querySelector('pre').textContent");
  const invoice = JSON.parse(output);
  assert.equal(invoice.invoice_number, 'INV-205'); assert.equal(invoice.total, 73.25);
  await evaluate("const card=Array.from(document.querySelectorAll('#jobs .job')).find(card=>card.querySelector('strong')?.textContent==='ui-extract');card.querySelector('summary').click();card.querySelector('details button').click();");
  await until(() => evaluate("Array.from(document.querySelectorAll('#jobs .job')).find(card=>card.querySelector('strong')?.textContent==='ui-extract').querySelectorAll('pre').length===2"));
  await sleep(2500);
  assert(await evaluate("Array.from(document.querySelectorAll('#jobs .job')).find(card=>card.querySelector('strong')?.textContent==='ui-extract').querySelector('details').open"), 'Output must remain open across refresh');
  await evaluate("document.getElementById('bot').value='workspace-qa';document.getElementById('input').value='Read project.txt and write a long detailed explanation of every fact.';document.getElementById('run-form').requestSubmit();");
  await until(() => evaluate("Array.from(document.querySelectorAll('#jobs button')).some(button=>button.textContent.trim()==='Cancel task')"));
  await evaluate("Array.from(document.querySelectorAll('#jobs button')).find(button=>button.textContent.trim()==='Cancel task').click();");
  await until(() => evaluate("document.querySelector('#jobs .job .tag')?.textContent==='cancelled'"));
  await send('Emulation.setDeviceMetricsOverride', { width: 390, height: 844, deviceScaleFactor: 1, mobile: true }, attached.sessionId);
  assert(await evaluate('document.documentElement.scrollWidth <= 390'), 'Mobile layout must not overflow');
  const screenshot = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: false }, attached.sessionId);
  await writeFile('evaluation/dashboard-mobile.png', Buffer.from(screenshot.data, 'base64'));
  await writeFile('evaluation/dashboard-acceptance.json', JSON.stringify({ checks: ['installed bot through UI', 'real UI task output', 'expanded output and trace survive refresh', 'UI cancellation', 'mobile no horizontal overflow', 'token removed from URL'], invoice }, null, 2));
  console.log('Dashboard acceptance passed.');
} finally {
  if (socket?.readyState === WebSocket.OPEN) { try { await send('Browser.close'); } catch {} socket.close(); }
  browser.kill();
}
