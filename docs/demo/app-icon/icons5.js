// 第四批：手绘系列。依赖 icons.js / icons2.js 的工具函数。

const FULL = 'filterUnits="userSpaceOnUse" x="0" y="0" width="1024" height="1024"';
// 手抖：低频噪声位移，让直线带毛边。
const roughF = (id, freq, scale, seed = 3, box = FULL) => `<filter id="${id}" ${box}>
  <feTurbulence type="fractalNoise" baseFrequency="${freq}" numOctaves="2" seed="${seed}" result="n"/>
  <feDisplacementMap in="SourceGraphic" in2="n" scale="${scale}" xChannelSelector="R" yChannelSelector="G"/></filter>`;
// 手抖 + 颗粒镂空：蜡笔、粉笔、干笔的飞白。
const grainF = (id, freq, scale, grain, seed = 5, bias = -1.15, gx = grain) => `<filter id="${id}" ${FULL}>
  <feTurbulence type="fractalNoise" baseFrequency="${freq}" numOctaves="2" seed="${seed}" result="n"/>
  <feDisplacementMap in="SourceGraphic" in2="n" scale="${scale}" xChannelSelector="R" yChannelSelector="G" result="d"/>
  <feTurbulence type="fractalNoise" baseFrequency="${gx} ${grain}" numOctaves="1" seed="${seed + 1}" result="g"/>
  <feColorMatrix in="g" type="matrix" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 3.4 0 0 0 ${bias}" result="m"/>
  <feComposite in="d" in2="m" operator="in"/></filter>`;
// 水彩：大幅位移 + 轻微模糊，边缘晕开。
const washF = (id, seed) => `<filter id="${id}" ${FULL}>
  <feTurbulence type="fractalNoise" baseFrequency=".012" numOctaves="3" seed="${seed}" result="n"/>
  <feDisplacementMap in="SourceGraphic" in2="n" scale="64" xChannelSelector="R" yChannelSelector="G" result="d"/><feGaussianBlur in="d" stdDeviation="3"/></filter>`;
// 纸纹叠加层。
const paper = (u, op = 0.12) => `<filter id="${u}pp" ${FULL}><feTurbulence type="fractalNoise" baseFrequency=".9" numOctaves="2" seed="9"/>
  <feColorMatrix type="saturate" values="0"/><feComponentTransfer><feFuncA type="linear" slope="${op}"/></feComponentTransfer></filter>
  <rect width="1024" height="1024" filter="url(#${u}pp)"/>`;
// 菜单栏图标的轻微手抖（固定 id，同参数多处复用无冲突）。
const trayRough = (id) => `<defs><filter id="${id}" x="-20%" y="-20%" width="140%" height="140%">
  <feTurbulence type="fractalNoise" baseFrequency=".28" numOctaves="1" seed="2" result="n"/>
  <feDisplacementMap in="SourceGraphic" in2="n" scale="1.1" xChannelSelector="R" yChannelSelector="G"/></filter></defs>`;

// 彩铅涂色：圆内往返折线 + 一圈轮廓。
function hatchDot(cx, cy, r, c, angDeg, sp = 15, w = 13) {
  const a = angDeg * Math.PI / 180, cs = Math.cos(a), sn = Math.sin(a);
  let d = '';
  for (let k = 0, x = -r * 0.9; x <= r * 0.9; k++, x += sp / 2) {
    const h = Math.sqrt(Math.max(r * r - x * x, 0)) * 0.88, y = k % 2 ? h : -h;
    d += (k ? 'L' : 'M') + (cx + x * cs - y * sn).toFixed(1) + ' ' + (cy + x * sn + y * cs).toFixed(1);
  }
  return `<path d="${d}" fill="none" stroke="${c}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round" stroke-opacity=".92"/>
    <circle cx="${cx}" cy="${cy}" r="${r}" fill="none" stroke="${c}" stroke-width="8"/>`;
}
// 毛笔弧：沿弧放圆点，起笔重、收笔轻。
function brushArc(cx, cy, r, a0, a1, w0, w1, c, n = 160) {
  let out = '';
  for (let i = 0; i <= n; i++) {
    const t = i / n, a = (a0 + (a1 - a0) * t) * Math.PI / 180;
    const w = w0 + (w1 - w0) * Math.pow(t, 1.4) + w0 * 0.07 * Math.sin(t * 11);
    out += `<circle cx="${(cx + r * Math.cos(a)).toFixed(2)}" cy="${(cy + r * Math.sin(a)).toFixed(2)}" r="${(w / 2).toFixed(2)}"/>`;
  }
  return `<g fill="${c}">${out}</g>`;
}
const wave = (x, y, n, amp = 22, step = 34) => `M${x} ${y}q${step / 2} ${-amp} ${step} 0` + `t${step} 0`.repeat(n);

