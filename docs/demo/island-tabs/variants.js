// 四种顶部标签方案。每个方案：mount(ctx) 建层，render(ctx, prev) 按 S 摆放并播放切换动画。
'use strict';

const DOT = 12, TRACK_H = 38;

// 按 id 复用节点，让 CSS transition 能从旧位置过渡到新位置。
function sync(layer, map, make) {
  const live = new Set(S.tabs.map(t => t.id));
  for (const [id, el] of map) if (!live.has(id)) { el.remove(); map.delete(id); }
  S.tabs.forEach(t => { if (!map.has(t.id)) { const el = make(t); map.set(t.id, el); layer.append(el); } });
  return S.tabs.map(t => map.get(t.id));
}
// 居中排一行；放不下时先压缩间距。
function rowLayout(W, ws, gapMax = 12) {
  const n = ws.length, sum = ws.reduce((a, b) => a + b, 0);
  const gap = n > 1 ? Math.max(4, Math.min(gapMax, (W - sum) / (n - 1))) : 0;
  let x = (W - sum - gap * (n - 1)) / 2;
  return ws.map(w => { const l = x; x += w + gap; return l; });
}
// 当前标签的标题宽度：受设置最大宽度和窗口剩余空间双重限制。
function activeTitleW(W, extra) {
  const tt = title(S.tabs[S.active], S.active);
  if (!tt) return { tt, w: 0 };
  const room = W - (S.tabs.length - 1) * (DOT + 5) - extra - 8;
  return { tt, w: Math.max(0, Math.min(textW(tt), room)) };
}
const inkFor = t => isEmpty(t) && S.tabs.indexOf(t) !== S.active ? emptyColor() : dotColor(t);
function clickable(el, t) {
  el.addEventListener('click', () => { unpeek(); switchTo(S.tabs.indexOf(t)); });
  el.addEventListener('mouseenter', () => peek(el, t));
  el.addEventListener('mouseleave', unpeek);
}
// 标题换字：沿切换方向滑入。
function swapTitle(el, text, changed) {
  if (el.textContent === text && !changed) return;
  el.textContent = text;
  if (changed) el.animate([
    { opacity: 0, transform: `translateX(${8 * lastDir}px)`, filter: 'blur(3px)' },
    { opacity: 1, transform: 'none', filter: 'blur(0)' },
  ], { duration: dur() * 0.7, easing: 'cubic-bezier(.2,.8,.2,1)', delay: dur() * 0.15, fill: 'backwards' });
}

// ---- A 胶囊展开 ----
VARIANTS.push({
  group: '第一组 · 切换动效',
  name: 'A · 胶囊展开',
  desc: '当前标签原地展开成一枚玻璃胶囊，里面是彩色点和标题；其他点让位，整体弹簧动画。最接近现在的版式，改动最小。',
  mount(ctx) { ctx.state.map = new Map(); },
  render(ctx) {
    const W = ctx.track.clientWidth;
    const els = sync(ctx.track, ctx.state.map, t => {
      const el = h('div', 'pill', '<span class="pd"></span><span class="ttl"></span>');
      clickable(el, t); return el;
    });
    const { tt, w } = activeTitleW(W, 32);
    const ws = S.tabs.map((_, i) => i === S.active ? (w ? 22 + w + 10 : 22) : DOT);
    const xs = rowLayout(W, ws, 10);
    els.forEach((el, i) => {
      const t = S.tabs[i], on = i === S.active, hh = on ? 22 : DOT;
      el.classList.toggle('on', on);
      Object.assign(el.style, { left: xs[i] + 'px', width: ws[i] + 'px', height: hh + 'px', top: (TRACK_H - hh) / 2 + 'px',
        borderRadius: hh / 2 + 'px', background: on ? '' : inkFor(t) });
      const pd = el.firstChild;
      Object.assign(pd.style, on
        ? { width: '10px', height: '10px', left: '6px', top: '6px', background: dotColor(t), opacity: 1 }
        : { width: DOT + 'px', height: DOT + 'px', left: '0', top: '0', background: inkFor(t), opacity: 0 });
      el.lastChild.textContent = on ? tt : '';
    });
  },
});

