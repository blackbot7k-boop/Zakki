// ⑦ 简约：每款 1–2 个元素，大面积留白，保证 Dock 32px 时仍一眼可认。
const minTile = (u, colors, body, light = false) => tile(u, colors, body, { light, gloss: 0.12 });

OPTIONS.push(
  {
    id: 'min-pill', name: '红签', tag: '简约 · 标签', batch: 7,
    feature: '当前标签胶囊',
    desc: '红底上只放一枚白色胶囊和一颗淡点，就是 Zakki 标签栏的缩影。元素最少，Dock 里最醒目。',
    app: u => minTile(u, ['#ff5a67', '#d4253a'], `
      ${bar(222, 442, 360, 140, '#fff')}<circle cx="726" cy="512" r="70" fill="#fff" fill-opacity=".5"/>`),
    tray: fg => `${bar(1.5, 6, 9.5, 6, fg)}<circle cx="14.5" cy="9" r="2.6" fill="none" stroke="${fg}" stroke-width="1.4"/>`,
  },
  {
    id: 'min-note', name: '一张便签', tag: '简约 · 便签', batch: 7,
    feature: '便签',
    desc: '暖黄底上一张白色折角便签，没有任何文字线条。形状本身就说明这是便签。',
    app: u => minTile(u, ['#ffe27a', '#f4b52c'], `
      <path d="M292 262H732V610L592 762H292Z" fill="#fff"/>
      <path d="M732 610H624Q592 610 592 642V762Z" fill="#f6d77e"/>`),
    tray: fg => `<path d="M3 2.5H15V11.2L11 15.5H3Z" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linejoin="round"/>
      <path d="M15 11.2H11V15.5" fill="none" stroke="${fg}" stroke-width="1.5" stroke-linejoin="round"/>`,
  },
  {
    id: 'min-z', name: '方 Z', tag: '简约 · 字母', batch: 7,
    feature: '品牌名首字母',
    desc: '深色底上一个圆头白色 Z，几何、干净，和系统工具类应用并排也不突兀。',
    app: u => minTile(u, ['#3a3a3c', '#1c1c1e'], `
      <path d="M332 334H692L332 690H692" fill="none" stroke="#fff" stroke-width="92" stroke-linecap="round" stroke-linejoin="round"/>`),
    tray: fg => `<path d="M4 4H14L4 14H14" fill="none" stroke="${fg}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>`,
  },
  {
    id: 'min-z-ink', name: '墨 Z', tag: '简约 · 手写', batch: 7,
    feature: '手写速记',
    desc: '米色纸上一笔写成的墨色 Z，末尾一颗朱红句点。保留手绘味道，但只有两个元素。',
    app: u => minTile(u, ['#fffaf0', '#efe4cf'], `
      <path d="M316 346Q520 318 698 334Q520 512 324 700Q514 686 690 690" fill="none" stroke="#1d1d1f" stroke-width="64" stroke-linecap="round" stroke-linejoin="round"/>
      <circle cx="770" cy="690" r="34" fill="${P.r}"/>`, true),
    tray: (fg, color) => `<path d="M3.5 4.4Q9 3.6 13.6 4Q9 8.8 3.8 13.8Q8.8 13.4 13.4 13.5" fill="none" stroke="${fg}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
      <circle cx="16" cy="13.5" r="1.3" fill="${color ? P.r : fg}"/>`,
  },
  {
    id: 'min-hash', name: '井号', tag: '简约 · Markdown', batch: 7,
    feature: 'Markdown',
    desc: '青绿底上一个白色井号，直接指向 Markdown。线条粗、间距大，小尺寸不会糊。',
    app: u => minTile(u, ['#36c9a4', '#13876e'], `
      <g fill="none" stroke="#fff" stroke-width="74" stroke-linecap="round">
        <path d="M440 284L392 740M650 284L602 740M300 428H736M288 596H724"/></g>`),
    tray: fg => `<g fill="none" stroke="${fg}" stroke-width="1.6" stroke-linecap="round"><path d="M7.3 2.8L6 15.2M12.3 2.8L11 15.2M3.4 6.6H15M3 11.4H14.6"/></g>`,
  },
  {
    id: 'min-tabs', name: '标签栏', tag: '简约 · 多标签', batch: 7,
    feature: '液态标签栏',
    desc: '白底上一枚红色胶囊加两颗灰点，就是顶栏的样子。比「彩签速记」去掉了便签和文字。',
    app: u => minTile(u, ['#ffffff', '#ececf0'], `
      ${bar(196, 434, 320, 156, P.r)}<circle cx="636" cy="512" r="62" fill="#c7c7cc"/><circle cx="796" cy="512" r="62" fill="#c7c7cc"/>`, true),
    tray: (fg, color) => `${bar(1, 6, 8.5, 6, color ? P.r : fg)}<circle cx="12" cy="9" r="2" fill="${fg}"/><circle cx="16.5" cy="9" r="2" fill="${fg}"/>`,
    trayW: 19,
  },
  {
    id: 'min-fold', name: '折角', tag: '简约 · 单形', batch: 7,
    feature: '便签',
    desc: '整块图标就是一张黄便签，只在右下角折起一角。真正的单一元素。',
    app: u => minTile(u, ['#ffe58a', '#f6c23e'], `
      <path d="M924 700V924H700Z" fill="#d99a1e"/>
      <path d="M924 700L700 924L700 700Z" fill="#fff4cc"/>
      <path d="M924 700L700 924" stroke="#000" stroke-opacity=".08" stroke-width="4"/>`),
    tray: fg => `<path d="M3 3H15V11L11 15H3Z" fill="none" stroke="${fg}" stroke-width="1.6" stroke-linejoin="round"/><path d="M15 11L11 15V11Z" fill="${fg}"/>`,
  },
  {
    id: 'min-caret', name: '光标', tag: '简约 · 速记', batch: 7,
    feature: '随手输入',
    desc: '白底上一行粗黑字线和一根红色光标，表达「打开就写」。',
    app: u => minTile(u, ['#ffffff', '#efeff2'], `
      ${line(292, 512, 600, 512, '#1d1d1f', 84)}${line(714, 382, 714, 642, P.r, 46)}`, true),
    tray: (fg, color) => `${line(2.5, 9, 10, 9, fg, 2.4)}${line(14, 4.5, 14, 13.5, color ? P.r : fg, 1.8)}`,
  },
  {
    id: 'min-liquid', name: '液态', tag: '简约 · 动效', batch: 7,
    feature: '标签切换的液态融合动画',
    desc: '蓝紫渐变上，一枚胶囊正在和旁边的圆点融合，对应切换标签时的动画。',
    app: u => minTile(u, ['#5b8cff', '#7646ee'], `
      ${bar(230, 442, 320, 140, '#fff')}
      <path d="M510 456Q600 490 660 466L660 558Q600 534 510 568Z" fill="#fff"/>
      <circle cx="704" cy="512" r="78" fill="#fff"/>`),
    tray: fg => `${bar(1, 6, 8, 6, fg)}<path d="M8 7Q10.5 8.2 12 7.4V10.6Q10.5 9.8 8 11Z" fill="${fg}"/><circle cx="14.5" cy="9" r="2.8" fill="${fg}"/>`,
  },
  {
    id: 'min-dot', name: '一点', tag: '简约 · 极简', batch: 7,
    feature: 'Tot 式彩点',
    desc: '浅灰底上一颗大红点，来自 Tot 的点式标签。最极端的简约，识别靠颜色。',
    app: u => minTile(u, ['#f8f8fa', '#e4e4e8'], `<circle cx="512" cy="512" r="176" fill="${P.r}"/>`, true),
    tray: (fg, color) => `<circle cx="9" cy="9" r="5.5" fill="${color ? P.r : fg}"/>`,
  },
);
