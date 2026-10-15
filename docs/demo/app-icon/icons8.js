const featurePaper = (u, content, colors = ['#fffaf0', '#eee3cf']) => tile(u, colors,
  `<defs>${roughF(u + 'pen', .018, 9)}</defs>${paper(u, .08)}<g filter="url(#${u}pen)">${content}</g>`,
  { light: true, gloss: .08 });

const featureHash = (x, y, s, c, w) => `<g transform="translate(${x} ${y}) scale(${s})" fill="none" stroke="${c}" stroke-width="${w}" stroke-linecap="round">
  <path d="M5 0L2 18M14 0L11 18M0 6H17M0 12H17"/></g>`;

OPTIONS.push(
  {
    id: 'fn-tab-note', name: '彩签速记', tag: '首推 · 多标签', batch: 6,
    feature: '独立彩色标签 + 当前标签胶囊',
    desc: '把最有辨识度的彩色标签放在便签顶端：一枚红胶囊、三颗彩点、两笔正文。少量元素，保留手绘纸感。',
    app: u => featurePaper(u, `
      <path d="M280 270L744 280L730 756L270 744Z" fill="#fffdfa" stroke="${INK}" stroke-width="15" stroke-linejoin="round"/>
      ${bar(314, 324, 188, 86, P.r)}${bar(354, 358, 102, 18, '#fff')}
      ${[P.o, P.g, P.b].map((c, i) => `<circle cx="${550 + i * 60}" cy="367" r="22" fill="${c}"/>`).join('')}
      ${line(324, 504, 622, 498, INK, 30)}${line(324, 602, 552, 596, '#8a857d', 23)}
      ${line(600, 566, 600, 632, P.r, 12)}`),
    trayW: 22,
    tray: (fg, color) => `<path d="M2 3.4H20V15H2Z" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linejoin="round"/>
      ${bar(4, 5.6, 7, 3.4, color ? P.r : fg)}<circle cx="14" cy="7.3" r="1.2" fill="${color ? P.g : fg}"/><circle cx="17" cy="7.3" r="1.2" fill="${color ? P.b : fg}"/>
      ${line(4.6, 11.8, 13.4, 11.8, fg, 1.4)}`,
  },
  {
    id: 'fn-markdown-note', name: '井号便签', tag: '首推 · Markdown', batch: 6,
    feature: 'Markdown 标题、段落与实时预览',
    desc: '大井号说明 Markdown，折角说明便签，顶部彩点连接 Zakki 的多标签。不会误认成聊天或云同步应用。',
    app: u => featurePaper(u, `
      <path d="M290 250H734V636L612 770H290Z" fill="#fffef9" stroke="${INK}" stroke-width="14" stroke-linejoin="round"/>
      <path d="M734 636H640Q612 636 612 664V770" fill="#ebe4d4" stroke="${INK}" stroke-width="12" stroke-linejoin="round"/>
      ${[P.r, P.o, P.g].map((c, i) => `<circle cx="${346 + i * 54}" cy="312" r="17" fill="${c}"/>`).join('')}
      ${featureHash(380, 400, 12, INK, 2.7)}${line(346, 694, 544, 694, '#9d968a', 18)}`, ['#f9e7b3', '#eed391']),
    tray: (fg, color) => `<path d="M3 1.8H15V11.8L11.2 16H3Z" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linejoin="round"/>
      ${featureHash(5.1, 5, .44, color ? P.r : fg, 2.7)}`,
  },
  {
    id: 'fn-window-sketch', name: '桌面小便签', tag: '界面缩影', batch: 6,
    feature: '胶囊标签 + 正文编辑 + 待办',
    desc: '用钢笔线画出当前应用的窗口结构，顶栏和内容一起保留。最贴近界面，但缩小后细节会多于前两款。',
    app: u => windowMini(u, true), trayW: 22, tray: windowTray,
  },
  {
    id: 'fn-stacked-notes', name: '多页随记', tag: '多标签', batch: 6,
    feature: '每个标签独立保存笔记',
    desc: '几张错开的彩色便签，每页一个标签色；最前面写着笔记。表示多份本地笔记，不暗示文件夹或云端协作。',
    app: u => featurePaper(u, `
      <g transform="rotate(-10 512 512)"><rect x="260" y="250" width="440" height="500" rx="24" fill="#b6d7ef" stroke="${INK}" stroke-width="12"/></g>
      <g transform="rotate(5 512 512)"><rect x="300" y="270" width="440" height="500" rx="24" fill="#ffe29b" stroke="${INK}" stroke-width="12"/></g>
      <rect x="326" y="302" width="420" height="480" rx="24" fill="#fffdf6" stroke="${INK}" stroke-width="14"/>
      ${bar(370, 350, 142, 60, P.r)}${line(374, 486, 664, 480, INK, 25)}${line(374, 572, 624, 568, '#8a857d', 21)}${line(374, 658, 580, 654, '#8a857d', 21)}`),
    tray: (fg, color) => `<path d="M2 12V2H12" fill="none" stroke="${fg}" stroke-width="1.4" stroke-linecap="round"/>
      <rect x="5" y="5" width="11" height="11" rx="1.3" fill="none" stroke="${fg}" stroke-width="1.4"/>
      ${bar(7, 7, 5.8, 2.4, color ? P.r : fg)}${line(7.4, 12.4, 12.6, 12.4, fg, 1.3)}`,
  },
  {
    id: 'fn-todo-sketch', name: '随手勾记', tag: '待办方向', batch: 6,
    feature: 'Markdown 待办勾选并回写',
    desc: '彩色勾选框配手绘便签，表达随手记录和完成事项。更偏待办功能，不如前两款全面代表应用。',
    app: u => todoNote(u, true),
    tray: (fg, color) => `<rect x="2" y="2" width="5.6" height="5.6" rx="1" fill="none" stroke="${fg}" stroke-width="1.3"/>
      ${check(3, 4.9, 4, color ? P.r : fg, 1.4)}${line(10, 4.8, 16, 4.8, fg, 1.4)}
      <rect x="2" y="11" width="5.6" height="5.6" rx="1" fill="none" stroke="${fg}" stroke-width="1.3"/>${line(10, 13.8, 14.6, 13.8, fg, 1.4)}`,
  },
  {
    id: 'fn-pin-note', name: '钉住灵感', tag: '浮动便签', batch: 6,
    feature: '窗口置顶，边工作边记',
    desc: '红图钉钉住一张有彩色标签的便签，突出桌面浮动与置顶。图钉沿用应用内的同类符号。',
    app: u => featurePaper(u, `
      <rect x="230" y="346" width="530" height="430" rx="26" fill="#dedbd4" stroke="#a19b91" stroke-width="10"/>
      <g transform="rotate(-5 512 540)"><path d="M292 310H738V748H292Z" fill="#fff0b7" stroke="${INK}" stroke-width="14" stroke-linejoin="round"/>
      ${[P.r, P.o, P.g].map((c, i) => `<circle cx="${346 + i * 44}" cy="378" r="14" fill="${c}"/>`).join('')}
      ${line(348, 498, 658, 498, INK, 25)}${line(348, 588, 586, 588, '#999077', 21)}</g>
      ${lucideAt(570, 170, 12, `<path d="${LU_PIN}"/><path d="M12 17v5"/>`, INK, 1.1, P.r)}`, ['#e5efdf', '#d2dfca']),
    tray: (fg, color) => `<path d="M3 7V16H15V11" fill="none" stroke="${fg}" stroke-width="1.3" stroke-linejoin="round"/>
      ${lucideAt(6, .2, .5, `<path d="${LU_PIN}"/><path d="M12 17v5"/>`, color ? P.r : fg, 2.6)}`,
  },
  {
    id: 'fn-tray-sketch', name: '菜单栏随记', tag: '驻留呼出', batch: 6,
    feature: '菜单栏驻留，点击显示或隐藏',
    desc: '上方圆环代表托盘，下方是唤出的笔记窗口。是功能示意，不表示应用实际采用下拉气泡窗口。',
    app: u => trayDrop(u, true), tray: trayDropTray,
  },
);
