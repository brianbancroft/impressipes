// IMP-13 brand: one source for the mark, wordmark, app icons, favicon and OG image.
// Run: node designs/imp-13/brand.mjs && designs/imp-13/render.sh
// Writes export/*.svg, the render pages in export/html/, and index.html (the brand sheet).
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'export');
mkdirSync(join(out, 'html'), { recursive: true });

/* ----- tokens (IMP-14) ----- */
const LIGHT = {
  bg: '#FFF8F0', surface: '#FFFFFF', 'surface-2': '#FBEFE2', line: '#EADBCB', ink: '#2B2118', 'ink-muted': '#6B5D50',
  accent: '#C62F55', 'on-accent': '#FFFFFF', 'accent-soft': '#FCE4EA',
  mustard: '#8A6A00', leaf: '#5E9626', sky: '#3C8DE0', berry: '#C42A7B', tangerine: '#E0701A',
};
const DARK = {
  bg: '#1A1512', surface: '#2A221D', 'surface-2': '#342B24', line: '#43372E', ink: '#F7EEE4', 'ink-muted': '#BDAE9F',
  accent: '#FF8DA5', 'on-accent': '#2B0A12', 'accent-soft': '#4A2230',
  mustard: '#CFA52E', leaf: '#9BD65F', sky: '#7DB8FF', berry: '#FF7DB8', tangerine: '#FFA24F',
};
// Tinted (iOS 18): grayscale only, the system applies the tint.
const TINT = { ...DARK, bg: 'none', mustard: '#9A9A9A', accent: '#F2F2F2', 'on-accent': '#262626', 'accent-soft': '#D0D0D0', leaf: '#C4C4C4', ink: '#FFFFFF' };

/* ----- the pie (same geometry as designs/imp-19 pie()) -----
   Drawn about the origin: scalloped crust r 58.7, filling r 40, leaf at 1 o'clock. */
const FS = (t, w = 3.4) => `fill="none" stroke="${t['on-accent']}" stroke-width="${w}" stroke-linecap="round" stroke-linejoin="round"`;
const FACES = {
  hello: (t) => `<circle cx="-25" cy="8" r="5.5" fill="${t['accent-soft']}" fill-opacity=".9"/><circle cx="25" cy="8" r="5.5" fill="${t['accent-soft']}" fill-opacity=".9"/>
    <circle cx="-13" cy="-5" r="4" fill="${t['on-accent']}"/><circle cx="13" cy="-5" r="4" fill="${t['on-accent']}"/><path d="M-7 6 Q0 13 7 6" ${FS(t)}/>`,
};
const LEAF = (t) => `<g transform="translate(23 -42) rotate(-30)"><path d="M0 0 C4 -13 19 -15 24 -6 C13 -3 6 3 0 0Z" fill="${t.leaf}"/></g>`;

/** Primary mark: the mascot with the hello face. For 48 px and up. */
function pie(t, { face = 'hello' } = {}) {
  let sc = '';
  for (let i = 0; i < 22; i++) { const a = i / 22 * Math.PI * 2; sc += `<circle cx="${(Math.cos(a) * 51.5).toFixed(1)}" cy="${(Math.sin(a) * 51.5).toFixed(1)}" r="7.2"/>`; }
  return `<g fill="${t.mustard}">${sc}<circle r="51"/></g>
    <circle r="42.5" fill="${t.ink}" fill-opacity=".14"/><circle r="40" fill="${t.accent}"/>
    ${face ? FACES[face](t) : ''}${LEAF(t)}`;
}

/** Small mark: no face, a lattice instead, fewer and larger scallops. For 16 to 47 px. */
function pieSmall(t) {
  let sc = '';
  for (let i = 0; i < 12; i++) { const a = i / 12 * Math.PI * 2; sc += `<circle cx="${(Math.cos(a) * 49).toFixed(1)}" cy="${(Math.sin(a) * 49).toFixed(1)}" r="10"/>`; }
  // Strips stop inside the crust circle (r 50), which is the same colour, so no clip is needed.
  const strip = (x, y, w, h) => `<rect x="${x}" y="${y}" width="${w}" height="${h}" rx="5.5"/>`;
  return `<g fill="${t.mustard}">${sc}<circle r="50"/></g>
    <circle r="38" fill="${t.accent}"/>
    <g fill="${t.mustard}">${strip(-46, -19, 92, 11)}${strip(-46, 8, 92, 11)}${strip(-19, -46, 11, 92)}${strip(8, -46, 11, 92)}</g>`;
}

