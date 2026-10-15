// 第二批图标方案：复用 icons.js 里的 P / tile / arc / polar / line / bar。
OPTIONS.forEach((o) => { o.batch = o.batch || 1; });

// 融球滤镜：模糊后阈值化 alpha，让相邻形状像液体一样粘连（对应标签切换的液态融合动画）。
const goo = (id, blur, k = 40, b = -18) => `<filter id="${id}" x="-20%" y="-20%" width="140%" height="140%">
  <feGaussianBlur in="SourceGraphic" stdDeviation="${blur}" result="b"/>
  <feColorMatrix in="b" mode="matrix" values="1 0 0 0 0 0 1 0 0 0 0 0 1 0 0 0 0 0 ${k} ${b}"/></filter>`;
const shadow = (id, dy, blur, color = '#000', op = 0.25) =>
  `<filter id="${id}" x="-30%" y="-30%" width="160%" height="160%"><feDropShadow dx="0" dy="${dy}" stdDeviation="${blur}" flood-color="${color}" flood-opacity="${op}"/></filter>`;

// 带凹口的书签色带。
const ribbon = (x, w, end, notch, c) => `<path d="M${x} -10H${x + w}V${end}L${x + w / 2} ${end - notch}L${x} ${end}Z" fill="${c}"/>`;

