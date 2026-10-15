// 第六批：贴合功能。每款都对应 README 里的一项真实功能（feature 字段会显示在卡片上）。
// 依赖 icons.js / icons2.js / icons5.js 的工具函数。

const INK = '#1d1d1f', MUTE = '#c7c7cc';
// Lucide 图标原样路径（与 internal/app/view_icons.go 一致）。
const LU_PIN = 'M9 10.76a2 2 0 0 1-1.11 1.79l-1.78.9A2 2 0 0 0 5 15.24V16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-.76a2 2 0 0 0-1.11-1.79l-1.78-.9A2 2 0 0 1 15 10.76V6h1a2 2 0 0 0 0-4H8a2 2 0 0 0 0 4h1z';
const LU_HISTORY = '<path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/><path d="M3 3v5h5"/><path d="M12 7v5l4 2"/>';
const lucideAt = (x, y, s, inner, stroke, w = 2, fill = 'none') =>
  `<g transform="translate(${x} ${y}) scale(${s})" fill="${fill}" stroke="${stroke}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round">${inner}</g>`;
const check = (x, y, s, c, w) => `<path d="M${x} ${y}l${s * 0.36} ${s * 0.36}l${s * 0.64} ${-s * 0.72}" fill="none" stroke="${c}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round"/>`;

// 应用窗口缩影：顶栏胶囊 + 彩点 + 置顶图标，正文是标题、待办、段落，底栏是状态点。
function windowMini(u, sk) {
  const frame = sk ? `stroke="${INK}" stroke-width="12"` : 'stroke="#fff" stroke-opacity=".9" stroke-width="4"';
  const body = `
    <rect x="196" y="236" width="632" height="560" rx="60" fill="#fff" fill-opacity="${sk ? 1 : 0.9}" ${frame}/>
    ${bar(236, 272, 200, 64, P.r)}${bar(270, 296, 110, 16, '#fff', 0.92)}
    ${[P.o, P.y, P.g, P.b].map((c, i) => `<circle cx="${478 + i * 46}" cy="304" r="17" fill="${c}"/>`).join('')}<circle cx="662" cy="304" r="17" fill="#d1d1d6"/>
    ${lucideAt(750, 284, 1.6, `<path d="${LU_PIN}"/><path d="M12 17v5"/>`, '#8e8e93', 2.4)}
    ${bar(240, 384, 250, 42, INK)}
    <rect x="240" y="456" width="46" height="46" rx="12" fill="${P.r}"/>${check(251, 481, 26, '#fff', 8)}${bar(306, 468, 230, 22, MUTE)}
    <path d="M300 479H548" stroke="#8e8e93" stroke-width="5" stroke-linecap="round"/>
    <rect x="243" y="527" width="40" height="40" rx="10" fill="none" stroke="${MUTE}" stroke-width="6"/>${bar(306, 536, 290, 22, MUTE)}
    ${bar(240, 600, 430, 22, MUTE)}${bar(240, 644, 330, 22, MUTE)}
    <path d="M196 708H828" stroke="#000" stroke-opacity=".07" stroke-width="4"/>
    <circle cx="258" cy="752" r="10" fill="${P.r}"/>${bar(282, 744, 190, 16, '#aeaeb2')}`;
  if (sk) return tile(u, ['#fffaf0', '#efe4cc'], `<defs>${roughF(u + 'r', 0.018, 10)}</defs>${paper(u, 0.12)}<g filter="url(#${u}r)">${body}</g>`, { light: true, gloss: 0.15 });
  return tile(u, ['#9ec5ff', '#c5a8ff'], `<defs><filter id="${u}bb" ${FULL}><feGaussianBlur stdDeviation="70"/></filter>${shadow(u + 'ws', 20, 26, '#2a2060', 0.3)}</defs>
    <g filter="url(#${u}bb)"><circle cx="250" cy="250" r="200" fill="#ff9ec7"/><circle cx="800" cy="820" r="220" fill="#7fe0ff"/></g><g filter="url(#${u}ws)">${body}</g>`);
}
const windowTray = (fg, color) => `<rect x="1" y="2.2" width="20" height="13.6" rx="3" fill="none" stroke="${fg}" stroke-width="1.4"/>
  ${bar(3.2, 4.4, 6.4, 3.4, color ? P.r : fg)}<circle cx="12.2" cy="6.1" r="1.1" fill="${color ? P.o : fg}"/><circle cx="15.2" cy="6.1" r="1.1" fill="${color ? P.y : fg}"/>
  ${line(3.6, 10, 14.4, 10, fg, 1.3)}${line(3.6, 12.8, 11, 12.8, fg, 1.3)}`;