const svg = (w, h, body, { vb = `0 0 ${w} ${h}`, label = 'Impressipes', extra = '' } = {}) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${vb}" width="${w}" height="${h}" role="img" aria-label="${label}">${extra}<title>${label}</title>${body}</svg>\n`;

/* ----- marks ----- */
const markSvg = (t) => svg(256, 256, `<g transform="translate(124 136) scale(1.85)">${pie(t)}</g>`, { label: 'Impressipes pie' });
const smallSvg = (t) => svg(128, 128, `<g transform="translate(64 64) scale(1.06)">${pieSmall(t)}</g>`, { label: 'Impressipes pie' });

/* ----- wordmark: lowercase Nunito 800, ink; lockup = mark + word ----- */
const FONT = `<style>@import url('https://fonts.googleapis.com/css2?family=Nunito:wght@800&amp;display=block');text{font-family:Nunito,ui-rounded,"SF Pro Rounded",system-ui,sans-serif;font-weight:800}</style>`;
const WORD_W = 560; // Nunito 800 "impressipes" at 112 px, pinned with textLength so layout never depends on the fallback face
const word = (t, x, y, size = 112, fill = t.ink) =>
  `<text x="${x}" y="${y}" font-size="${size}" fill="${fill}" letter-spacing="-1" textLength="${(WORD_W * size / 112).toFixed(0)}" lengthAdjust="spacingAndGlyphs">impressipes</text>`;
const lockupSvg = (t, bg) => svg(800, 200,
  `${bg ? `<rect width="800" height="200" rx="0" fill="${t.bg}"/>` : ''}<g transform="translate(96 106) scale(1.38)">${pie(t)}</g>${word(t, 200, 136)}`,
  { extra: FONT, label: 'Impressipes' });
const stackedSvg = (t) => svg(560, 420, `<g transform="translate(276 150) scale(2)">${pie(t)}</g>${word(t, 280 - 230, 370, 92)}`, { extra: FONT, label: 'Impressipes' })
  .replace('textLength', 'text-anchor="start" textLength');

/* ----- app icons, 1024 square. iOS masks to a squircle, watchOS to a circle. ----- */
const icon = (t, { bg = t.bg } = {}) => svg(1024, 1024,
  `${bg === 'none' ? '' : `<rect width="1024" height="1024" fill="${bg}"/>`}
   <ellipse cx="512" cy="884" rx="250" ry="30" fill="${t.ink}" fill-opacity="${bg === 'none' ? 0 : .08}"/>
   <g transform="translate(500 540) scale(5.3)">${pie(t)}</g>`, { label: 'Impressipes app icon' });

