// 灵动岛标签 Demo：共享状态、设置面板、快捷键、重命名。各方案的渲染在 variants.js。
'use strict';

const PALETTE = ['#d4253a', '#f67500', '#ffd01c', '#65c802', '#0f75e9', '#a40bd8', '#98989d',
  '#00a6a6', '#ff4f9a', '#6b5cff', '#8d6e63', '#2fb170'];
const MAX_TABS = 12;
const STORE = 'zakki-island-demo-v1';

const S = load() || {
  active: 0,
  tabs: [
    { name: '今日待办', color: PALETTE[0], text: '# 今日待办\n- [ ] 写周报\n- [ ] 回复邮件' },
    { name: '', color: PALETTE[1], text: '没有命名的标签，标题显示取决于设置里的“未命名时显示”。' },
    { name: '一个非常非常长的标签名称用来测试超长省略效果', color: PALETTE[2], text: '标题超出最大宽度会省略。' },
    { name: '灵感', color: PALETTE[3], text: '' },
    { name: '读书笔记', color: PALETTE[4], text: '《设计心理学》第 3 章' },
    { name: '', color: PALETTE[5], text: '' },
    { name: 'Scratch', color: PALETTE[6], text: '' },
  ].map((t, i) => ({ id: i + 1, ...t })),
  cfg: {},
};
// 新加的设置项带默认值合并，旧 localStorage 也能用。
S.cfg = { dark: 0, islandTone: 'dark', mono: 0, wallpaper: 'aurora', fallback: 'index', maxW: 120, speed: 1,
  falloff: 0.78, dock: 1, preview: 1, wheel: 1, ...S.cfg };
const NUM_KEYS = new Set(['dark', 'mono', 'dock', 'preview', 'wheel']);
let nextId = Math.max(...S.tabs.map(t => t.id)) + 1;
let lastDir = 1; // 最近一次切换方向：1 向右，-1 向左。
const VARIANTS = [];

function load() { try { return JSON.parse(localStorage.getItem(STORE)); } catch { return null; } }
function save() { localStorage.setItem(STORE, JSON.stringify(S)); }