// ---- B 灵动岛浮游 ----
VARIANTS.push({
  name: 'B · 灵动岛浮游',
  desc: '一座深色玻璃“岛”盖住当前点。切换时岛先收缩成水滴、带拖尾滑到目标，再弹开显示新标题，标题沿方向滑入。最像 iPhone。',
  mount(ctx) {
    ctx.state.map = new Map();
    ctx.state.layer = h('div', ''); ctx.state.layer.style.cssText = 'position:absolute;inset:0';
    ctx.state.island = h('div', 'island', '<span class="idot"></span><span class="ttl"></span>');
    ctx.track.append(ctx.state.layer, ctx.state.island);
  },
  render(ctx, prev) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.layer, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const { tt, w } = activeTitleW(W, 34);
    const iw = w ? 9 + 8 + 6 + w + 11 : 26;
    const ws = S.tabs.map((_, i) => i === S.active ? iw : DOT);
    const xs = rowLayout(W, ws, 11);
    els.forEach((el, i) => Object.assign(el.style, {
      left: xs[i] + (i === S.active ? (iw - DOT) / 2 : 0) + 'px', top: (TRACK_H - DOT) / 2 + 'px',
      width: DOT + 'px', height: DOT + 'px', background: inkFor(S.tabs[i]), opacity: i === S.active ? 0 : 1 }));
    const isl = st.island, t = S.tabs[S.active];
    const to = { left: xs[S.active], width: iw };
    const from = st.last || to;
    Object.assign(isl.style, { left: to.left + 'px', width: to.width + 'px', top: (TRACK_H - 24) / 2 + 'px' });
    isl.firstChild.style.background = dotColor(t);
    const changed = st.lastId !== undefined && st.lastId !== t.id;
    swapTitle(isl.lastChild, tt, changed);
    if (changed) {
      // 收缩成水滴 → 沿方向拉长滑行 → 落位后过冲回弹。
      const mid = (from.left + from.width / 2 + to.left + to.width / 2) / 2;
      const dist = Math.abs(to.left - from.left);
      const stretch = Math.min(26 + dist * 0.35, 70);
      isl.animate([
        { left: from.left + 'px', width: from.width + 'px', height: '24px', top: '7px' },
        { offset: 0.22, left: from.left + from.width / 2 - 13 + 'px', width: '26px', height: '20px', top: '9px' },
        { offset: 0.55, left: mid - stretch / 2 + 'px', width: stretch + 'px', height: '16px', top: '11px' },
        { offset: 0.8, left: to.left - 3 + 'px', width: to.width + 6 + 'px', height: '26px', top: '6px' },
        { left: to.left + 'px', width: to.width + 'px', height: '24px', top: '7px' },
      ], { duration: dur() * 1.25, easing: 'cubic-bezier(.4,0,.2,1)' });
    }
    st.last = to; st.lastId = t.id;
  },
});