/* ----- favicon: small mark, follows the browser's colour scheme ----- */
const favicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-60 -60 120 120" width="32" height="32"><style>
.c{fill:${LIGHT.mustard}}.f{fill:${LIGHT.accent}}@media (prefers-color-scheme:dark){.c{fill:${DARK.mustard}}.f{fill:${DARK.accent}}}</style>
${pieSmall({ mustard: 'X', accent: 'Y' }).replaceAll('fill="X"', 'class="c"').replaceAll('fill="Y"', 'class="f"')}</svg>\n`;

/* ----- OG image 1200 x 630 ----- */
const star = (x, y, s, c) => `<path transform="translate(${x} ${y}) scale(${s / 10})" d="M0 -10 Q1.6 -1.6 10 0 Q1.6 1.6 0 10 Q-1.6 1.6 -10 0 Q-1.6 -1.6 0 -10z" fill="${c}"/>`;
const og = (t) => svg(1200, 630, `<rect width="1200" height="630" fill="${t.bg}"/>
  <circle cx="300" cy="330" r="230" fill="${t['accent-soft']}"/>
  <ellipse cx="300" cy="520" rx="170" ry="18" fill="${t.ink}" fill-opacity=".08"/>
  <g transform="translate(296 318) scale(3.2)">${pie(t)}</g>
  ${star(118, 150, 22, t.mustard)}${star(498, 470, 16, t.sky)}${star(490, 150, 12, t.berry)}<circle cx="150" cy="500" r="8" fill="${t.tangerine}"/>
  ${word(t, 590, 300, 108)}
  <text x="594" y="372" font-size="38" fill="${t['ink-muted']}" style="font-family:system-ui,-apple-system,'Segoe UI',sans-serif;font-weight:500">Recipes to impress.</text>
  <text x="594" y="420" font-size="38" fill="${t['ink-muted']}" style="font-family:system-ui,-apple-system,'Segoe UI',sans-serif;font-weight:500">One shopping list for the family.</text>
  <rect x="594" y="470" width="300" height="56" rx="28" fill="${t.accent}"/>
  <text x="744" y="507" font-size="26" text-anchor="middle" fill="${t['on-accent']}">impressipes.bancroft.io</text>`,
  { extra: FONT, label: 'Impressipes. Recipes to impress. One shopping list for the family.' });

/* ----- write exports ----- */
const files = {
  'mark-light.svg': markSvg(LIGHT), 'mark-dark.svg': markSvg(DARK),
  'mark-small-light.svg': smallSvg(LIGHT), 'mark-small-dark.svg': smallSvg(DARK),
  'lockup-light.svg': lockupSvg(LIGHT), 'lockup-dark.svg': lockupSvg(DARK),
  'lockup-stacked-light.svg': stackedSvg(LIGHT), 'lockup-stacked-dark.svg': stackedSvg(DARK),
  'icon-ios-light.svg': icon(LIGHT), 'icon-ios-dark.svg': icon(DARK), 'icon-ios-tinted.svg': icon(TINT, { bg: 'none' }),
  'icon-watch.svg': icon(LIGHT),
  'favicon.svg': favicon,
  'og-light.svg': og(LIGHT),
};
for (const [name, body] of Object.entries(files)) writeFileSync(join(out, name), body);

// Render pages: each export in a page of exactly its size, so headless Chrome screenshots it 1:1
// after the web font loads. render.sh reads export/html/manifest.txt (name width height).
const sizes = [];
for (const [name, body] of Object.entries(files)) {
  const [, w, h] = body.match(/width="(\d+)" height="(\d+)"/);
  const transparent = !/^(og|icon-ios-light|icon-ios-dark|icon-watch)/.test(name);
  writeFileSync(join(out, 'html', name.replace('.svg', '.html')),
    `<!doctype html><meta charset="utf-8"><style>html,body{margin:0;background:${transparent ? 'transparent' : '#000'}}svg{display:block}</style>${body}`);
  sizes.push(`${name.replace('.svg', '')} ${w} ${h}`);
}
writeFileSync(join(out, 'html', 'manifest.txt'), sizes.join('\n') + '\n');

/* ----- brand sheet (index.html), rendered to PDF by render.sh ----- */
const inline = (s, w) => s.replace(/width="\d+" height="\d+"/, `width="${w}" style="height:auto"`).replace(/<title>.*?<\/title>/, '');
const iconAt = (s, w, r) => `<div style="width:${w}px;height:${w}px;border-radius:${r};overflow:hidden;flex:none;box-shadow:0 1px 3px rgba(0,0,0,.18)">${inline(s, w)}</div>`;
const swatch = (hex, name, note) => `<div class="sw"><span style="background:${hex}"></span><b>${name}</b><code>${hex}</code><i>${note}</i></div>`;
const page = (title, sub, body) => `<section class="page"><h1>${title}</h1><p class="sub">${sub}</p>${body}</section>`;

const pages = [];
pages.push(`<section class="page cover">
  <div class="cv">${inline(stackedSvg(LIGHT), 520)}</div>
  <div class="cvt"><h1>Impressipes brand (IMP-13)</h1>
  <p class="sub">Impress + recipes. A warm, cute family app for recipes and shopping lists. The pie mascot from IMP-19 is the mark, rhubarb pink from IMP-14 is the accent, Nunito 800 is the wordmark.</p>
  <ul class="dec">
    <li><b>Name:</b> Impressipes, written <code>impressipes</code> in the wordmark and “Impressipes” in sentence text. Domain <code>impressipes.bancroft.io</code>.</li>
    <li><b>Mark:</b> the pie mascot, hello face, leaf at 1 o’clock. Below 48 px it becomes the small mark: no face, a lattice, 12 big scallops.</li>
    <li><b>Colour:</b> rhubarb <code>#C62F55</code> filling and mustard crust on cream <code>#FFF8F0</code>; <code>#FF8DA5</code> and <code>#CFA52E</code> on <code>#1A1512</code> in dark. No new colours.</li>
    <li><b>App icon:</b> pie on cream for iOS and watchOS; dark and tinted variants for iOS 18. The watch circle crop keeps the whole pie.</li>
    <li><b>Web:</b> SVG favicon that follows the colour scheme, ICO and PNG fallbacks, apple-touch-icon, manifest icons, 1200 × 630 OG image.</li>
  </ul></div></section>`);

pages.push(page('Personality and voice', 'Who the app is when it talks. The pie shows up to cheer or console, never to decorate. Copy is short and plain.', `<div class="grid3">
  <section class="card"><h3>Warm</h3><p>A home kitchen, not a restaurant. Family names, real food, cream paper and brown ink.</p><p class="ex do">“Sam added 3 things to Costco.”</p><p class="ex dont">“Your collaborative list has been updated.”</p></section>
  <section class="card"><h3>Playful, quietly</h3><p>The pie appears in empty, done, offline and error states. It is never on a busy screen, never a pattern, never in the way of the list.</p><p class="ex do">“All done. Nothing left to get.”</p><p class="ex dont">“Woohoo!! 🎉🎉 You crushed it!!!”</p></section>
  <section class="card"><h3>Plain-spoken</h3><p>Short words, sentence case, verbs on buttons. One exclamation mark at most, and only to celebrate.</p><p class="ex do">“Add to list” · “Try again”</p><p class="ex dont">“Proceed” · “An unexpected error has occurred”</p></section>
