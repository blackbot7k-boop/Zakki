// 图标方案定义。
// app(u)：返回 1024×1024 画布内的 SVG 内容，u 用于生成唯一 id（同页多次渲染不冲突）。
// tray(fg, color)：返回高 18 的菜单栏图标内容；fg 是单色前景，color=true 时允许彩色点缀。
// trayW：菜单栏图标宽度（viewBox 宽），默认 18。
const P = { r: '#d4253a', o: '#f67500', y: '#ffd01c', g: '#65c802', b: '#0f75e9', p: '#a40bd8', gr: '#98989d', c: '#00a3b4', pk: '#f7429c', in: '#5856d6', sky: '#32ade6' };
const DOT7 = [P.r, P.o, P.y, P.g, P.b, P.p, P.gr];

// 近似 macOS 图标的超椭圆底板（n=5），占 1024 画布中央 824。
function squircle(cx = 512, cy = 512, a = 412, n = 5, steps = 256) {
  let d = '';
  for (let i = 0; i < steps; i++) {
    const t = (i / steps) * Math.PI * 2, c = Math.cos(t), s = Math.sin(t);
    const x = cx + a * Math.sign(c) * Math.pow(Math.abs(c), 2 / n);
    const y = cy + a * Math.sign(s) * Math.pow(Math.abs(s), 2 / n);
    d += (i ? 'L' : 'M') + x.toFixed(1) + ' ' + y.toFixed(1);
  }
  return d + 'Z';
}
const TILE = squircle();

const lin = (id, a, b, x2 = 1, y2 = 1) =>
  `<linearGradient id="${id}" x1="0" y1="0" x2="${x2}" y2="${y2}"><stop offset="0" stop-color="${a}"/><stop offset="1" stop-color="${b}"/></linearGradient>`;

// 底板：投影 + 渐变底色 + 顶部微光 + 浅色底板的发丝描边。
function tile(u, [a, b], body, { light = false, gloss = 0.14 } = {}) {
  return `<defs>
    <filter id="${u}sh" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="10" stdDeviation="12" flood-opacity=".26"/></filter>
    <clipPath id="${u}cl"><path d="${TILE}"/></clipPath>
    ${lin(u + 'bg', a, b)}
    <linearGradient id="${u}gl" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity="${gloss}"/><stop offset=".55" stop-color="#fff" stop-opacity="0"/></linearGradient>
  </defs>
  <path d="${TILE}" fill="url(#${u}bg)" filter="url(#${u}sh)"/>
  <g clip-path="url(#${u}cl)">${body}<path d="${TILE}" fill="url(#${u}gl)"/></g>
  ${light ? `<path d="${TILE}" fill="none" stroke="#000" stroke-opacity=".08" stroke-width="3"/>` : ''}`;
}

// 圆弧段：以 centerDeg 为中心、可见弧长 visDeg（含圆头）的一段描边。
function arc(cx, cy, r, sw, centerDeg, visDeg, color) {
  const C = 2 * Math.PI * r, capDeg = (sw / 2 / r) * 180 / Math.PI;
  const dashDeg = Math.max(visDeg - 2 * capDeg, 0.01), dash = C * dashDeg / 360;
  return `<circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="${color}" stroke-width="${sw}" stroke-linecap="round"
    stroke-dasharray="${dash.toFixed(2)} ${(C - dash).toFixed(2)}" transform="rotate(${(centerDeg - dashDeg / 2).toFixed(2)} ${cx} ${cy})"/>`;
}
const polar = (cx, cy, r, deg) => [cx + r * Math.cos(deg * Math.PI / 180), cy + r * Math.sin(deg * Math.PI / 180)];
const line = (x1, y1, x2, y2, c, w, extra = '') => `<path d="M${x1} ${y1}L${x2} ${y2}" stroke="${c}" stroke-width="${w}" stroke-linecap="round" fill="none" ${extra}/>`;
const bar = (x, y, w, h, c, op = 1) => `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="${h / 2}" fill="${c}" fill-opacity="${op}"/>`;