OPTIONS.push(
  {
    id: 'quick-bolt', name: '闪电速记', tag: '速记', batch: 2,
    desc: '一道白色闪电：想到就记、瞬间唤出。橙色暖调，Dock 里很醒目。',
    app: (u) => tile(u, ['#ffb340', '#f5511d'],
      `<path d="M566 214L322 562H500L452 810L706 450H528Z" fill="#fff" stroke="#fff" stroke-width="44" stroke-linejoin="round"/>`),
    tray: (fg, color) => `<path d="M10.2 1.8L4.4 10H8.6L7.6 16.2L13.6 7.6H9.4Z" fill="${color ? P.o : fg}" stroke="${color ? P.o : fg}" stroke-width="1" stroke-linejoin="round"/>`,
  },
  {
    id: 'ribbons', name: '书签色带', tag: 'Tot 传承', batch: 2,
    desc: '七条标签色书签从顶部垂下，长短不一，像夹在本子里的多个标签页。',
    app: (u) => tile(u, ['#fbfbfd', '#e4e4ea'],
      `<defs>${shadow(u + 'rs', 8, 10, '#000', 0.18)}</defs><g filter="url(#${u}rs)">` +
      TOT7.map((c, i) => ribbon(266 + i * 72, 60, [620, 540, 700, 480, 660, 570, 510][i], 34, c)).join('') + '</g>',
      { light: true, gloss: 0.3 }),
    tray: (fg, color) => [[2.6, 13, P.r], [7.4, 16, P.y], [12.2, 11, P.b]]
      .map(([x, end, c]) => `<path d="M${x} 1.5H${x + 3.4}V${end}L${x + 1.7} ${end - 1.8}L${x} ${end}Z" fill="${color ? c : fg}"/>`).join(''),
  },
  {
    id: 'liquid', name: '液态融合', tag: '动效', batch: 2,
    desc: '胶囊和圆点像液体一样粘在一起，取自标签切换时的液态融合动画。',
    app: (u) => tile(u, ['#00b4c6', '#2f6bff'],
      `<defs>${goo(u + 'g', 28)}</defs><g filter="url(#${u}g)" fill="#fff">
        <rect x="214" y="402" width="400" height="220" rx="110"/><circle cx="716" cy="512" r="84"/><circle cx="850" cy="512" r="40"/></g>`),
    trayW: 22,
    tray: (fg, color) => `<defs>${goo('trayGoo', 0.9, 30, -12)}</defs><g filter="url(#trayGoo)" fill="${fg}">
      <rect x="1.2" y="4.6" width="10.6" height="8.8" rx="4.4"/><circle cx="14.6" cy="9" r="2.9"/></g>
      <circle cx="19.6" cy="9" r="1.5" fill="${color ? P.y : fg}"/>`,
  },
  {
    id: 'asterisk', name: '彩色星号', tag: 'Markdown', batch: 2,
    desc: 'Markdown 的 * 号拆成六瓣标签色，中心留白，像一朵花或一个火花。',
    app: (u) => tile(u, ['#ffffff', '#ececf1'],
      [P.r, P.o, P.y, P.g, P.b, P.p].map((c, i) => {
        const a = -90 + i * 60, [x1, y1] = polar(512, 512, 92, a), [x2, y2] = polar(512, 512, 262, a);
        return line(x1.toFixed(1), y1.toFixed(1), x2.toFixed(1), y2.toFixed(1), c, 100);
      }).join(''), { light: true, gloss: 0.3 }),
    tray: (fg, color) => [P.r, P.o, P.y, P.g, P.b, P.p].map((c, i) => {
      const a = -90 + i * 60, [x1, y1] = polar(9, 9, 2.4, a), [x2, y2] = polar(9, 9, 7.4, a);
      return line(x1.toFixed(2), y1.toFixed(2), x2.toFixed(2), y2.toFixed(2), color ? c : fg, 2.2);
    }).join(''),
  },
  {
    id: 'sticky-wall', name: '便利贴墙', tag: '具象', batch: 2,
    desc: '四张彩色便利贴贴成一面墙，每张是一个标签页。软糯的马卡龙色。',
    app: (u) => tile(u, ['#f6f2ea', '#e3ddd0'],
      `<defs>${shadow(u + 'ws', 10, 12, '#5a4a2a', 0.22)}</defs>` +
      [[392, 392, -5, '#ffe066'], [632, 392, 4, '#ff9ec7'], [392, 632, 3, '#8fd3ff'], [632, 632, -3, '#a8e06c']]
        .map(([cx, cy, r, c]) => `<g transform="rotate(${r} ${cx} ${cy})"><rect x="${cx - 112}" y="${cy - 112}" width="224" height="224" rx="26" fill="${c}" filter="url(#${u}ws)"/>` +
          (c === '#ffe066' ? bar(cx - 70, cy - 50, 140, 26, '#1d1d1f', 0.75) + bar(cx - 70, cy + 4, 110, 22, '#000', 0.18) + bar(cx - 70, cy + 46, 130, 22, '#000', 0.18) : '') + '</g>').join(''),
      { light: true, gloss: 0.25 }),
    tray: (fg, color) => [[1.8, 1.8, P.y], [9.6, 1.8, P.pk], [1.8, 9.6, P.sky], [9.6, 9.6, P.g]]
      .map(([x, y, c], i) => color
        ? `<rect x="${x}" y="${y}" width="6.6" height="6.6" rx="1.6" fill="${c}"/>`
        : `<rect x="${x + 0.7}" y="${y + 0.7}" width="5.2" height="5.2" rx="1.3" fill="${i === 0 ? fg : 'none'}" stroke="${fg}" stroke-width="1.4"/>`).join(''),
  },
  {
    id: 'glass-island', name: '玻璃胶囊', tag: '界面写实', batch: 2,
    desc: '深色底板上的毛玻璃灵动岛，里面是红色当前标签与几颗彩点，背后透出彩色光晕。',
    app: (u) => tile(u, ['#34343a', '#111114'], `
      <defs><filter id="${u}bl" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="70"/></filter></defs>
      <g filter="url(#${u}bl)" opacity=".85"><circle cx="320" cy="300" r="190" fill="${P.p}"/><circle cx="740" cy="760" r="210" fill="${P.b}"/><circle cx="780" cy="280" r="130" fill="${P.pk}"/></g>
      <rect x="208" y="416" width="608" height="192" rx="96" fill="#fff" fill-opacity=".16" stroke="#fff" stroke-opacity=".38" stroke-width="5"/>
      ${bar(244, 452, 228, 120, P.r)}${bar(290, 500, 120, 24, '#fff', 0.9)}
      ${[P.o, P.y, P.g].map((c, i) => `<circle cx="${550 + i * 82}" cy="512" r="30" fill="${c}"/>`).join('')}`, { gloss: 0.08 }),
    trayW: 22,
    tray: (fg, color) => `<rect x="1" y="4.4" width="20" height="9.2" rx="4.6" fill="none" stroke="${fg}" stroke-width="1.3"/>
      ${bar(3, 6.4, 7.4, 5.2, color ? P.r : fg)}<circle cx="13.6" cy="9" r="1.3" fill="${color ? P.o : fg}"/><circle cx="17.4" cy="9" r="1.3" fill="${color ? P.y : fg}"/>`,
  },
  {
    id: 'seal-z', name: '圆章 Z', tag: '字标', batch: 2,
    desc: '米色纸上盖一枚朱红圆章，章里是白色 Z。比「雑」印章更易缩小，不依赖字体。',
    app: (u) => tile(u, ['#faf6ee', '#ebe3d2'], `
      <defs>${shadow(u + 'ss', 6, 8, '#7a1010', 0.25)}</defs>
      <g transform="rotate(-8 512 512)"><circle cx="512" cy="512" r="262" fill="${P.r}" filter="url(#${u}ss)"/>
      <circle cx="512" cy="512" r="224" fill="none" stroke="#fff" stroke-opacity=".9" stroke-width="14"/>
      <path d="M402 404H622L402 620H622" fill="none" stroke="#fff" stroke-width="64" stroke-linecap="round" stroke-linejoin="round"/></g>`,
      { light: true, gloss: 0.3 }),
    tray: (fg, color) => color
      ? `<circle cx="9" cy="9" r="8" fill="${P.r}"/><path d="M6.2 6H11.8L6.2 12H11.8" fill="none" stroke="#fff" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>`
      : `<circle cx="9" cy="9" r="7.3" fill="none" stroke="${fg}" stroke-width="1.5"/><path d="M6.4 6.2H11.6L6.4 11.8H11.6" fill="none" stroke="${fg}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>`,
  },
  {
    id: 'ink-drop', name: '墨水滴', tag: '极简', batch: 2,
    desc: '深夜蓝底上一滴蓝墨水，带一点高光。代表「写」，气质安静。',
    app: (u) => tile(u, ['#1d2c52', '#0a0f1f'], `
      <defs><linearGradient id="${u}ink" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#7fd4ff"/><stop offset="1" stop-color="${P.b}"/></linearGradient></defs>
      <path d="M512 214C512 214 300 452 300 606A212 212 0 0 0 724 606C724 452 512 214 512 214Z" fill="url(#${u}ink)"/>
      <ellipse cx="432" cy="610" rx="30" ry="64" fill="#fff" fill-opacity=".5" transform="rotate(18 432 610)"/>`, { gloss: 0.08 }),
    tray: (fg, color) => `<path d="M9 1.8C9 1.8 3.7 7.8 3.7 11.4A5.3 5.3 0 0 0 14.3 11.4C14.3 7.8 9 1.8 9 1.8Z" fill="${color ? P.b : 'none'}" stroke="${color ? P.b : fg}" stroke-width="1.5" stroke-linejoin="round"/>
      ${color ? '' : `<path d="M6.4 11.2A2.6 2.6 0 0 0 8.4 13.8" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linecap="round"/>`}`,
  },
  {
    id: 'aurora-orb', name: '极光球', tag: '抽象', batch: 2,
    desc: '标签色在球体里晕染流动，类似新版 macOS 的光感风格，最「现代」的一款。',
    app: (u) => tile(u, ['#ffffff', '#eceef3'], `
      <defs><clipPath id="${u}oc"><circle cx="512" cy="512" r="262"/></clipPath>
        <filter id="${u}ob" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="60"/></filter>
        <radialGradient id="${u}hl" cx=".35" cy=".28" r=".5"><stop offset="0" stop-color="#fff" stop-opacity=".75"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>
        ${shadow(u + 'os', 18, 24, '#3a2a8a', 0.28)}</defs>
      <circle cx="512" cy="512" r="262" fill="${P.in}" filter="url(#${u}os)"/>
      <g clip-path="url(#${u}oc)"><g filter="url(#${u}ob)">
        <circle cx="380" cy="380" r="170" fill="${P.pk}"/><circle cx="660" cy="360" r="160" fill="${P.o}"/><circle cx="640" cy="660" r="190" fill="${P.sky}"/><circle cx="380" cy="660" r="150" fill="${P.p}"/><circle cx="520" cy="520" r="90" fill="${P.y}"/></g>
        <circle cx="512" cy="512" r="262" fill="url(#${u}hl)"/></g>`, { light: true, gloss: 0.3 }),
    tray: (fg) => `<path fill-rule="evenodd" fill="${fg}" d="M9 2A7 7 0 1 1 9 16A7 7 0 1 1 9 2ZM11.6 4.6A1.9 1.9 0 1 0 11.6 8.4A1.9 1.9 0 1 0 11.6 4.6Z"/>`,
  },
  {
    id: 'spiral-pad', name: '线圈本', tag: '具象', batch: 2,
    desc: '一本带线圈和横线的速记本，红色页边线，绿色底板，经典又好认。',
    app: (u) => tile(u, ['#7ed957', '#2a9d4b'], `
      <defs>${shadow(u + 'ps', 14, 18, '#0b3d1a', 0.3)}</defs>
      <rect x="296" y="256" width="432" height="540" rx="48" fill="#fff" filter="url(#${u}ps)"/>
      ${[0, 1, 2, 3, 4].map((i) => bar(296, 404 + i * 72, 432, 6, '#bcd6f5')).join('')}
      <rect x="366" y="300" width="6" height="496" fill="#f6a0a6"/>
      ${bar(396, 376, 220, 30, '#1d1d1f', 0.82)}${bar(396, 452, 280, 22, '#000', 0.16)}${bar(396, 524, 240, 22, '#000', 0.16)}
      ${[0, 1, 2, 3, 4, 5].map((i) => `<circle cx="${356 + i * 62}" cy="300" r="13" fill="#2a9d4b" fill-opacity=".35"/>${bar(345 + i * 62, 214, 22, 100, '#3a3a3c')}`).join('')}`),
    tray: (fg) => `<rect x="3.5" y="3.6" width="11" height="12.6" rx="2" fill="none" stroke="${fg}" stroke-width="1.5"/>
      ${line(6.4, 1.6, 6.4, 5.2, fg, 1.4)}${line(9, 1.6, 9, 5.2, fg, 1.4)}${line(11.6, 1.6, 11.6, 5.2, fg, 1.4)}
      ${line(6.2, 9.4, 11.8, 9.4, fg, 1.3)}${line(6.2, 12.4, 10.4, 12.4, fg, 1.3)}`,
  },
);