</div>
<div class="grid3" style="margin-top:24px">
  <section class="card"><h3>Tagline</h3><p class="big">Recipes to impress.<br>One shopping list for the family.</p><p>Used on the OG image and sign-in. Two lines, the second is the useful one.</p></section>
  <section class="card"><h3>Naming in copy</h3><ul><li>“Impressipes”, capital I, in sentences and the App Store.</li><li>Never “Impressipies”, “ImpressiPes” or “the Impressipes app”.</li><li>The mascot has no name in the UI. It is “the pie” in docs and alt text.</li></ul></section>
  <section class="card"><h3>Type</h3><p class="big" style="font:800 40px var(--display)">impressipes</p><p>Wordmark and titles: Nunito 800 on web, SF Pro Rounded Heavy on Apple. Everything else is system text (IMP-14).</p></section>
</div>`));

pages.push(page('The mark', 'One pie, two sizes. The primary mark is the IMP-19 mascot unchanged. The small mark drops the face and adds a lattice, which answers the IMP-19 worry that the scalloped crust reads as a flower at 32 to 40 pt.', `<div class="row">
  <div class="tile light">${inline(markSvg(LIGHT), 240)}<span>Primary · light</span></div>
  <div class="tile dark">${inline(markSvg(DARK), 240)}<span>Primary · dark</span></div>
  <div class="tile light">${inline(smallSvg(LIGHT), 150)}<span>Small · light</span></div>
  <div class="tile dark">${inline(smallSvg(DARK), 150)}<span>Small · dark</span></div>
  <div class="tile light ladder">${[64, 48, 32, 24, 16].map((s) => `<div>${inline(s >= 48 ? markSvg(LIGHT) : smallSvg(LIGHT), s)}<small>${s}</small></div>`).join('')}<span>primary ≥ 48 · small &lt; 48</span></div>