// ---- C 液态融合 ----
VARIANTS.push({
  name: 'C · 液态融合',
  desc: '点和当前胶囊是同一种“液体”：胶囊滑过时会和沿途的点粘连再断开。当前胶囊用该标签颜色实心填充，辨识度最高。',
  mount(ctx) {
    ctx.state.map = new Map();
    ctx.state.goo = h('div', 'goo');
    ctx.state.active = h('div', 'blob');
    ctx.state.txt = h('div', 'gtxt', '<span class="ttl"></span>');
    ctx.state.goo.append(ctx.state.active);
    ctx.track.append(ctx.state.goo, ctx.state.txt);
    ctx.state.active.style.transition = 'left var(--dur) var(--spring), width var(--dur) var(--spring), background .3s';
  },
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.goo, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const { tt, w } = activeTitleW(W, 24);
    const aw = w ? w + 22 : 22;
    const ws = S.tabs.map((_, i) => i === S.active ? aw : DOT);
    const xs = rowLayout(W, ws, 12);
    els.forEach((el, i) => Object.assign(el.style, {
      left: xs[i] + (ws[i] - DOT) / 2 + 'px', top: (TRACK_H - DOT) / 2 + 'px', width: DOT + 'px', height: DOT + 'px',
      // 液态滤镜按 alpha 阈值裁切，空白点必须用不透明灰，否则会消失。
      background: i === S.active ? dotColor(S.tabs[i]) : isEmpty(S.tabs[i]) ? (S.cfg.dark ? '#58585c' : '#c4c4c8') : inkFor(S.tabs[i]) }));
    const t = S.tabs[S.active], col = dotColor(t);
    Object.assign(st.active.style, { left: xs[S.active] + 'px', width: aw + 'px', top: (TRACK_H - 22) / 2 + 'px', background: col });
    st.active.onclick = () => openRename(ctx);
    Object.assign(st.txt.style, { left: xs[S.active] + 'px', width: aw + 'px', top: (TRACK_H - 22) / 2 + 'px', color: inkOn(col),
      transition: 'left var(--dur) var(--spring), width var(--dur) var(--spring)' });
    st.txt.firstChild.style.width = '100%';
    st.txt.firstChild.style.textAlign = 'center';
    swapTitle(st.txt.firstChild, tt, st.lastId !== undefined && st.lastId !== t.id);
    st.lastId = t.id;
  },
});

// ---- D 中央岛 + 转盘 ----
VARIANTS.push({
  name: 'D · 中央岛 · 转盘',
  desc: '岛固定在顶栏正中，标题始终在同一位置，读起来最稳。切换时其余点像转盘一样从岛两侧滑过，岛本身做一次轻微的“呼吸”回弹。',
  mount(ctx) {
    ctx.state.map = new Map();
    ctx.state.mask = h('div', 'dmask');
    ctx.state.island = h('div', 'island d-island', '<span class="idot"></span><span class="ttl"></span>');
    ctx.track.append(ctx.state.mask, ctx.state.island);
  },
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.mask, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const { tt, w } = activeTitleW(W, 34);
    const iw = w ? 9 + 8 + 6 + w + 11 : 26;
    const il = (W - iw) / 2, gap = 9;
    els.forEach((el, i) => {
      const k = i - S.active; // 相对当前的偏移：负数在左，正数在右。
      const x = k < 0 ? il - 8 + (k + 1) * (DOT + gap) - DOT : k > 0 ? il + iw + 8 + (k - 1) * (DOT + gap) : W / 2 - DOT / 2;
      Object.assign(el.style, { left: x + 'px', top: (TRACK_H - DOT) / 2 + 'px', width: DOT + 'px', height: DOT + 'px',
        background: inkFor(S.tabs[i]), opacity: k === 0 ? 0 : Math.max(0.25, 1 - (Math.abs(k) - 1) * 0.12) });
    });
    const isl = st.island, t = S.tabs[S.active];
    Object.assign(isl.style, { left: il + 'px', width: iw + 'px', top: (TRACK_H - 24) / 2 + 'px' });
    isl.firstChild.style.background = dotColor(t);
    const changed = st.lastId !== undefined && st.lastId !== t.id;
    swapTitle(isl.lastChild, tt, changed);
    if (changed) {
      isl.animate([{ transform: 'scale(1)' }, { transform: 'scale(.92,.86)', offset: 0.3 }, { transform: 'scale(1.04,1.06)', offset: 0.65 }, { transform: 'scale(1)' }],
        { duration: dur(), easing: 'ease-out' });
      isl.animate([{ boxShadow: `0 0 0 0 ${dotColor(t)}aa` }, { boxShadow: `0 0 0 6px ${dotColor(t)}00` }], { duration: dur() * 1.2 });
    }
    st.lastId = t.id;
  },
});
