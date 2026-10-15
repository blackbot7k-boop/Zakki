// 第三批：同一品牌元素（便签 / 彩点 / Z）用不同视觉风格呈现。依赖 icons.js、icons2.js 的工具函数。

// 像素图：map 每个字符对应 colors 里的颜色，'.' 为透明。
function pix(map, colors, ox, oy, s) {
  let out = '';
  map.forEach((row, y) => [...row].forEach((ch, x) => {
    if (colors[ch]) out += `<rect x="${ox + x * s}" y="${oy + y * s}" width="${s}" height="${s}" fill="${colors[ch]}"/>`;
  }));
  return `<g shape-rendering="crispEdges">${out}</g>`;
}
const PIX_NOTE = [
  '................',
  '..##########....',
  '..#wwwwwwww##...',
  '..#wrrwwwww#f#..',
  '..#wrrwwwww####.',
  '..#wwwwwwwwwww#.',
  '..#wkkkkkkkwww#.',
  '..#wwwwwwwwwww#.',
  '..#wggggggggww#.',
  '..#wwwwwwwwwww#.',
  '..#wggggggwwww#.',
  '..#wwwwwwwwwww#.',
  '..#wgggggggwww#.',
  '..#wwwwwwwwwww#.',
  '..#############.',
  '................',
];
const PIX_TRAY = ['#######..', '#.....##.', '#.rr...##', '#.rr....#', '#.......#', '#.#####.#', '#.......#', '#.####..#', '#########'];

// 黏土球：径向渐变 + 投影，模拟软糯的 3D 体积。
const clayBall = (id, cx, cy, r, light, dark) => `<radialGradient id="${id}" cx=".36" cy=".3" r=".78"><stop offset="0" stop-color="${light}"/><stop offset="1" stop-color="${dark}"/></radialGradient>
  <circle cx="${cx}" cy="${cy}" r="${r}" fill="url(#${id})"/>`;

// 霓虹：模糊光晕 + 白色灯芯。
const neon = (u, d, color, w) => `<path d="${d}" fill="none" stroke="${color}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round" filter="url(#${u}glow)"/>
  <path d="${d}" fill="none" stroke="#fff" stroke-opacity=".85" stroke-width="${w * 0.28}" stroke-linecap="round" stroke-linejoin="round"/>`;