</div>
<div class="grid3" style="margin-top:28px">
  <section class="card"><h3>Construction</h3><ul><li>Crust: mustard, 22 scallops (small: 12).</li><li>Filling: accent, with a 14% ink rim for depth.</li><li>Face: on-accent eyes and smile, accent-soft cheeks.</li><li>Leaf: leaf colour, at 1 o’clock, never moves.</li></ul></section>
  <section class="card"><h3>Clear space and minimums</h3><ul><li>Clear space: one filling radius (40 units) on every side.</li><li>Primary mark minimum 48 px; small mark 16 px.</li><li>Lockup minimum 120 px wide.</li></ul></section>
  <section class="card"><h3>Don’t</h3><ul><li>Recolour outside the tokens, or put the pie on rhubarb (the filling disappears).</li><li>Rotate, tile, crop or outline it.</li><li>Change the face in the logo. Other faces belong to empty states only.</li></ul></section>
</div>`));

pages.push(page('Wordmark and lockups', 'Lowercase <code>impressipes</code> in Nunito 800, ink colour, tracking −1. The pie sits left of the word in the horizontal lockup and above it in the stacked one.', `<div class="row">
  <div class="tile light wide">${inline(lockupSvg(LIGHT), 640)}<span>Horizontal · light</span></div>
  <div class="tile dark wide">${inline(lockupSvg(DARK), 640)}<span>Horizontal · dark</span></div>
</div><div class="row" style="margin-top:24px">
  <div class="tile light">${inline(stackedSvg(LIGHT), 300)}<span>Stacked · light</span></div>
  <div class="tile dark">${inline(stackedSvg(DARK), 300)}<span>Stacked · dark</span></div>
  <section class="card" style="flex:1"><h3>Colour and contrast</h3>
  ${swatch(LIGHT.ink, 'ink on bg', '15.0:1, wordmark light')}
  ${swatch(DARK.ink, 'ink on bg (dark)', '15.8:1, wordmark dark')}
  ${swatch(LIGHT.mustard, 'mustard on bg', '4.8:1, crust edge light')}
  ${swatch(DARK.mustard, 'mustard on bg (dark)', '7.8:1, crust edge dark')}
  ${swatch(LIGHT.accent, 'accent next to mustard', 'filling vs crust 1.1:1, so the ink rim and hue keep them apart')}
  <p>The accent is unchanged from IMP-14, so the p3 contrast table still holds.</p></section>
</div>`));

const home = (bg, s, label) => `<div class="tile" style="background:${bg}">${iconAt(s, 180, '40px')}<span style="color:${bg === '#E9E4DE' ? '#6B5D50' : '#BDAE9F'}">${label}</span></div>`;
pages.push(page('App icons', 'One 1024 px master per appearance; Xcode makes the rest. The pie sits slightly left and low so the leaf balances the top right. No text, no rhubarb background.', `<div class="row">
  ${home('#E9E4DE', icon(LIGHT), 'iOS · Any')}
  ${home('#1A1512', icon(DARK), 'iOS · Dark')}
  <div class="tile" style="background:#111">${iconAt(icon(TINT, { bg: 'none' }).replace('<rect', '<rect'), 180, '40px')}<span style="color:#ddd">iOS · Tinted (grayscale)</span></div>
  <div class="tile" style="background:#000">${iconAt(icon(LIGHT), 180, '50%')}<span style="color:#ddd">watchOS · circle crop</span></div>
</div>
<div class="row" style="margin-top:24px">
  <div class="tile light ladder" style="flex:1">${[180, 120, 87, 60, 40, 29].map((s) => `<div>${iconAt(icon(LIGHT), s, `${Math.round(s * 0.225)}px`)}<small>${s}</small></div>`).join('')}<span>iOS sizes from the master (180 home, 120 spotlight, 87 settings, 60, 40, 29)</span></div>
  <div class="tile dark ladder">${[100, 72, 58, 44].map((s) => `<div>${iconAt(icon(LIGHT), s, '50%')}<small>${s}</small></div>`).join('')}<span>watchOS sizes on black</span></div>