// 待办便签：勾选回写源文本，三行分别用标签色勾选框。
function todoNote(u, sk) {
  const rows = [[P.r, true, 300], [P.o, true, 250], [P.g, false, 280]].map(([c, done, w], i) => {
    const y = 376 + i * 132;
    return (done
      ? `<rect x="290" y="${y}" width="84" height="84" rx="22" fill="${c}"/>${check(310, y + 44, 44, '#fff', 13)}`
      : `<rect x="295" y="${y + 5}" width="74" height="74" rx="19" fill="none" stroke="${c}" stroke-width="10"/>`) +
      bar(410, y + 26, w, 32, done ? MUTE : INK, done ? 1 : 0.85) + (done ? `<path d="M400 ${y + 42}H${420 + w}" stroke="#8e8e93" stroke-width="8" stroke-linecap="round"/>` : '');
  }).join('');
  const note = `<rect x="246" y="250" width="532" height="560" rx="${sk ? 30 : 56}" fill="#fff" ${sk ? `stroke="${INK}" stroke-width="12"` : ''}/>${bar(290, 296, 180, 34, INK, 0.85)}${rows}`;
  if (sk) return tile(u, ['#fff3c4', '#ffd24a'], `<defs>${roughF(u + 'r', 0.018, 11)}</defs><g filter="url(#${u}r)" transform="rotate(-3 512 530)">${note}</g>`, { gloss: 0.15 });
  return tile(u, ['#ffe27a', '#ffb81c'], `<defs>${shadow(u + 'ns', 16, 22, '#8a5a00', 0.3)}</defs><g filter="url(#${u}ns)">${note}</g>`);
}
const todoTray = (fg, color) => `<rect x="1.8" y="2.4" width="5" height="5" rx="1.3" fill="${color ? P.r : fg}"/>
  ${check(2.8, 5, 3, color ? '#fff' : (fg === '#000' ? '#fff' : '#000'), 1.1)}${line(9.4, 4.9, 16, 4.9, fg, 1.4, 'stroke-opacity=".55"')}
  <rect x="2.4" y="11" width="3.8" height="3.8" rx="1" fill="none" stroke="${color ? P.g : fg}" stroke-width="1.3"/>${line(9.4, 12.9, 14.6, 12.9, fg, 1.4)}`;

// 菜单栏呼出：托盘图标高亮，下方吊出一个便签窗口。
function trayDrop(u, sk) {
  const win = `<path d="M690 286L716 316H664Z" fill="#fff"/>
    <rect x="300" y="312" width="520" height="500" rx="48" fill="#fff" ${sk ? `stroke="${INK}" stroke-width="12"` : ''}/>
    ${bar(340, 350, 170, 56, P.r)}${bar(368, 371, 90, 14, '#fff', 0.92)}${[P.o, P.y, P.g].map((c, i) => `<circle cx="${552 + i * 42}" cy="378" r="15" fill="${c}"/>`).join('')}
    ${bar(344, 462, 230, 36, INK, 0.85)}${bar(344, 530, 400, 22, MUTE)}${bar(344, 578, 360, 22, MUTE)}${bar(344, 626, 400, 22, MUTE)}${bar(344, 674, 240, 22, MUTE)}`;
  const bar0 = `<rect x="0" y="0" width="1024" height="236" fill="#fff" fill-opacity="${sk ? 0 : 0.55}"/>
    ${sk ? `<path d="M90 236H934" stroke="${INK}" stroke-width="10"/>` : ''}
    <rect x="636" y="152" width="108" height="72" rx="16" fill="${sk ? '#ffe27a' : '#000'}" fill-opacity="${sk ? 1 : 0.14}"/>
    <circle cx="690" cy="188" r="22" fill="none" stroke="${INK}" stroke-width="9"/>
    ${bar(784, 178, 70, 20, INK, 0.55)}${bar(520, 178, 70, 20, INK, 0.35)}${bar(160, 176, 120, 24, INK, 0.75)}${bar(310, 178, 80, 20, INK, 0.35)}`;
  if (sk) return tile(u, ['#fffaf0', '#efe4cc'], `<defs>${roughF(u + 'r', 0.018, 10)}</defs>${paper(u, 0.12)}<g filter="url(#${u}r)">${bar0}${win}</g>`, { light: true, gloss: 0.15 });
  return tile(u, ['#4f9dff', '#1c4fc4'], `<defs>${shadow(u + 'ds', 18, 22, '#001a4a', 0.35)}</defs>${bar0}<g filter="url(#${u}ds)">${win}</g>`);
}
const trayDropTray = (fg, color) => `${line(1, 2, 17, 2, fg, 1.4, 'stroke-opacity=".5"')}<circle cx="12.4" cy="2" r="1.6" fill="${color ? P.r : fg}"/>
  <rect x="4" y="5.4" width="12" height="10.6" rx="2" fill="none" stroke="${fg}" stroke-width="1.4"/>${line(6.6, 9, 12.6, 9, fg, 1.3)}${line(6.6, 12, 10.8, 12, fg, 1.3)}`;
