// 第三批（续）：更多视觉风格。

// 四角星闪光（Y2K）。
const sparkle = (cx, cy, r, c = '#fff') =>
  `<path d="M${cx} ${cy - r}Q${cx} ${cy} ${cx + r} ${cy}Q${cx} ${cy} ${cx} ${cy + r}Q${cx} ${cy} ${cx - r} ${cy}Q${cx} ${cy} ${cx} ${cy - r}Z" fill="${c}"/>`;

// 等距立方薄板：顶面 + 左右侧面。
function isoSlab(cy, [top, left, right], hw = 240, hh = 140, t = 56) {
  const L = 512 - hw, R = 512 + hw;
  return `<path d="M${L} ${cy}L512 ${cy + hh}V${cy + hh + t}L${L} ${cy + t}Z" fill="${left}"/>
    <path d="M${R} ${cy}L512 ${cy + hh}V${cy + hh + t}L${R} ${cy + t}Z" fill="${right}"/>
    <path d="M512 ${cy - hh}L${R} ${cy}L512 ${cy + hh}L${L} ${cy}Z" fill="${top}"/>`;
}

// 青海波：自上而下逐行绘制，后一行盖住前一行形成鳞片。
function seigaiha(y0, y1, r, bg, fg) {
  let out = '';
  for (let row = 0, y = y0; y <= y1; row++, y += r / 2) {
    for (let x = row % 2 ? -r : 0; x <= 1024 + r; x += 2 * r) {
      out += `<circle cx="${x}" cy="${y}" r="${r}" fill="${bg}" stroke="${fg}" stroke-width="6"/>`;
      for (const k of [0.74, 0.48]) out += `<circle cx="${x}" cy="${y}" r="${r * k}" fill="none" stroke="${fg}" stroke-width="6"/>`;
      out += `<circle cx="${x}" cy="${y}" r="${r * 0.22}" fill="${fg}"/>`;
    }
  }
  return out;
}