</div>`));

pages.push(page('Web: favicon, home-screen icon, OG image', 'The SVG favicon switches to the dark values with the browser theme. ICO and 32 px PNG are fallbacks. The OG image is light only.', `<div class="row">
  <div class="tile" style="background:#fff;flex:none;width:330px"><div class="tab light-tab">${inline(favicon, 16)}<b>Impressipes</b></div><div class="tab dark-tab">${inline(favicon.replace(/\.c\{fill:#8A6A00\}\.f\{fill:#C62F55\}/, `.c{fill:${DARK.mustard}}.f{fill:${DARK.accent}}`), 16)}<b>Impressipes</b></div><span>Browser tabs, light and dark</span></div>
  <div class="tile light" style="flex:none">${iconAt(icon(LIGHT), 120, '27px')}<span>apple-touch-icon 180, manifest 192 / 512</span></div>
  <div class="tile light" style="flex:1">${inline(og(LIGHT), 720)}<span>og.png, 1200 × 630</span></div>
</div>
<div class="grid3" style="margin-top:28px">
  <section class="card"><h3>Head tags</h3><pre>&lt;link rel="icon" href="/favicon.ico" sizes="32x32"&gt;
&lt;link rel="icon" href="/favicon.svg" type="image/svg+xml"&gt;
&lt;link rel="apple-touch-icon" href="/apple-touch-icon.png"&gt;
&lt;link rel="manifest" href="/site.webmanifest"&gt;
&lt;meta name="theme-color" content="#FFF8F0"&gt;
&lt;meta property="og:image" content="…/og.png"&gt;</pre></section>
  <section class="card"><h3>Manifest</h3><ul><li><code>name</code> and <code>short_name</code>: Impressipes</li><li><code>background_color</code> and <code>theme_color</code>: <code>#FFF8F0</code></li><li>Icons 192 and 512, plus 512 maskable (the pie fits the 80% safe circle).</li></ul></section>
  <section class="card"><h3>Sign-in and invite</h3><p>The web placeholder mark (an “I” on indigo) is replaced by the primary mark at 56 px above the title. IMP-30 builds those screens.</p></section>
</div>`));

pages.push(page('Handoff', 'Every file is generated by <code>designs/imp-13/brand.mjs</code> and rendered by <code>render.sh</code>. Change the generator, not the files.', `<div class="grid3">
  <section class="card"><h3>designs/imp-13/export/</h3><ul>
  <li><code>mark-{light,dark}.svg/png</code>, <code>mark-small-{light,dark}.svg/png</code></li>
  <li><code>lockup-{light,dark}.svg/png</code>, <code>lockup-stacked-{light,dark}.svg/png</code></li>
  <li><code>og-light.svg</code>, <code>og.png</code></li>
  <li><code>AppIcon.appiconset/</code> (iOS: Any, Dark, Tinted)</li>
  <li><code>WatchAppIcon.appiconset/</code> (watchOS)</li></ul></section>
  <section class="card"><h3>Where they go</h3><ul>
  <li><b>IMP-27</b> (XcodeGen): copy the two appiconsets into each target's <code>Assets.xcassets</code> as <code>AppIcon</code>. Both targets already set <code>ASSETCATALOG_COMPILER_APPICON_NAME = AppIcon</code>.</li>
  <li><b>IMP-44</b> (web shell): favicon, icons, manifest and OG tags are already in <code>apps/web</code>. Self-host Nunito 800 there for the wordmark.</li>
  <li><b>IMP-30, IMP-31</b>: primary mark above sign-in and invite titles.</li></ul></section>
  <section class="card"><h3>Open</h3><ul>
  <li>The code still says “Impressipies” (bundle IDs, passkey RP name, web titles, Xcode targets). IMP-28 plans <code>io.bancroft.impressipes</code>; rename with it.</li>
  <li>The SVG wordmark uses live Nunito text pinned with <code>textLength</code>. Convert it to outlines once Nunito is self-hosted (IMP-44).</li>
  <li>Check the small mark and the 29 pt icon on a device in IMP-12 round 2.</li></ul></section>
</div>`));

writeFileSync(join(here, 'index.html'), `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>Impressipes brand (IMP-13)</title>
<style>
@import url('https://fonts.googleapis.com/css2?family=Nunito:wght@600;700;800&display=block');
@page { size: 1760px 1080px; margin: 0; }
* { box-sizing: border-box; }
:root { --display: "Nunito", ui-rounded, "SF Pro Rounded", system-ui, sans-serif; }
html, body { margin: 0; background: #e9e4de; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; color: #2B2118; -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.page { width: 1760px; height: 1080px; padding: 40px 56px; break-after: page; position: relative; overflow: hidden; }
.page > h1 { font: 800 28px var(--display); margin: 0 0 4px; }
.page > .sub { font: 500 15px/1.4 system-ui; color: #6B5D50; margin: 0 0 22px; max-width: 1500px; }
.cover { display: flex; gap: 72px; align-items: center; background: #FFF8F0; }
.cover h1 { font: 800 44px var(--display); margin: 0 0 12px; }
.cover .sub { font-size: 19px; max-width: 760px; }
.dec { font: 400 17px/1.55 system-ui; padding-left: 20px; max-width: 900px; }
.dec li { margin-bottom: 8px; }
code { font: 600 .85em ui-monospace, Menlo, monospace; background: #fbefe2; padding: 1px 5px; border-radius: 5px; }
.grid3 { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 24px; }
.card { background: #FFF8F0; border: 1px solid #EADBCB; border-radius: 20px; padding: 22px 24px; }
.card h3 { font: 800 19px var(--display); margin: 0 0 10px; }
.card p, .card li { font: 400 15px/1.5 system-ui; margin: 0 0 8px; }
.card ul { padding-left: 18px; margin: 0; }
.card .big { font: 700 22px/1.35 var(--display); }
.ex { padding: 8px 12px; border-radius: 12px; }
.ex.do { background: #E3F0DA; } .ex.do::before { content: "Do  "; font-weight: 700; }
.ex.dont { background: #FCE4EA; } .ex.dont::before { content: "Don’t  "; font-weight: 700; }
pre { font: 500 12.5px/1.6 ui-monospace, Menlo, monospace; background: #fbefe2; padding: 12px; border-radius: 12px; margin: 0; white-space: pre-wrap; }
.row { display: flex; gap: 24px; align-items: stretch; }
.tile { border-radius: 20px; padding: 28px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; position: relative; padding-bottom: 48px; }
.tile.light { background: #FFF8F0; border: 1px solid #EADBCB; } .tile.dark { background: #1A1512; }
.tile > span { position: absolute; bottom: 14px; font: 600 13px ui-monospace, Menlo, monospace; color: #6B5D50; }
.tile.dark > span { color: #BDAE9F; }
.tile.wide { flex: 1; }
.ladder { flex-direction: row; align-items: flex-end; gap: 22px; }
.ladder div { display: flex; flex-direction: column; align-items: center; gap: 6px; }
.ladder small { font: 600 12px ui-monospace, Menlo, monospace; color: #6B5D50; }
.tile.dark small { color: #BDAE9F; }
.sw { display: grid; grid-template-columns: 28px 170px 90px 1fr; align-items: center; gap: 10px; font: 400 14px system-ui; margin-bottom: 8px; }
.sw span { width: 28px; height: 28px; border-radius: 50%; border: 1px solid rgba(0,0,0,.1); }
.sw i { font-style: normal; color: #6B5D50; }
.tab { display: flex; align-items: center; gap: 8px; width: 240px; padding: 10px 14px; border-radius: 10px 10px 0 0; font: 500 14px system-ui; }
.light-tab { background: #f1f1f1; color: #222; } .dark-tab { background: #2b2b2b; color: #eee; }
</style></head><body>
${pages.join('\n')}
</body></html>
`);
console.log(`wrote ${Object.keys(files).length} SVGs, ${sizes.length} render pages and index.html`);