// ---- 外观辅助 ----
const isEmpty = t => t.text.trim() === '';
function dotColor(t) {
  if (S.cfg.mono) return S.cfg.dark ? '#d1d1d6' : '#3a3a3c';
  return t.color;
}
function emptyColor() { return S.cfg.dark ? 'rgba(255,255,255,.22)' : 'rgba(0,0,0,.18)'; }
function title(t, i) {
  if (t.name.trim()) return t.name.trim();
  if (S.cfg.fallback === 'index') return `标签 ${i + 1}`;
  if (S.cfg.fallback === 'first') {
    const line = t.text.split('\n').find(l => l.trim());
    return line ? line.replace(/^[#>\-*\s\[\]x]+/i, '').trim() || `标签 ${i + 1}` : `标签 ${i + 1}`;
  }
  return '';
}
// 浅色底（黄/绿）上用深色字。
function inkOn(hex) {
  const n = parseInt(hex.slice(1), 16), r = n >> 16, g = (n >> 8) & 255, b = n & 255;
  return (0.299 * r + 0.587 * g + 0.114 * b) / 255 > 0.62 ? '#1d1d1f' : '#fff';
}
const measureCtx = document.createElement('canvas').getContext('2d');
function textW(s) {
  measureCtx.font = '600 12px -apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif';
  return Math.min(Math.ceil(measureCtx.measureText(s).width) + 1, S.cfg.maxW);
}
const dur = () => Math.round(420 * S.cfg.speed);
const h = (tag, cls, html) => { const e = document.createElement(tag); if (cls) e.className = cls; if (html) e.innerHTML = html; return e; };

// ---- 动作 ----
function switchTo(i) {
  i = Math.max(0, Math.min(S.tabs.length - 1, i));
  if (i === S.active) return;
  lastDir = i > S.active ? 1 : -1;
  const prev = S.active;
  S.active = i;
  renderAll(prev);
}
function addTab() {
  if (S.tabs.length >= MAX_TABS) return toast(`最多 ${MAX_TABS} 个标签`);
  const used = new Set(S.tabs.map(t => t.color));
  const color = PALETTE.find(c => !used.has(c)) || PALETTE[S.tabs.length % PALETTE.length];
  S.tabs.push({ id: nextId++, name: '', color, text: '' });
  toast(`已新建标签 ${S.tabs.length}`);
  switchTo(S.tabs.length - 1);
}
function removeTab(i) {
  if (S.tabs.length <= 1) return toast('至少保留一个标签');
  const t = S.tabs[i];
  if (!isEmpty(t) && !confirm(`删除「${title(t, i) || '标签 ' + (i + 1)}」？内容会先备份再删除。`)) return;
  S.tabs.splice(i, 1);
  const prev = S.active;
  if (S.active >= S.tabs.length || S.active > i) S.active = Math.max(0, S.active - 1);
  renderAll(prev === i ? -1 : prev);
}
function moveTab(i, d) {
  const j = i + d;
  if (j < 0 || j >= S.tabs.length) return;
  [S.tabs[i], S.tabs[j]] = [S.tabs[j], S.tabs[i]];
  if (S.active === i) S.active = j; else if (S.active === j) S.active = i;
  renderAll();
}
function renameTab(i, name) { S.tabs[i].name = name.slice(0, 60); renderAll(); }

// ---- 卡片外壳：顶栏 + 内容区 + 底栏，track 交给方案渲染 ----
const ICON_CLEAR = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="9"/><path d="m15 9-6 6M9 9l6 6"/></svg>';
const ICON_PIN = '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 17v5M9 10.8V6h6v4.8l2 2.2v2H7v-2z"/></svg>';
const cards = [];

function mountCards() {
  const grid = document.getElementById('grid');
  VARIANTS.forEach(v => {
    if (v.group) grid.append(h('div', 'group', v.group));
    const card = h('section', 'card');
    card.append(h('h2', '', v.name), h('p', 'desc', v.desc));
    const win = h('div', 'win');
    const hdr = h('div', 'hdr');
    const track = h('div', 'track');
    hdr.append(h('span', 'ico', ICON_CLEAR), track, h('span', 'ico', ICON_PIN));
    const body = h('textarea', 'body');
    body.spellcheck = false;
    body.addEventListener('input', () => { S.tabs[S.active].text = body.value; renderAll(); });
    const status = h('div', 'status');
    win.append(hdr, body, status);
    card.append(win);
    grid.append(card);
    const ctx = { v, win, track, body, status, state: {} };
    track.addEventListener('dblclick', () => openRename(ctx));
    // 滚轮/触控板切换：累计到阈值才跳一格，避免触控板惯性连跳。
    let acc = 0, lock = 0;
    hdr.addEventListener('wheel', e => {
      if (!S.cfg.wheel) return;
      e.preventDefault();
      const now = Date.now();
      acc += Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : e.deltaY;
      if (now < lock || Math.abs(acc) < 40) return;
      switchTo(S.active + Math.sign(acc));
      acc = 0; lock = now + 260;
    }, { passive: false });
    v.mount(ctx);
    cards.push(ctx);
  });
}

function renderAll(prev = S.active) {
  save();
  const t = S.tabs[S.active];
  cards.forEach(ctx => {
    ctx.v.render(ctx, prev);
    if (document.activeElement !== ctx.body) ctx.body.value = t.text;
    ctx.body.placeholder = `在「${title(t, S.active) || '标签 ' + (S.active + 1)}」里记点什么…`;
    const lines = t.text ? t.text.split('\n').length : 0;
    ctx.status.innerHTML = `<i style="background:${dotColor(t)}"></i>${lines} 行 · ${[...t.text].length} 字符<span style="flex:1"></span>⌥${S.active < 9 ? S.active + 1 : '…'}`;
  });
  renderPanel();
}

// ---- 重命名浮层 ----
let renameCtx = null;
function openRename(ctx = cards[0]) {
  closeRename();
  renameCtx = ctx;
  const i = S.active, t = S.tabs[i];
  const box = h('div', 'rename', `<input maxlength="60" placeholder="标签 ${i + 1}"><small>Enter 确认 · Esc 取消 · 留空恢复默认</small>`);
  box.style.setProperty('--ac', t.color);
  const input = box.querySelector('input');
  input.value = t.name;
  let done = false;
  const commit = ok => { if (done) return; done = true; if (ok) renameTab(i, input.value.trim()); closeRename(); };
  input.addEventListener('keydown', e => {
    if (e.key === 'Enter') commit(true);
    if (e.key === 'Escape') commit(false);
    e.stopPropagation();
  });
  input.addEventListener('blur', () => commit(true));
  ctx.win.append(box);
  input.focus(); input.select();
}
function closeRename() { if (renameCtx) { renameCtx.win.querySelector('.rename')?.remove(); renameCtx = null; } }

// ---- 设置面板 ----
function renderPanel() {
  const list = document.getElementById('tabs');
  if (list.contains(document.activeElement)) return; // 正在面板里改名时不重建，免得丢光标。
  list.innerHTML = '';
  S.tabs.forEach((t, i) => {
    const row = h('div', 'trow' + (i === S.active ? ' on' : ''));
    const sw = h('button', 'sw'); sw.style.background = t.color; sw.title = '点击换颜色';
    sw.onclick = () => { t.color = PALETTE[(PALETTE.indexOf(t.color) + 1) % PALETTE.length]; renderAll(); };
    const name = h('input'); name.value = t.name; name.placeholder = `标签 ${i + 1}`; name.maxLength = 60;
    name.onfocus = () => switchTo(i);
    name.oninput = () => { t.name = name.value; save(); cards.forEach(c => c.v.render(c, S.active)); };
    name.onblur = () => renderAll();
    const k = h('span', 'k', i < 9 ? `⌥${i + 1}` : '');
    const up = h('button', 'b', '↑'), dn = h('button', 'b', '↓'), del = h('button', 'b', '✕');
    up.onclick = () => moveTab(i, -1); dn.onclick = () => moveTab(i, 1); del.onclick = () => removeTab(i);
    up.title = '上移'; dn.title = '下移'; del.title = '删除';
    row.append(sw, name, k, up, dn, del);
    list.append(row);
  });
  document.getElementById('add').disabled = S.tabs.length >= MAX_TABS;
}

function applyCfg() {
  const c = S.cfg, b = document.body;
  b.classList.toggle('dark', !!c.dark);
  b.classList.toggle('island-light', c.islandTone === 'light');
  b.className = b.className.replace(/wp-\w+/g, '').trim() + ' wp-' + c.wallpaper;
  b.style.setProperty('--dur', dur() + 'ms');
  document.querySelectorAll('.seg').forEach(seg => seg.querySelectorAll('button').forEach(btn =>
    btn.classList.toggle('on', String(c[seg.dataset.k]) === btn.dataset.v)));
  document.querySelectorAll('input[type=range]').forEach(r => { r.value = c[r.dataset.k]; });
  document.getElementById('maxWv').textContent = c.maxW + 'px';
  document.getElementById('falloffv').textContent = Number(c.falloff).toFixed(2);
  document.getElementById('speedv').textContent = '×' + Number(c.speed).toFixed(1);
}

function bindPanel() {
  document.querySelectorAll('.seg').forEach(seg => seg.addEventListener('click', e => {
    const v = e.target.dataset.v; if (v == null) return;
    const k = seg.dataset.k;
    S.cfg[k] = NUM_KEYS.has(k) ? Number(v) : v;
    applyCfg(); renderAll();
  }));
  document.querySelectorAll('input[type=range]').forEach(r => r.addEventListener('input', () => {
    S.cfg[r.dataset.k] = Number(r.value); applyCfg(); renderAll();
  }));
  document.getElementById('add').onclick = addTab;
}

// ---- 快捷键（浏览器占用 ⌘ 数字，Demo 用 ⌥；用 e.code，因为 ⌥ 会改变 e.key）----
document.addEventListener('keydown', e => {
  if (!e.altKey || e.metaKey || e.ctrlKey) return;
  const inField = /^(INPUT|TEXTAREA)$/.test(document.activeElement?.tagName);
  const m = e.code.match(/^Digit([1-9])$/);
  let hit = true;
  if (m) switchTo(Number(m[1]) - 1);
  else if (e.code === 'ArrowLeft' && !inField) switchTo(S.active - 1);
  else if (e.code === 'ArrowRight' && !inField) switchTo(S.active + 1);
  else if (e.code === 'BracketLeft') switchTo(S.active - 1);
  else if (e.code === 'BracketRight') switchTo(S.active + 1);
  else if (e.code === 'KeyN') addTab();
  else if (e.code === 'KeyR') openRename(cards.find(c => c.win.contains(document.activeElement)) || cards[0]);
  else if (e.code === 'Backspace' && !inField) removeTab(S.active);
  else hit = false;
  if (hit) e.preventDefault();
});

// ---- 悬停预览：标题 + 前三行 ----
const peekEl = h('div', 'peek');
document.body.append(peekEl);
function peek(el, t) {
  const i = S.tabs.indexOf(t);
  if (!S.cfg.preview || i === S.active || i < 0) return;
  const r = el.getBoundingClientRect();
  const lines = t.text.split('\n').filter(l => l.trim()).slice(0, 3);
  peekEl.innerHTML = '';
  const b = h('b'); b.textContent = title(t, i) || `标签 ${i + 1}`; b.style.color = dotColor(t);
  const s = h('span'); s.textContent = lines.length ? lines.join('\n') : '（空）';
  peekEl.append(b, s);
  peekEl.style.left = r.left + r.width / 2 + 'px';
  peekEl.style.top = r.bottom + 10 + 'px';
  peekEl.classList.add('show');
}
function unpeek() { peekEl.classList.remove('show'); }

let toastTimer;
function toast(msg) {
  const el = document.getElementById('toast');
  el.textContent = msg; el.classList.add('show');
  clearTimeout(toastTimer); toastTimer = setTimeout(() => el.classList.remove('show'), 1400);
}

function boot() {
  applyCfg(); bindPanel(); mountCards(); renderAll();
  window.addEventListener('resize', () => renderAll());
}