OPTIONS.push(
  {
    id: 'style-liquid-glass', name: '液态玻璃', tag: 'macOS 26 风格', batch: 3,
    desc: '半透明玻璃便签浮在彩色光晕上，边缘有高光描边，贴近 macOS Tahoe 的 Liquid Glass 语言。',
    app: (u) => tile(u, ['#5f8bff', '#b07cff'], `
      <defs><filter id="${u}gb" x="-50%" y="-50%" width="200%" height="200%"><feGaussianBlur stdDeviation="50"/></filter>
        <linearGradient id="${u}ge" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="#fff" stop-opacity=".95"/><stop offset=".5" stop-color="#fff" stop-opacity=".12"/><stop offset="1" stop-color="#fff" stop-opacity=".55"/></linearGradient>
        <linearGradient id="${u}gf" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#fff" stop-opacity=".42"/><stop offset="1" stop-color="#fff" stop-opacity=".14"/></linearGradient></defs>
      <g filter="url(#${u}gb)"><circle cx="330" cy="320" r="170" fill="#ff6fa8"/><circle cx="720" cy="740" r="190" fill="#3fd8ff"/></g>
      <rect x="262" y="232" width="500" height="560" rx="110" fill="url(#${u}gf)" stroke="url(#${u}ge)" stroke-width="9"/>
      ${bar(336, 336, 230, 46, '#fff', 0.95)}${bar(336, 430, 352, 30, '#fff', 0.55)}${bar(336, 490, 296, 30, '#fff', 0.55)}${bar(336, 550, 330, 30, '#fff', 0.55)}
      ${[P.r, P.y, P.g].map((c, i) => `<circle cx="${366 + i * 84}" cy="684" r="28" fill="${c}"/><circle cx="${358 + i * 84}" cy="674" r="9" fill="#fff" fill-opacity=".6"/>`).join('')}`, { gloss: 0.2 }),
    tray: (fg, color) => `<rect x="2.6" y="1.8" width="12.8" height="14.4" rx="3.6" fill="none" stroke="${fg}" stroke-width="1.5"/>
      ${line(5.6, 5.6, 10, 5.6, fg, 1.5)}${line(5.6, 8.6, 12.4, 8.6, fg, 1.3, 'stroke-opacity=".6"')}
      ${[P.r, P.y, P.g].map((c, i) => `<circle cx="${6 + i * 3}" cy="12.6" r="1.1" fill="${color ? c : fg}"/>`).join('')}`,
  },
  {
    id: 'style-leather', name: '皮革手账', tag: '拟物', batch: 3,
    desc: '棕色皮革封面、缝线、松紧绑带和压印的 Z，加一根红色书签带。iOS 6 时代的拟物味道。',
    app: (u) => tile(u, ['#9a6542', '#4a2915'], `
      <defs><filter id="${u}tx" x="0" y="0" width="100%" height="100%"><feTurbulence type="fractalNoise" baseFrequency=".85" numOctaves="2" seed="4"/>
        <feColorMatrix type="saturate" values="0"/><feComponentTransfer><feFuncA type="linear" slope=".22"/></feComponentTransfer></filter></defs>
      <rect width="1024" height="1024" filter="url(#${u}tx)"/>
      <path d="${squircle(512, 512, 352)}" fill="none" stroke="#f0d2a8" stroke-opacity=".75" stroke-width="7" stroke-dasharray="20 14"/>
      <path d="M302 856H370V960L336 930L302 960Z" fill="${P.r}"/>
      <path d="M326 382H566L326 642H566" fill="none" stroke="#c48f68" stroke-width="66" stroke-linecap="round" stroke-linejoin="round" transform="translate(0 5)"/>
      <path d="M326 382H566L326 642H566" fill="none" stroke="#3a1d0c" stroke-width="66" stroke-linecap="round" stroke-linejoin="round"/>
      <rect x="664" y="-10" width="56" height="1044" fill="#24120a"/><rect x="672" y="-10" width="6" height="1044" fill="#fff" fill-opacity=".12"/>`, { gloss: 0.12 }),
    tray: (fg) => `<rect x="3" y="1.8" width="12" height="14.4" rx="2" fill="none" stroke="${fg}" stroke-width="1.5"/>
      ${line(11.6, 1.8, 11.6, 16.2, fg, 1.5)}<path d="M5.6 6H9L5.6 12H9" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>`,
  },
  {
    id: 'style-clay', name: '黏土 3D', tag: '黏土风', batch: 3,
    desc: '软糯的黏土便签，顶上压着三颗彩色黏土球，薰衣草底色，可爱路线。',
    app: (u) => tile(u, ['#cdbcff', '#9a86ff'], `
      <defs>${shadow(u + 'cs', 22, 26, '#3a2a8a', 0.35)}
        <radialGradient id="${u}cn" cx=".35" cy=".25" r=".9"><stop offset="0" stop-color="#ffffff"/><stop offset="1" stop-color="#e2d9ff"/></radialGradient></defs>
      <g filter="url(#${u}cs)">
        <rect x="252" y="330" width="520" height="450" rx="130" fill="url(#${u}cn)"/>
        ${bar(332, 470, 300, 44, '#b8a8f0')}${bar(332, 550, 360, 38, '#d6ccfa')}${bar(332, 618, 250, 38, '#d6ccfa')}
        ${clayBall(u + 'c1', 382, 318, 66, '#ff8a96', '#c81d34')}${clayBall(u + 'c2', 512, 300, 66, '#ffe47a', '#f0a800')}${clayBall(u + 'c3', 642, 318, 66, '#a8ee6a', '#3f9a00')}
      </g>`, { gloss: 0.18 }),
    tray: (fg, color) => `<rect x="2.4" y="6" width="13.2" height="10.2" rx="3.6" fill="none" stroke="${fg}" stroke-width="1.5"/>
      ${line(5.4, 10.4, 10.6, 10.4, fg, 1.4)}${[P.r, P.y, P.g].map((c, i) => `<circle cx="${5.4 + i * 3.6}" cy="4.6" r="1.9" fill="${color ? c : fg}"/>`).join('')}`,
  },
  {
    id: 'style-neumorph', name: '新拟态', tag: 'Neumorphism', batch: 3,
    desc: '浅灰底板上凸起的圆盘、凹陷的内圈和一颗红点，靠柔和的双向阴影表现体积，非常克制。',
    app: (u) => tile(u, ['#e9ecf2', '#dde2ea'], `
      <defs><filter id="${u}nm" x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="26" dy="26" stdDeviation="30" flood-color="#9eacc2" flood-opacity=".75" result="a"/>
          <feDropShadow in="SourceGraphic" dx="-26" dy="-26" stdDeviation="30" flood-color="#fff" flood-opacity="1" result="b"/>
          <feMerge><feMergeNode in="a"/><feMergeNode in="b"/></feMerge></filter>
        ${lin(u + 'in', '#d2d8e2', '#f6f8fb')}</defs>
      <circle cx="512" cy="512" r="250" fill="#e6e9ef" filter="url(#${u}nm)"/>
      <circle cx="512" cy="512" r="150" fill="url(#${u}in)"/>
      <circle cx="512" cy="512" r="52" fill="${P.r}"/><circle cx="496" cy="494" r="16" fill="#fff" fill-opacity=".45"/>`, { light: true, gloss: 0 }),
    tray: (fg, color) => `<circle cx="9" cy="9" r="7.2" fill="none" stroke="${fg}" stroke-width="1.4" stroke-opacity=".55"/>
      <circle cx="9" cy="9" r="4.4" fill="none" stroke="${fg}" stroke-width="1.4"/><circle cx="9" cy="9" r="1.9" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'style-pixel', name: '像素便签', tag: '8-bit 像素', batch: 3,
    desc: '16×16 像素画的折角便签，红色方块是当前标签。菜单栏图标也是对齐像素网格的 9×9 点阵。',
    app: (u) => tile(u, ['#3f7dff', '#2a4fd6'],
      pix(PIX_NOTE, { '#': '#1d1d1f', w: '#ffffff', f: '#d6d6dc', r: P.r, k: '#1d1d1f', g: '#a1a1a6' }, 192, 192, 40), { gloss: 0 }),
    tray: (fg, color) => pix(PIX_TRAY, { '#': fg, r: color ? P.r : fg }, 0, 0, 2),
  },
  {
    id: 'style-neon', name: '霓虹灯牌', tag: '赛博霓虹', batch: 3,
    desc: '深紫夜色里一块霓虹灯牌：青色边框、粉色 Z、黄色句点，带光晕和灯芯。',
    app: (u) => tile(u, ['#1d0d33', '#06030d'], `
      <defs><filter id="${u}glow" x="-30%" y="-30%" width="160%" height="160%"><feGaussianBlur stdDeviation="16" result="b"/>
        <feMerge><feMergeNode in="b"/><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>
      ${neon(u, 'M362 232H662Q792 232 792 362V662Q792 792 662 792H362Q232 792 232 662V362Q232 232 362 232Z', '#29e7ff', 24)}
      ${neon(u, 'M376 392H636L376 632H636', '#ff4fd8', 42)}
      <circle cx="704" cy="632" r="23" fill="#fff36b" filter="url(#${u}glow)"/><circle cx="704" cy="632" r="9" fill="#fff" fill-opacity=".85"/>`, { gloss: 0.04 }),
    tray: (fg, color) => `<rect x="1.8" y="1.8" width="14.4" height="14.4" rx="3.8" fill="none" stroke="${color ? '#29e7ff' : fg}" stroke-width="1.3"/>
      <path d="M5.4 5.8H11.2L5.4 12.2H11.2" fill="none" stroke="${color ? '#ff4fd8' : fg}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>
      <circle cx="13.2" cy="12.2" r="1.1" fill="${color ? '#fff36b' : fg}"/>`,
  },
  {
    id: 'style-bauhaus', name: '包豪斯', tag: '几何构成', batch: 3,
    desc: '黑色横条、红色斜块拼成 Z，再配一个黄圆、一个蓝方，米色底，经典三原色构成。',
    app: (u) => tile(u, ['#f5eedc', '#eadfc4'], `
      <rect x="262" y="262" width="500" height="92" fill="#1d1d1f"/><rect x="262" y="670" width="500" height="92" fill="#1d1d1f"/>
      <path d="M652 354H762L372 670H262Z" fill="#d94032"/>
      <circle cx="680" cy="548" r="76" fill="#f2b705"/><rect x="282" y="414" width="96" height="96" fill="#1f5aa6"/>`, { light: true, gloss: 0 }),
    tray: (fg, color) => `<rect x="2.6" y="2.8" width="12.8" height="2.6" fill="${fg}"/><rect x="2.6" y="12.6" width="12.8" height="2.6" fill="${fg}"/>
      <path d="M12.4 5.4H15.4L5.6 12.6H2.6Z" fill="${color ? '#d94032' : fg}"/><circle cx="13.4" cy="9.4" r="1.6" fill="${color ? '#f2b705' : fg}"/>`,
  },
);
