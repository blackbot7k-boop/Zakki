// 第二组：点的显示效果。都复用中央岛（标题位置稳定），差别在其余点怎么摆、怎么缩放。
'use strict';

// 共用：建一座带弹簧过渡的岛，返回 render 时调用的更新函数。
function mountIsland(ctx) {
  const st = ctx.state;
  st.map = new Map();
  st.layer = h('div', ''); st.layer.style.cssText = 'position:absolute;inset:0';
  st.island = h('div', 'island d-island', '<span class="idot"></span><span class="ttl"></span>');
  ctx.track.append(st.layer, st.island);
  st.island.addEventListener('click', () => openRename(ctx));
}
function placeIsland(ctx, left, width) {
  const st = ctx.state, isl = st.island, t = S.tabs[S.active];
  Object.assign(isl.style, { left: left + 'px', width: width + 'px', top: (TRACK_H - 24) / 2 + 'px' });
  isl.firstChild.style.background = dotColor(t);
  const changed = st.lastId !== undefined && st.lastId !== t.id;
  swapTitle(isl.lastChild, title(t, S.active), changed);
  if (changed) isl.animate([{ transform: 'scale(1)' }, { transform: 'scale(.9,.84)', offset: 0.3 },
    { transform: 'scale(1.04,1.06)', offset: 0.65 }, { transform: 'scale(1)' }], { duration: dur(), easing: 'ease-out' });
  st.lastId = t.id;
}
const islandW = (W, reserve) => { const { w } = activeTitleW(W, reserve); return w ? 9 + 8 + 6 + w + 11 : 26; };
// 岛两侧依次排开大小不一的点；sizeOf(k) 给出相对偏移 k 处的直径。
function sideLayout(W, iw, sizeOf, gap) {
  const n = S.tabs.length, a = S.active;
  let lw = 0, rw = 0;
  for (let i = 0; i < n; i++) if (i !== a) (i < a ? (lw += sizeOf(i - a) + gap) : (rw += sizeOf(i - a) + gap));
  // 岛居中；若一侧放不下就整体平移（窄窗口时）。
  let il = (W - iw) / 2;
  il = Math.max(lw + 2, Math.min(il, W - iw - rw - 2));
  const out = [];
  let x = il;
  for (let i = a - 1; i >= 0; i--) { const s = sizeOf(i - a); x -= gap + s; out[i] = { x, s }; }
  x = il + iw;
  for (let i = a + 1; i < n; i++) { const s = sizeOf(i - a); x += gap; out[i] = { x, s }; x += s; }
  out[a] = { x: il + iw / 2 - DOT / 2, s: DOT };
  return { il, out };
}
function setDot(el, x, s, y, extra = {}) {
  Object.assign(el.style, { left: x + 'px', top: (y ?? (TRACK_H - s) / 2) + 'px', width: s + 'px', height: s + 'px', ...extra });
}

// ---- E 距离衰减 + Dock 放大（你的想法）----
VARIANTS.push({
  group: '第二组 · 点的显示效果（都以中央岛为基础）',
  name: 'E · 距离衰减 + Dock 放大',
  desc: '离当前标签越远的点越小越淡，一眼看出“我在哪、两边还有多少”。鼠标划过时像 macOS Dock 一样局部放大，方便点中远处的小点。',
  mount(ctx) {
    mountIsland(ctx);
    const st = ctx.state;
    ctx.track.addEventListener('mousemove', e => {
      if (!S.cfg.dock) return;
      st.mx = e.clientX - ctx.track.getBoundingClientRect().left;
      ctx.track.classList.add('hover'); this.render(ctx);
    });
    ctx.track.addEventListener('mouseleave', () => { st.mx = null; ctx.track.classList.remove('hover'); this.render(ctx); });
  },
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.layer, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const iw = islandW(W, 30);
    const base = k => Math.max(5, DOT * Math.pow(S.cfg.falloff, Math.abs(k) - 1));
    // 先按基础尺寸算出圆心，再用圆心与鼠标的距离算放大倍数（与 Dock 一致，避免布局抖动反馈）。
    const { out: b } = sideLayout(W, iw, base, 7);
    const boost = k => {
      if (st.mx == null) return 1;
      const i = k + S.active, c = b[i].x + b[i].s / 2;
      return 1 + 0.75 * Math.max(0, 1 - Math.abs(c - st.mx) / 46);
    };
    const { il, out } = sideLayout(W, iw, k => Math.min(16, base(k) * boost(k)), 7);
    els.forEach((el, i) => {
      const k = i - S.active, { x, s } = out[i];
      setDot(el, x, s, undefined, { background: inkFor(S.tabs[i]),
        opacity: k === 0 ? 0 : Math.max(0.35, Math.pow(S.cfg.falloff + 0.08, Math.abs(k) - 1)) });
    });
    placeIsland(ctx, il, iw);
  },
});