OPTIONS.push(
  {
    id: 'style-sketch', name: '手绘草图', tag: '手绘', batch: 3,
    desc: '横线本上的铅笔速写：歪歪的便签框、几行字和一个红色对勾，线条带手抖的毛边。',
    app: (u) => tile(u, ['#fffdf7', '#f2ecdc'], `
      <defs><filter id="${u}rough"><feTurbulence type="fractalNoise" baseFrequency=".018" numOctaves="3" seed="7" result="n"/>
        <feDisplacementMap in="SourceGraphic" in2="n" scale="18"/></filter></defs>
      ${[...Array(14)].map((_, i) => `<path d="M0 ${150 + i * 64}H1024" stroke="#9ec5ee" stroke-opacity=".55" stroke-width="3"/>`).join('')}
      <path d="M208 0V1024" stroke="#f08a8a" stroke-opacity=".6" stroke-width="4"/>
      <g filter="url(#${u}rough)" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <path d="M300 250L736 266L724 784L290 770Z" stroke="#2b2b2b" stroke-width="18"/>
        <path d="M362 396L640 390M362 494L664 502M362 594L548 590" stroke="#2b2b2b" stroke-width="16"/>
        <path d="M566 674L616 726L704 610" stroke="${P.r}" stroke-width="24"/>
      </g>`, { light: true, gloss: 0.15 }),
    tray: (fg, color) => `<g transform="rotate(45 9 9)" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linejoin="round">
      <rect x="6.8" y="0.6" width="4.4" height="12" rx="1"/><path d="M6.8 12.6L9 16.8L11.2 12.6" fill="${color ? P.o : 'none'}"/>${line(6.8, 3.4, 11.2, 3.4, fg, 1.4)}</g>`,
  },
  {
    id: 'style-papercut', name: '剪纸层叠', tag: '剪纸', batch: 3,
    desc: '一层层剪纸山丘叠出景深，夕阳挂在上方。暖橙到深红，有手工纸艺的质感。',
    app: (u) => tile(u, ['#ffe2b8', '#ffa27e'], `
      <defs><filter id="${u}pc" x="-10%" y="-30%" width="120%" height="160%"><feDropShadow dx="0" dy="-10" stdDeviation="12" flood-color="#5a1010" flood-opacity=".32"/></filter></defs>
      <circle cx="640" cy="360" r="112" fill="#fff6e2"/>
      ${[['M0 600C200 520 360 640 540 580S860 500 1024 560', '#ff8a65'], ['M0 690C180 630 340 720 520 680S840 610 1024 670', '#f4511e'],
         ['M0 790C220 730 380 820 560 780S860 720 1024 770', '#c62828'], ['M0 880C200 840 420 900 600 870S880 830 1024 860', '#7b1424']]
        .map(([d, c]) => `<path d="${d}V1024H0Z" fill="${c}" filter="url(#${u}pc)"/>`).join('')}`, { gloss: 0.12 }),
    tray: (fg, color) => `<circle cx="12.8" cy="4.4" r="2.5" fill="${color ? P.o : fg}"/>
      <path d="M1 16L6.2 8.2L9.6 12.6L11.8 10L17 16Z" fill="${fg}" stroke="${fg}" stroke-width="1" stroke-linejoin="round"/>`,
  },
  {
    id: 'style-retro70', name: '70 年代复古', tag: '复古', batch: 3,
    desc: '奶油底上的四色彩虹拱门，砖红、橘、芥末黄、墨绿，70 年代海报配色。',
    app: (u) => tile(u, ['#fbf1dc', '#f1dfba'],
      [[312, '#e2553b'], [236, '#f29c38'], [160, '#f6c945'], [84, '#3a9e8f']]
        .map(([r, c]) => `<path d="M${512 - r} 712A${r} ${r} 0 0 1 ${512 + r} 712" fill="none" stroke="${c}" stroke-width="76"/>`).join('') +
      `<rect x="160" y="712" width="704" height="14" rx="7" fill="#7a4a2a" fill-opacity=".25"/>`, { light: true, gloss: 0.2 }),
    tray: (fg, color) => [[7, '#e2553b'], [4.6, '#f29c38'], [2.2, '#3a9e8f']]
      .map(([r, c]) => `<path d="M${9 - r} 14A${r} ${r} 0 0 1 ${9 + r} 14" fill="none" stroke="${color ? c : fg}" stroke-width="1.7"/>`).join(''),
  },
  {
    id: 'style-wa', name: '和风青海波', tag: '和风', batch: 3,
    desc: '朱红日轮从青海波纹样里升起，靛蓝与米白，呼应 Zakki（雑記）的日文出身。',
    app: (u) => tile(u, ['#f7f1e3', '#ede2c8'], `<circle cx="512" cy="430" r="176" fill="#d23a2f"/>${seigaiha(600, 1100, 96, '#22337a', '#f7f1e3')}`, { light: true, gloss: 0.1 }),
    tray: (fg, color) => `<path d="M3.4 10.4A5.6 5.6 0 0 1 14.6 10.4Z" fill="${color ? '#d23a2f' : fg}"/>
      <path d="M1.2 15.6A2.6 2.6 0 0 1 6.4 15.6A2.6 2.6 0 0 1 11.6 15.6A2.6 2.6 0 0 1 16.8 15.6" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linecap="round"/>`,
  },
  {
    id: 'style-brutal', name: '新粗野主义', tag: 'Neo-Brutalism', batch: 3,
    desc: '高饱和黄底、粗黑描边、硬投影，再贴一张红色圆贴纸。直白、大声、很网感。',
    app: (u) => tile(u, ['#ffe14d', '#ffd60a'], `
      <rect x="306" y="286" width="440" height="480" rx="18" fill="#111"/>
      <rect x="272" y="252" width="440" height="480" rx="18" fill="#fff" stroke="#111" stroke-width="22"/>
      <rect x="318" y="300" width="348" height="74" fill="#111"/>
      ${[[300, 420], [340, 488], [220, 556], [290, 624]].map(([w, y]) => `<rect x="318" y="${y}" width="${w}" height="32" fill="#111"/>`).join('')}
      <circle cx="712" cy="268" r="80" fill="${P.r}" stroke="#111" stroke-width="20"/>`, { gloss: 0 }),
    tray: (fg, color) => `<path d="M15.6 5.4V16H5" fill="none" stroke="${fg}" stroke-width="2"/>
      <rect x="2" y="2.6" width="11.6" height="11.6" rx="1" fill="none" stroke="${fg}" stroke-width="1.6"/>
      ${line(4.6, 6.6, 10.6, 6.6, fg, 1.6)}${line(4.6, 9.8, 9, 9.8, fg, 1.6)}<circle cx="14" cy="3.4" r="2.4" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'style-y2k', name: 'Y2K 镀铬', tag: 'Y2K', batch: 3,
    desc: '千禧年风格的镀铬 Z，冰蓝到丁香紫的渐变底，加几颗闪光星星。',
    app: (u) => tile(u, ['#9fd3ff', '#e3b8ff'], `
      <defs><linearGradient id="${u}cr" gradientUnits="userSpaceOnUse" x1="0" y1="270" x2="0" y2="760">
        <stop offset="0" stop-color="#ffffff"/><stop offset=".42" stop-color="#a7b0c2"/><stop offset=".5" stop-color="#3b4252"/>
        <stop offset=".58" stop-color="#d5dcea"/><stop offset="1" stop-color="#ffffff"/></linearGradient></defs>
      <path d="M300 330H700L330 700H730" fill="none" stroke="#2a2f45" stroke-width="146" stroke-linecap="round" stroke-linejoin="round"/>
      <path d="M300 330H700L330 700H730" fill="none" stroke="url(#${u}cr)" stroke-width="116" stroke-linecap="round" stroke-linejoin="round"/>
      <path d="M300 306H690" fill="none" stroke="#fff" stroke-opacity=".8" stroke-width="14" stroke-linecap="round"/>
      ${sparkle(774, 290, 64)}${sparkle(262, 780, 40)}${sparkle(810, 790, 26)}`, { gloss: 0.25 }),
    tray: (fg) => `<path d="M3 4.8H10.6L3.4 13.4H11" fill="none" stroke="${fg}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>${sparkle(14.8, 4.4, 2.9, fg)}${sparkle(15.2, 13.6, 1.6, fg)}`,
  },
  {
    id: 'style-iso', name: '等距层叠', tag: '等距 3D', batch: 3,
    desc: '三块等距视角的彩色薄板叠在一起，代表叠放的多个标签页，深色底，偏科技感。',
    app: (u) => tile(u, ['#2f3248', '#14151f'],
      isoSlab(640, ['#4f9dff', '#2c6fd6', '#1d4fa8']) + isoSlab(500, ['#8fe04a', '#5aa818', '#3f7d0c']) + isoSlab(360, [P.r, '#a81a2c', '#7a1020']), { gloss: 0.06 }),
    tray: (fg, color) => `<path d="M9 2L16 5.6L9 9.2L2 5.6Z" fill="${color ? P.r : 'none'}" stroke="${color ? P.r : fg}" stroke-width="1.4" stroke-linejoin="round"/>
      <path d="M2 9.2L9 12.8L16 9.2M2 12.6L9 16.2L16 12.6" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>`,
  },
);
