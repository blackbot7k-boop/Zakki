// 第五批：手绘系列 · 续。依赖 icons.js / icons2.js / icons5.js 的工具函数。

// 蜡笔来回涂的锯齿线。
const zig = (x, y, n, amp, step) => `M${x} ${y + amp}` + [...Array(n)].map((_, i) => `l${step} ${i % 2 ? amp * 2 : -amp * 2}`).join('');
// 斜线排线图案。
const hatchPat = (id, gap, w, angle, color) => `<pattern id="${id}" width="${gap}" height="${gap}" patternUnits="userSpaceOnUse" patternTransform="rotate(${angle})">
  <rect width="${w}" height="${gap}" fill="${color}"/></pattern>`;

OPTIONS.push(
  {
    id: 'hd-crayon', name: '牛皮纸蜡笔', tag: '蜡笔', batch: 5,
    desc: '牛皮纸上用白蜡笔画便签框，彩色蜡笔来回涂成几行字，左上一个涂满的红点，蜡质颗粒很明显。',
    app: (u) => tile(u, ['#d6b88f', '#b48a5a'], `<defs>${grainF(u + 'k', 0.025, 12, 0.85, 4, -0.45)}</defs>${paper(u, 0.18)}
      <g filter="url(#${u}k)" fill="none" stroke-linecap="round" stroke-linejoin="round" transform="rotate(-4 512 512)">
        <path d="M290 276L734 262L748 752L282 766Z" stroke="#fffaf0" stroke-width="34"/>
        ${hatchDot(384, 378, 52, P.r, 40, 13, 16)}
        <path d="${zig(470, 368, 12, 16, 20)}" stroke="#1d1d1f" stroke-width="20"/>
        <path d="${zig(356, 500, 16, 14, 20)}" stroke="${P.y}" stroke-width="22"/>
        <path d="${zig(356, 590, 14, 14, 20)}" stroke="#5fb4ff" stroke-width="22"/>
        <path d="${zig(356, 680, 10, 14, 20)}" stroke="#8fd14f" stroke-width="22"/>
      </g>`, { gloss: 0.1 }),
    tray: (fg, color) => `${trayRough('trCr')}<g filter="url(#trCr)" fill="none" stroke="${fg}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
      <path d="M2.6 2.6L15.4 2.2L15.8 15.6L2.4 15.8Z"/><path d="${zig(5, 6.6, 4, 1.1, 2)}" stroke-width="1.3"/><path d="M5 11.6H9.4"/></g>
      <circle cx="12.4" cy="11.6" r="1.6" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'hd-hatch', name: '针管笔排线', tag: '针管笔', batch: 5,
    desc: '0.3 针管笔画的线稿便签，阴影用斜线和交叉排线表现，像建筑速写或铜版画，只留一颗红点。',
    app: (u) => tile(u, ['#ffffff', '#f2f2f2'], `<defs>${roughF(u + 'r', 0.02, 7)}
        ${hatchPat(u + 'h1', 22, 5, 45, '#1d1d1f')}${hatchPat(u + 'h2', 22, 5, -45, '#1d1d1f')}</defs>
      <path d="M352 330H782V700L682 800H352Z" fill="url(#${u}h1)" opacity=".7"/>
      <path d="M682 700H782L682 800Z" fill="url(#${u}h2)" opacity=".8"/>
      <g filter="url(#${u}r)" stroke="#1d1d1f" stroke-linecap="round" stroke-linejoin="round">
        <path d="M282 250H712V620L612 720H282Z" fill="#fff" stroke-width="9"/>
        <path d="M712 620H632Q612 620 612 640V720Z" fill="url(#${u}h1)" stroke-width="7"/>
        <path d="M418 362H632M340 452H640M340 526H600M340 600H520" fill="none" stroke-width="6"/></g>
      <circle cx="362" cy="362" r="30" fill="${P.r}"/>`, { light: true, gloss: 0.1 }),
    tray: (fg, color) => `<path d="M2.6 2H13V11.2L10.4 13.8H2.6Z" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linejoin="round"/>
      <path d="M15.4 5V15.4H5M14.8 8.4L11.4 11.8M15.2 11.6L12.6 14.2" fill="none" stroke="${fg}" stroke-width="1.1" stroke-linecap="round" stroke-opacity=".7"/>
      <circle cx="5.2" cy="5" r="1.2" fill="${color ? P.r : fg}"/>${line(7.4, 5, 10.6, 5, fg, 1.1)}${line(5, 8, 10.6, 8, fg, 1.1)}`,
  },
  {
    id: 'hd-washi', name: '和纸胶带', tag: '拼贴手账', batch: 5,
    desc: '一张手撕边的便签被两段和纸胶带贴在墙上，粉色条纹、薄荷圆点，便签上画了一颗红笔小爱心。',
    app: (u) => tile(u, ['#fcf1e8', '#f1dccb'], `<defs>${roughF(u + 't', 0.09, 14, 6)}${roughF(u + 'r', 0.02, 8)}${shadow(u + 'ns', 10, 12, '#6a4a3a', 0.22)}
        <pattern id="${u}st" width="28" height="28" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><rect width="28" height="28" fill="#ffd3e4"/><rect width="13" height="28" fill="#ff8fbd"/></pattern>
        <pattern id="${u}dt" width="34" height="34" patternUnits="userSpaceOnUse"><rect width="34" height="34" fill="#c9f1e4"/><circle cx="17" cy="17" r="6" fill="#4cc9a4"/></pattern></defs>
      <g transform="rotate(-4 512 520)"><g filter="url(#${u}ns)"><rect x="290" y="290" width="444" height="480" fill="#fffdf8" filter="url(#${u}t)"/></g>
        <g filter="url(#${u}r)" fill="none" stroke-linecap="round">
          <path d="M346 430H660M346 510H640M346 590H600" stroke="#c9bcb0" stroke-width="12"/>
          <path d="M600 702C540 654 528 604 566 592C586 586 600 600 600 616C600 600 614 586 634 592C672 604 660 654 600 702Z" stroke="${P.r}" stroke-width="12" stroke-linejoin="round"/></g></g>
      <g filter="url(#${u}t)" opacity=".88"><rect x="226" y="248" width="220" height="70" fill="url(#${u}st)" transform="rotate(-32 336 283)"/>
        <rect x="586" y="236" width="220" height="70" fill="url(#${u}dt)" transform="rotate(28 696 271)"/></g>`, { light: true, gloss: 0.15 }),
    tray: (fg, color) => `<rect x="3" y="4.4" width="12" height="12" rx=".6" fill="none" stroke="${fg}" stroke-width="1.4"/>
      <rect x="5.4" y="1.6" width="7.2" height="3.6" rx=".4" fill="${color ? '#ff8fbd' : fg}" fill-opacity="${color ? 1 : 0.6}" transform="rotate(-8 9 3.4)"/>
      ${line(5.6, 9, 12.4, 9, fg, 1.2)}${line(5.6, 12, 10.4, 12, fg, 1.2)}`,
  },
  {
    id: 'hd-plane', name: '纸飞机', tag: '涂鸦', batch: 5,
    desc: '一架手绘纸飞机拖着打圈的虚线飞向右上角，旁边一朵小云：想法写下来，随手发出去。',
    app: (u) => tile(u, ['#eef8ff', '#c6e4ff'], `<defs>${roughF(u + 'r', 0.018, 12)}</defs>
      <g filter="url(#${u}r)" fill="none" stroke-linecap="round" stroke-linejoin="round">
        <path d="M168 846C270 780 380 780 380 690C380 600 268 614 280 690C292 770 430 760 500 610" stroke="#3a6fb0" stroke-width="16" stroke-dasharray="30 26"/>
        <path d="M232 330Q232 280 282 284Q300 240 352 254Q396 236 414 280Q452 290 440 330Z" fill="#fff" stroke="#2b2b2b" stroke-width="12"/>
        <path d="M800 270L296 430L506 486Z" fill="#fff" stroke="#1d1d1f" stroke-width="18"/>
        <path d="M800 270L506 486L566 660Z" fill="#d8e6f3" stroke="#1d1d1f" stroke-width="18"/>
        <path d="M506 486L530 590" stroke="#1d1d1f" stroke-width="14"/></g>`, { light: true, gloss: 0.2 }),
    tray: (fg, color) => `${trayRough('trPl')}<g filter="url(#trPl)" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linejoin="round" stroke-linecap="round">
      <path d="M16.6 2L1.6 7.6L7.6 9.6Z" fill="${color ? '#fff' : 'none'}"/><path d="M16.6 2L7.6 9.6L9.8 16Z" fill="${color ? '#9cc9ff' : 'none'}"/>
      <path d="M5.4 13.4Q3.4 14.4 2 16.6" stroke-dasharray="1.4 1.6"/></g>`,
  },
  {
    id: 'hd-oneline', name: '一笔画', tag: '连续线稿', batch: 5,
    desc: '笔不离纸：一根线画出便签、折角和里面的 Z，再打个圈收尾，尾巴上一颗红点。蜜桃色底，很时髦的线稿风。',
    app: (u) => tile(u, ['#fff3e6', '#ffd2bd'], `<defs>${roughF(u + 'r', 0.012, 8)}</defs>
      <path filter="url(#${u}r)" fill="none" stroke="#2b2b2b" stroke-width="16" stroke-linecap="round" stroke-linejoin="round"
        d="M176 800C240 790 300 780 300 730V290H720V650L620 750V670Q620 650 640 650H720M620 750H360C330 750 330 700 360 640L400 420H624L404 616H628C710 616 764 560 722 520C690 490 650 532 694 568C740 606 790 640 846 720"/>
      <circle cx="856" cy="736" r="28" fill="${P.r}"/>`, { light: true, gloss: 0.2 }),
    tray: (fg, color) => `<path fill="none" stroke="${fg}" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"
      d="M1.2 16.6Q3.4 16 3.4 14.4V2.6H14.6V11.4L11.4 14.6H5.6Q4.8 14.6 5.4 12.4L6.6 5.8H11.8L6.6 11.2H12.6Q15 11.2 14 13.6"/>
      <circle cx="15.6" cy="15.4" r="1.4" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'hd-stamp', name: '橡皮章', tag: '印章', batch: 5,
    desc: '手刻橡皮章盖出来的靛蓝印：便签图形加 ZAKKI 字样，边缘缺墨、颗粒斑驳，旁边还有一个盖淡了的重影。',
    app: (u) => tile(u, ['#f8f4ea', '#ebe2cf'], `<defs>${grainF(u + 's', 0.03, 7, 0.5, 21, -0.55)}</defs>${paper(u, 0.12)}
      ${[[0.07, 'translate(60 48) rotate(9 512 512)'], [1, 'rotate(-6 512 512)']].map(([op, tr]) => `<g transform="${tr}" opacity="${op}" filter="url(#${u}s)">
        <rect x="236" y="236" width="552" height="552" rx="64" fill="none" stroke="#2c4ea0" stroke-width="30"/>
        <rect x="270" y="270" width="484" height="484" rx="40" fill="none" stroke="#2c4ea0" stroke-width="8"/>
        <path d="M372 318H652V538L566 624H372Z" fill="#2c4ea0"/><path d="M652 538H586Q566 538 566 558V624Z" fill="#f8f4ea"/>
        <path d="M424 400H600M424 460H600M424 520H540" stroke="#f8f4ea" stroke-width="22" stroke-linecap="round"/>
        <text x="512" y="712" text-anchor="middle" font-size="92" font-weight="800" letter-spacing="10" fill="#2c4ea0"
          font-family="'Avenir Next','Helvetica Neue',Arial,sans-serif">ZAKKI</text></g>`).join('')}`, { light: true, gloss: 0.12 }),
    tray: (fg, color) => `<rect x="1.4" y="1.4" width="15.2" height="15.2" rx="2.6" fill="none" stroke="${color ? '#2c4ea0' : fg}" stroke-width="1.5"/>
      <path fill-rule="evenodd" fill="${color ? '#2c4ea0' : fg}" d="M4.6 4H13.4V10.8L10.8 13.4H4.6ZM6.4 6.2H11.6V7.6H6.4ZM6.4 9H10V10.4H6.4Z"/>`,
  },
  {
    id: 'hd-embroidery', name: '刺绣贴布', tag: '刺绣', batch: 5,
    desc: '牛仔布上缝了一块黄色毛毡布贴，锁边针脚、虚线缝线，中间是缎面绣的红色 Z，有手作的温度。',
    app: (u) => tile(u, ['#46699a', '#233a60'], `<defs>
        <pattern id="${u}wv" width="10" height="10" patternUnits="userSpaceOnUse"><path d="M0 10L10 0" stroke="#fff" stroke-opacity=".08" stroke-width="3"/></pattern>
        ${hatchPat(u + 'sat', 12, 7, 62, '#e8404f')}${shadow(u + 'ps', 12, 14, '#000', 0.35)}
        <radialGradient id="${u}fl" cx=".4" cy=".35" r=".8"><stop offset="0" stop-color="#ffd54a"/><stop offset="1" stop-color="#f0b400"/></radialGradient></defs>
      <rect width="1024" height="1024" fill="url(#${u}wv)"/>
      <circle cx="512" cy="512" r="282" fill="url(#${u}fl)" filter="url(#${u}ps)"/>
      <circle cx="512" cy="512" r="282" fill="none" stroke="#c98d00" stroke-width="22" stroke-dasharray="3 6"/>
      <circle cx="512" cy="512" r="236" fill="none" stroke="#fff" stroke-width="9" stroke-dasharray="22 16" stroke-linecap="round"/>
      <path d="M404 404H620L404 620H620" fill="none" stroke="#b81d2e" stroke-width="96" stroke-linecap="round" stroke-linejoin="round"/>
      <path d="M404 404H620L404 620H620" fill="none" stroke="url(#${u}sat)" stroke-width="84" stroke-linecap="round" stroke-linejoin="round"/>`, { gloss: 0.08 }),
    tray: (fg, color) => `<circle cx="9" cy="9" r="7.4" fill="none" stroke="${fg}" stroke-width="1.3" stroke-dasharray="1.8 1.4"/>
      <path d="M6 6H12L6 12H12" fill="none" stroke="${color ? P.r : fg}" stroke-width="1.9" stroke-linecap="round" stroke-linejoin="round"/>`,
  },
);