OPTIONS.push(
  {
    id: 'hd-marker', name: '马克笔错位', tag: '马克笔', batch: 4,
    desc: '黑色马克笔勾出 Z 和句点，黄、红色块故意错位，像孔版印刷（Riso）的套色偏移。',
    app: (u) => tile(u, ['#fff8ec', '#f3e6cc'], `<defs>${roughF(u + 'r', 0.016, 16)}</defs>${paper(u)}
      <g transform="translate(26 22)"><path d="M320 340H690L332 688H704" fill="none" stroke="${P.y}" stroke-width="150" stroke-linecap="round" stroke-linejoin="round"/>
        <circle cx="792" cy="688" r="62" fill="${P.r}"/></g>
      <g filter="url(#${u}r)" fill="none" stroke="#151515" stroke-linecap="round" stroke-linejoin="round">
        <path d="M320 340H690L332 688H704" stroke-width="44"/><circle cx="792" cy="688" r="58" stroke-width="22"/>
        <path d="M770 300L820 250M790 340L858 330M736 280L742 210" stroke-width="20"/></g>`, { light: true, gloss: 0.15 }),
    tray: (fg, color) => `${trayRough('trMk')}<g filter="url(#trMk)" fill="none" stroke="${fg}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
      <path d="M3 4.6H11.2L3.4 13.2H11.4"/><circle cx="14.8" cy="13" r="1.8" fill="${color ? P.r : 'none'}" stroke-width="1.4"/></g>`,
  },
  {
    id: 'hd-pencil', name: '彩铅七点', tag: '彩铅', batch: 4,
    desc: '七个标签色用彩色铅笔来回涂满，留着笔触和纸面颗粒，像小朋友认真涂的色卡。',
    app: (u) => tile(u, ['#ffffff', '#f1eee7'], `<defs>${grainF(u + 'g', 0.02, 8, 0.6, 5, -0.7)}</defs>${paper(u, 0.1)}
      <g filter="url(#${u}g)">${TOT7.map((c, i) => { const [x, y] = polar(512, 512, 236, -90 + i * 360 / 7); return hatchDot(x, y, 78, c, 30 + i * 23); }).join('')}</g>`,
      { light: true, gloss: 0.2 }),
    tray: (fg, color) => `${trayRough('trPc')}<g filter="url(#trPc)">` +
      TOT7.map((c, i) => { const [x, y] = polar(9, 9, 6.3, -90 + i * 360 / 7); return `<circle cx="${x}" cy="${y}" r="1.6" fill="${color ? c : fg}"/>`; }).join('') + '</g>',
  },
  {
    id: 'hd-watercolor', name: '水彩淡彩', tag: '水彩', batch: 4,
    desc: '先刷一层晕开的黄色水彩，再用细钢笔勾折角便签，红点也是一滴水彩。插画本的味道。',
    app: (u) => tile(u, ['#fdfbf6', '#eee7d8'], `<defs>${washF(u + 'w1', 3)}${washF(u + 'w2', 8)}${roughF(u + 'r', 0.02, 9)}</defs>${paper(u, 0.14)}
      <g filter="url(#${u}w1)" style="mix-blend-mode:multiply"><rect x="270" y="270" width="470" height="500" rx="30" fill="#ffd54a" fill-opacity=".6"/>
        <rect x="270" y="270" width="470" height="500" rx="30" fill="none" stroke="#f0a800" stroke-opacity=".35" stroke-width="14"/></g>
      <g filter="url(#${u}w2)" style="mix-blend-mode:multiply"><circle cx="420" cy="380" r="62" fill="${P.r}" fill-opacity=".75"/><circle cx="660" cy="700" r="90" fill="#7cc4ff" fill-opacity=".45"/></g>
      <g filter="url(#${u}r)" fill="none" stroke="#2a2a2a" stroke-width="10" stroke-linecap="round" stroke-linejoin="round">
        <path d="M320 250H690Q720 250 720 280V640L610 750H320Q292 750 292 722V280Q292 250 320 250Z"/><path d="M720 640H640Q610 640 610 670V750"/>
        <path d="M512 380H650M360 480H650M360 560H610M360 640H540"/></g>`, { light: true, gloss: 0.15 }),
    tray: (fg, color) => `${trayRough('trWc')}<g filter="url(#trWc)" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round">
      <path d="M4.6 2.4H13.4A1.2 1.2 0 0 1 14.6 3.6V11.4L10.8 15.6H4.6A1.2 1.2 0 0 1 3.4 14.4V3.6A1.2 1.2 0 0 1 4.6 2.4Z"/><path d="M14.6 11.4H12A1.2 1.2 0 0 0 10.8 12.6V15.6"/>
      <path d="M9.2 6.2H12M6.2 9.2H12"/></g><circle cx="6.4" cy="6.2" r="1.5" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'hd-enso', name: '墨笔禅圆', tag: '毛笔', batch: 4,
    desc: '一笔写成的禅圆（円相），起笔浓重、收笔飞白，右下盖一枚朱红 Z 印。最有东方气质的手绘。',
    app: (u) => tile(u, ['#f7f2e7', '#e8dfca'], `<defs>${grainF(u + 'b', 0.012, 18, 0.16, 11, -0.55, 0.012)}${roughF(u + 'sr', 0.04, 6)}</defs>${paper(u, 0.14)}
      <g filter="url(#${u}b)">${brushArc(500, 492, 236, 112, 432, 118, 26, '#1b1b1b')}</g>
      <g filter="url(#${u}sr)" transform="rotate(4 712 736)"><rect x="660" y="684" width="104" height="104" rx="10" fill="#c8322b"/>
        <path d="M688 714H736L690 760H738" fill="none" stroke="#f7f2e7" stroke-width="14" stroke-linecap="round" stroke-linejoin="round"/></g>`, { light: true, gloss: 0.1 }),
    tray: (fg, color) => brushArc(8.4, 8.4, 6, 112, 428, 2.9, 0.8, fg, 70) + `<rect x="13.2" y="13.2" width="3.6" height="3.6" rx=".6" fill="${color ? '#c8322b' : fg}"/>`,
  },
  {
    id: 'hd-chalk', name: '粉笔黑板', tag: '粉笔', batch: 4,
    desc: '墨绿黑板上用粉笔画的待办清单，彩色粉笔打勾，背景留着擦过的白雾。',
    app: (u) => tile(u, ['#36553f', '#1d3126'], `
      <defs>${grainF(u + 'c', 0.02, 8, 0.45, 5, -0.6)}<filter id="${u}sm" ${FULL}><feGaussianBlur stdDeviation="46"/></filter></defs>
      <g filter="url(#${u}sm)" fill="#fff" fill-opacity=".07"><ellipse cx="360" cy="300" rx="240" ry="90"/><ellipse cx="680" cy="760" rx="260" ry="80"/></g>
      <g filter="url(#${u}c)" fill="none" stroke-linecap="round" stroke-linejoin="round">
        ${[352, 512, 672].map((y, i) => `<rect x="262" y="${y - 52}" width="104" height="104" rx="14" stroke="#f2f2ea" stroke-width="24"/>
          <path d="M432 ${y}H${[772, 720, 632][i]}" stroke="#f2f2ea" stroke-width="34" ${i < 2 ? 'stroke-opacity=".5"' : ''}/>`).join('')}
        <path d="M276 346L314 388L392 290" stroke="#ff9aa8" stroke-width="34"/><path d="M276 506L314 548L392 450" stroke="#ffe27a" stroke-width="34"/>
      </g>`, { gloss: 0.06 }),
    tray: (fg, color) => `${trayRough('trCh')}<g filter="url(#trCh)" fill="none" stroke="${fg}" stroke-linecap="round" stroke-linejoin="round" stroke-width="1.4">
      <rect x="2.2" y="2.4" width="4.4" height="4.4" rx=".8"/><rect x="2.2" y="11.2" width="4.4" height="4.4" rx=".8"/>
      <path d="M9 4.6H16M9 13.4H14.4"/><path d="M2.8 4.4L4.2 5.8L7.6 1.8" stroke="${color ? '#ff7a8a' : fg}" stroke-width="1.6"/></g>`,
  },
  {
    id: 'hd-mascot', name: '便签小人', tag: '涂鸦角色', batch: 4,
    desc: '一张会笑的黄色便签，粗黑描边、腮红和小闪光，把应用做成一个角色，最可爱的一款。',
    app: (u) => tile(u, ['#e3f3ff', '#b6dbff'], `<defs>${roughF(u + 'r', 0.014, 14)}</defs>
      <g transform="rotate(-6 512 520)">
        <path d="M300 280H724V650L620 754H300Z" fill="#ffe066"/><path d="M724 650H640Q620 650 620 670V754Z" fill="#f2c230"/>
        <circle cx="404" cy="584" r="34" fill="#ff8fa3" fill-opacity=".6"/><circle cx="620" cy="584" r="34" fill="#ff8fa3" fill-opacity=".6"/>
        <g filter="url(#${u}r)" fill="none" stroke="#151515" stroke-width="18" stroke-linecap="round" stroke-linejoin="round">
          <path d="M300 280H724V650L620 754H300Z"/><path d="M724 650H640Q620 650 620 670V754"/><path d="M462 590Q512 640 562 590"/></g>
        <ellipse cx="440" cy="506" rx="20" ry="30" fill="#151515"/><ellipse cx="584" cy="506" rx="20" ry="30" fill="#151515"/>
      </g>
      <g filter="url(#${u}r)" stroke="#151515" stroke-width="16" stroke-linecap="round"><path d="M770 218L800 176M806 252L856 236M742 196L736 146"/></g>`, { gloss: 0.15 }),
    tray: (fg) => `${trayRough('trMs')}<g filter="url(#trMs)"><path d="M3 2.6H15V12.4L11.8 15.6H3Z" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linejoin="round"/>
      <circle cx="7" cy="7.6" r="1.1" fill="${fg}"/><circle cx="11" cy="7.6" r="1.1" fill="${fg}"/><path d="M7 10.6Q9 12.4 11 10.6" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linecap="round"/></g>`,
  },
  {
    id: 'hd-bujo', name: '荧光手账', tag: '子弹笔记', batch: 4,
    desc: '方格纸上的子弹笔记：蓝色圆珠笔画框和波浪字迹，第一行涂了黄色荧光笔，角落一颗小星星。',
    app: (u) => tile(u, ['#ffffff', '#f3f4f6'], `<defs>${roughF(u + 'r', 0.02, 10)}</defs>
      ${[...Array(16)].map((_, i) => `<path d="M${100 + i * 56} 0V1024M0 ${100 + i * 56}H1024" stroke="#5b8def" stroke-opacity=".16" stroke-width="3"/>`).join('')}
      <path d="M276 372H748" stroke="#ffe14d" stroke-opacity=".85" stroke-width="84" stroke-linecap="round" style="mix-blend-mode:multiply"/>
      <g filter="url(#${u}r)" fill="none" stroke="#1f3a8a" stroke-width="12" stroke-linecap="round" stroke-linejoin="round">
        ${[372, 512, 652].map((y, i) => `<rect x="290" y="${y - 30}" width="60" height="60" rx="6"/><path d="${wave(392, y + 6, [9, 7, 8][i])}"/>`).join('')}
        <path d="M298 372L318 394L360 340M298 512L318 534L360 480" stroke="${P.r}" stroke-width="16"/>
        <path d="M744 194L760 236L804 238L770 264L782 306L744 282L706 306L718 264L684 238L728 236Z" stroke="${P.o}"/></g>`, { light: true, gloss: 0.15 }),
    tray: (fg, color) => `${trayRough('trBj')}<rect x="1" y="2.4" width="16" height="5.6" rx="2.8" fill="${color ? '#ffd60a' : fg}" fill-opacity="${color ? 0.9 : 0.35}"/>
      <g filter="url(#trBj)" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round">
      <path d="M2.6 5.2L4.2 6.8L7 3.2" stroke="${color ? P.r : fg}" stroke-width="1.6"/><path d="M9 5.2H15.4"/>
      <rect x="2.6" y="11" width="4" height="4" rx=".6"/><path d="M9 13H14"/></g>`,
  },
);