// ---- F 邻近展开 · 远端折叠 ----
VARIANTS.push({
  name: 'F · 邻近展开 · 远端折叠',
  desc: '只完整显示两侧最近的 2 个点，更远的叠成一摞“扑克牌”并标出数量。标签多到 10+ 时顶栏依然干净；悬停一摞会把它摊开。',
  mount(ctx) {
    mountIsland(ctx);
    const st = ctx.state;
    st.badges = [h('div', 'ttl'), h('div', 'ttl')];
    st.badges.forEach((b, side) => {
      b.style.cssText = 'position:absolute;top:12px;font-size:10px;font-weight:600;color:var(--muted);pointer-events:none;transition:left var(--dur) var(--spring),opacity .2s';
      ctx.track.append(b);
    });
    st.open = 0; // -1 左摞摊开，1 右摞摊开
    ctx.track.addEventListener('mousemove', e => {
      const x = e.clientX - ctx.track.getBoundingClientRect().left, W = ctx.track.clientWidth;
      const o = x < W * 0.28 ? -1 : x > W * 0.72 ? 1 : 0;
      if (o !== st.open) { st.open = o; this.render(ctx); }
    });
    ctx.track.addEventListener('mouseleave', () => { st.open = 0; this.render(ctx); });
  },
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state, NEAR = 2;
    const els = sync(st.layer, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const iw = islandW(W, 120);
    const il = (W - iw) / 2, gap = 9, n = S.tabs.length, a = S.active;
    const hidden = [0, 0];
    els.forEach((el, i) => {
      const k = i - a, d = Math.abs(k), side = k < 0 ? -1 : 1;
      if (k === 0) return setDot(el, il + iw / 2 - DOT / 2, DOT, undefined, { opacity: 0, zIndex: 0 });
      const edge = k < 0 ? il - gap : il + iw + gap;
      if (d <= NEAR || st.open === side) {
        const x = k < 0 ? edge - d * DOT - (d - 1) * gap : edge + (d - 1) * (DOT + gap);
        return setDot(el, x, DOT, undefined, { background: inkFor(S.tabs[i]), opacity: 1, zIndex: 1, boxShadow: '' });
      }
      // 远端：叠成一摞，每张只露 3px。
      hidden[k < 0 ? 0 : 1]++;
      const j = d - NEAR - 1, s = 10;
      const stackEdge = k < 0 ? edge - NEAR * (DOT + gap) - s : edge + NEAR * (DOT + gap);
      setDot(el, stackEdge + (k < 0 ? -j * 3 : j * 3), s, undefined, {
        background: inkFor(S.tabs[i]), opacity: Math.max(0.3, 0.85 - j * 0.15), zIndex: 1 - j,
        boxShadow: S.cfg.dark ? '0 0 0 1.5px #2a2a2e' : '0 0 0 1.5px rgba(255,255,255,.9)' });
    });
    // 数量角标
    st.badges.forEach((b, s) => {
      const cnt = hidden[s];
      b.textContent = cnt ? `+${cnt}` : '';
      b.style.opacity = cnt && st.open !== (s ? 1 : -1) ? 1 : 0;
      const edge = s === 0 ? il - gap - NEAR * (DOT + gap) - 10 - (cnt - 1) * 3 - 18 : il + iw + gap + NEAR * (DOT + gap) + 10 + (cnt - 1) * 3 + 5;
      b.style.left = edge + 'px';
    });
    placeIsland(ctx, il, iw);
  },
});

// ---- G 弧形转盘（透视）----
VARIANTS.push({
  name: 'G · 弧形转盘',
  desc: '点排在一段向下弯的弧上，像 iOS 时间选择器的滚轮：越远越小、越低、越淡。用触控板在顶栏上滑动就像拨动转盘。',
  mount: mountIsland,
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.layer, st.map, t => { const el = h('div', 'dot'); clickable(el, t); return el; });
    const iw = islandW(W, 120), il = (W - iw) / 2, step = 0.26; // 每格对应的弧度
    els.forEach((el, i) => {
      const k = i - S.active;
      if (k === 0) return setDot(el, W / 2 - DOT / 2, DOT, undefined, { opacity: 0 });
      const d = Math.abs(k), th = Math.min(d * step, 1.45);
      const s = Math.max(4, DOT * (0.55 + 0.45 * Math.cos(th)));
      // 弧上的水平投影按 sin 增长，越远的点挤得越密，形成透视。
      const c = 8 + DOT / 2 + 30 * (Math.sin(th) / Math.sin(step) - 1);
      const x = (k < 0 ? il - c : il + iw + c) - s / 2;
      const y = (TRACK_H - s) / 2 + (1 - Math.cos(th)) * 9;
      setDot(el, x, s, y, { background: inkFor(S.tabs[i]), opacity: th >= 1.45 ? 0 : 0.25 + 0.75 * Math.cos(th) });
    });
    placeIsland(ctx, il, iw);
  },
});

// ---- H 首字徽章 ----
VARIANTS.push({
  name: 'H · 首字徽章',
  desc: '已命名的标签在点里显示名字首字（“今”“灵”“读”），不用悬停就能认出；未命名的保持普通点。也结合了距离衰减，远处的徽章退化成小点。',
  mount: mountIsland,
  render(ctx) {
    const W = ctx.track.clientWidth, st = ctx.state;
    const els = sync(st.layer, st.map, t => { const el = h('div', 'dot badge'); clickable(el, t); return el; });
    const iw = islandW(W, 120);
    const size = k => Math.abs(k) <= 3 ? 17 : Math.max(6, 17 * Math.pow(S.cfg.falloff, Math.abs(k) - 3));
    const { il, out } = sideLayout(W, iw, size, 6);
    els.forEach((el, i) => {
      const t = S.tabs[i], k = i - S.active, { x, s } = out[i];
      const ch = t.name.trim() ? [...t.name.trim()][0] : '';
      const ink = inkFor(t), show = ch && s >= 14;
      el.textContent = show ? ch : '';
      setDot(el, x, s, undefined, {
        opacity: k === 0 ? 0 : 1,
        background: show ? ink : (isEmpty(t) ? 'transparent' : ink),
        boxShadow: !show && isEmpty(t) ? `inset 0 0 0 1.5px ${emptyColor()}` : '',
        color: show ? inkOn(S.cfg.mono ? ink : t.color) : 'transparent',
      });
    });
    placeIsland(ctx, il, iw);
  },
});