function sevenDots(dark) {
  return (u) => tile(u, dark ? ['#2c2c2e', '#161618'] : ['#ffffff', '#e9e9ee'],
    DOT7.map((c, i) => { const [x, y] = polar(512, 512, 232, -90 + i * 360 / 7); return `<circle cx="${x}" cy="${y}" r="74" fill="${dark && c === P.gr ? '#6e6e73' : c}"/>`; }).join(''),
    { light: !dark, gloss: dark ? 0.08 : 0.3 });
}
const sevenDotsTray = (fg, color) =>
  DOT7.map((c, i) => { const [x, y] = polar(9, 9, 6.2, -90 + i * 360 / 7); return `<circle cx="${x}" cy="${y}" r="1.6" fill="${color ? c : fg}"/>`; }).join('');

const OPTIONS = [
  {
    id: 'seven-dots', name: '七点', tag: '经典配色',
    desc: '七个标签色围成一圈，直接对应 经典 7 点导航。浅色纸感底板，辨识度来自色彩本身。',
    app: sevenDots(false), tray: sevenDotsTray,
  },
  {
    id: 'seven-dots-night', name: '七点 · 夜', tag: '经典配色',
    desc: '同一套七点，换成深色底板，在 Dock 里更沉稳、颜色更跳。',
    app: sevenDots(true), tray: sevenDotsTray,
  },
  {
    id: 'spectrum-ring', name: '光谱环', tag: '抽象',
    desc: '圆环被切成 7 段标签色，延续现有「白环」托盘图标的轮廓，过渡最平滑。',
    app: (u) => tile(u, ['#2c2c2e', '#111113'], DOT7.map((c, i) => arc(512, 512, 250, 92, -90 + i * 360 / 7, 360 / 7 - 7, c)).join(''), { gloss: 0.08 }),
    tray: (fg, color) => DOT7.map((c, i) => arc(9, 9, 6.3, 2.2, -90 + i * 360 / 7, 360 / 7 - 12, color ? c : fg)).join(''),
  },
  {
    id: 'ring-orbit', name: '轨道环', tag: '现有演进',
    desc: '保留当前蓝紫渐变与白环，在右上角开一个缺口放一颗黄点，像光标落在便签上。',
    app: (u) => { const [x, y] = polar(512, 512, 232, -45); return tile(u, ['#4f7cf7', '#9a5cf5'], arc(512, 512, 232, 84, 135, 308, '#fff') + `<circle cx="${x}" cy="${y}" r="58" fill="${P.y}"/>`); },
    tray: (fg, color) => { const [x, y] = polar(9, 9, 6.2, -45); return arc(9, 9, 6.2, 2, 135, 302, fg) + `<circle cx="${x}" cy="${y}" r="1.9" fill="${color ? P.y : fg}"/>`; },
  },
  {
    id: 'island-tabs', name: '胶囊标签', tag: '界面写实',
    desc: '把应用顶栏缩成图标：当前标签是红色胶囊，其余是彩点，下面是几行笔记。',
    app: (u) => tile(u, ['#fbfbfd', '#e4e4ea'],
      bar(204, 318, 280, 124, P.r) + bar(250, 368, 150, 24, '#fff', 0.92) +
      [P.o, P.y, P.g].map((c, i) => `<circle cx="${552 + i * 116}" cy="380" r="44" fill="${c}"/>`).join('') +
      bar(250, 540, 360, 44, '#1d1d1f', 0.85) + bar(250, 628, 524, 30, '#c7c7cc') + bar(250, 690, 460, 30, '#c7c7cc') + bar(250, 752, 300, 30, '#c7c7cc'),
      { light: true, gloss: 0.3 }),
    trayW: 22,
    tray: (fg, color) => bar(1.5, 5.5, 9.5, 7, color ? P.r : fg) + `<circle cx="14.6" cy="9" r="2" fill="${color ? P.o : fg}"/><circle cx="19.6" cy="9" r="2" fill="${color ? P.y : fg}"/>`,
  },
  {
    id: 'sticky-note', name: '折角便签', tag: '具象',
    desc: '黄色底板上一张折角便签，标题前带 Markdown 的 #，一眼看出是写字的地方。',
    app: (u) => tile(u, ['#ffe066', '#ffbf1c'], `
      <defs><filter id="${u}ns" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="14" stdDeviation="18" flood-color="#8a5a00" flood-opacity=".3"/></filter></defs>
      <g transform="rotate(-5 512 512)">
        <path d="M330 260H694Q734 260 734 300V634L604 764H330Q290 764 290 724V300Q290 260 330 260Z" fill="#fffdf6" filter="url(#${u}ns)"/>
        <path d="M734 634H634Q604 634 604 664V764Z" fill="#ece4cc"/>
        ${line(380, 322, 368, 398, P.o, 13)}${line(412, 322, 400, 398, P.o, 13)}${line(352, 346, 432, 346, P.o, 13)}${line(348, 374, 428, 374, P.o, 13)}
        ${bar(452, 338, 210, 44, '#2c2c2e')}${bar(350, 450, 330, 30, '#ddd8ca')}${bar(350, 512, 300, 30, '#ddd8ca')}${bar(350, 574, 330, 30, '#ddd8ca')}${bar(350, 636, 200, 30, '#ddd8ca')}
      </g>`, { gloss: 0.18 }),
    tray: (fg) => `<path d="M5 2.5H13.5A1.5 1.5 0 0 1 15 4V11L11 15.5H5A1.5 1.5 0 0 1 3.5 14V4A1.5 1.5 0 0 1 5 2.5Z" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linejoin="round"/>
      <path d="M15 11H12.5A1.5 1.5 0 0 0 11 12.5V15.5" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linejoin="round"/>
      ${line(6.2, 6.5, 12.2, 6.5, fg, 1.5)}${line(6.2, 9.5, 10, 9.5, fg, 1.5)}`,
  },
  {
    id: 'z-monogram', name: 'Z 字标', tag: '字标',
    desc: '一笔写成的 Z 加一个黄色句点，取 Zakki 首字母，靛→粉渐变更有个性。',
    app: (u) => tile(u, [P.in, P.pk], `<path d="M265 320H625L265 704H625" fill="none" stroke="#fff" stroke-width="112" stroke-linecap="round" stroke-linejoin="round"/><circle cx="772" cy="704" r="54" fill="${P.y}"/>`),
    tray: (fg, color) => `<path d="M3.2 4.6H11.4L3.2 13.4H11.4" fill="none" stroke="${fg}" stroke-width="2.1" stroke-linecap="round" stroke-linejoin="round"/><circle cx="15.2" cy="13.4" r="1.55" fill="${color ? P.y : fg}"/>`,
  },
  {
    id: 'dot-caret', name: '墨点光标', tag: '极简',
    desc: '一颗经典红点加一根输入光标：「点一下就开始写」。米色纸感，最安静的一款。',
    app: (u) => tile(u, ['#fbf8f1', '#ece5d6'], `<circle cx="459" cy="512" r="172" fill="${P.r}"/>${bar(690, 326, 48, 372, '#1d1d1f')}`, { light: true, gloss: 0.3 }),
    tray: (fg, color) => `<circle cx="7" cy="9" r="4.6" fill="${color ? P.r : fg}"/>${bar(13.4, 3.2, 2, 11.6, fg)}`,
  },
  {
    id: 'stacked-cards', name: '叠放卡片', tag: '具象',
    desc: '三张错落的卡片代表多个标签页，最前面一张写着笔记，蓝色系清爽。',
    app: (u) => tile(u, [P.b, P.sky], `
      <defs><filter id="${u}cs" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="12" stdDeviation="16" flood-color="#002a66" flood-opacity=".3"/></filter></defs>
      <rect x="302" y="252" width="420" height="520" rx="64" fill="#fff" fill-opacity=".28" transform="rotate(-16 512 512)"/>
      <rect x="302" y="252" width="420" height="520" rx="64" fill="#fff" fill-opacity=".5" transform="rotate(-7 512 512)"/>
      <g transform="rotate(4 512 512)">
        <rect x="302" y="252" width="420" height="520" rx="64" fill="#fff" filter="url(#${u}cs)"/>
        <circle cx="384" cy="352" r="32" fill="${P.r}"/>${bar(438, 334, 210, 36, '#1d1d1f', 0.85)}
        ${bar(362, 432, 300, 28, '#c7c7cc')}${bar(362, 490, 270, 28, '#c7c7cc')}${bar(362, 548, 300, 28, '#c7c7cc')}${bar(362, 606, 190, 28, '#c7c7cc')}
      </g>`),
    tray: (fg, color) => `<path d="M2.8 12.6V4.2A2 2 0 0 1 4.8 2.2H11.4" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linecap="round" stroke-opacity=".6"/>
      <rect x="5.6" y="4.8" width="10" height="11.4" rx="2" fill="none" stroke="${fg}" stroke-width="1.5"/>
      <circle cx="8.4" cy="8.4" r="1.1" fill="${color ? P.r : fg}"/>${line(10.6, 8.4, 13, 8.4, fg, 1.4)}${line(8.2, 11.6, 13, 11.6, fg, 1.4)}`,
  },
  {
    id: 'markdown-hash', name: '彩色井号', tag: 'Markdown',
    desc: '四笔标签色组成 Markdown 的 #，深色底板，强调「Markdown 便签」这个定位。',
    app: (u) => tile(u, ['#2c2c2e', '#111113'],
      line(318, 430, 726, 430, P.b, 78) + line(298, 594, 706, 594, P.g, 78) + line(452, 300, 410, 724, P.r, 78) + line(636, 300, 594, 724, P.o, 78), { gloss: 0.08 }),
    tray: (fg, color) => line(3.2, 6.8, 15.6, 6.8, color ? P.b : fg, 1.8) + line(2.4, 11.2, 14.8, 11.2, color ? P.g : fg, 1.8) +
      line(7.8, 2.8, 6.6, 15.2, color ? P.r : fg, 1.8) + line(12.8, 2.8, 11.6, 15.2, color ? P.o : fg, 1.8),
  },
  {
    id: 'kanji-seal', name: '雑 · 印章', tag: '字标',
    desc: 'Zakki 即日文「雑記」。朱红印章框里一个「雑」字，东方气质最强；依赖系统宋体/明朝体渲染。',
    app: (u) => tile(u, ['#e2403f', '#b51d2c'], `
      <rect x="236" y="236" width="552" height="552" rx="54" fill="none" stroke="#fff" stroke-opacity=".92" stroke-width="20"/>
      <text x="512" y="530" text-anchor="middle" dominant-baseline="central" font-size="400" font-weight="700" fill="#fff"
        font-family="'Hiragino Mincho ProN','Songti SC','Noto Serif CJK SC',serif">雑</text>`),
    tray: (fg) => `<rect x="1.8" y="1.8" width="14.4" height="14.4" rx="3" fill="none" stroke="${fg}" stroke-width="1.4"/>
      <text x="9" y="9.6" text-anchor="middle" dominant-baseline="central" font-size="10.5" font-weight="700" fill="${fg}"
        font-family="'Hiragino Sans','PingFang SC',sans-serif">雑</text>`,
  },
  {
    id: 'current', name: '当前图标', tag: '对照',
    desc: '现在 resources/icon.png 的样子（蓝紫渐变 + 白环），放进来做对照。',
    app: (u) => tile(u, ['#4f7cf7', '#9a5cf5'], `<circle cx="512" cy="512" r="230" fill="none" stroke="#fff" stroke-width="96"/>`),
    tray: (fg) => `<circle cx="9" cy="9" r="6.25" fill="none" stroke="${fg}" stroke-width="2.5"/>`,
  },
];
