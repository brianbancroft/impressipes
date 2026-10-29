#!/usr/bin/env bash
# Renders the IMP-13 exports to PNG, builds the appiconsets, web icons and the brand sheet PDF.
# Needs Google Chrome and python3 with Pillow. Run brand.mjs first.
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
out="$here/export"
web="$here/../../apps/web/public"
chrome="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
mkdir -p "$out/png" "$web"

shot() { # name width height
  "$chrome" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
    --default-background-color=00000000 --virtual-time-budget=4000 \
    --window-size="$2,$3" --screenshot="$out/png/$1.png" "file://$out/html/$1.html" >/dev/null 2>&1
}
while read -r name w h; do shot "$name" "$w" "$h"; done < "$out/html/manifest.txt"

"$chrome" --headless=new --disable-gpu --virtual-time-budget=6000 --no-pdf-header-footer \
  --print-to-pdf="$here/impressipes-brand-IMP-13.pdf" "file://$here/index.html" >/dev/null 2>&1

python3 -I - "$out" "$web" <<'PY'
import json, shutil, sys
from pathlib import Path
from PIL import Image
out, web = Path(sys.argv[1]), Path(sys.argv[2])
png = out / 'png'
info = {"author": "xcode", "version": 1}

def opaque(name):
    im = Image.open(png / f'{name}.png').convert('RGBA')
    bg = Image.new('RGB', im.size, im.getpixel((0, 0))[:3])
    bg.paste(im, mask=im.split()[3])
    return bg

# iOS: single-size 1024 with Any, Dark and Tinted appearances.
ios = out / 'AppIcon.appiconset'; ios.mkdir(exist_ok=True)
opaque('icon-ios-light').save(ios / 'icon-1024.png')
Image.open(png / 'icon-ios-dark.png').convert('RGB').save(ios / 'icon-1024-dark.png')
Image.open(png / 'icon-ios-tinted.png').convert('LA').save(ios / 'icon-1024-tinted.png')
base = {"idiom": "universal", "platform": "ios", "size": "1024x1024"}
(ios / 'Contents.json').write_text(json.dumps({"images": [
    {**base, "filename": "icon-1024.png"},
    {**base, "filename": "icon-1024-dark.png", "appearances": [{"appearance": "luminosity", "value": "dark"}]},
    {**base, "filename": "icon-1024-tinted.png", "appearances": [{"appearance": "luminosity", "value": "tinted"}]},
], "info": info}, indent=2) + '\n')

# watchOS: single-size 1024, the system crops it to a circle.
watch = out / 'WatchAppIcon.appiconset'; watch.mkdir(exist_ok=True)
opaque('icon-watch').save(watch / 'icon-1024.png')
(watch / 'Contents.json').write_text(json.dumps({"images": [
    {"idiom": "universal", "platform": "watchos", "size": "1024x1024", "filename": "icon-1024.png"},
], "info": info}, indent=2) + '\n')

# Web.
master = opaque('icon-ios-light')
master.resize((180, 180), Image.LANCZOS).save(web / 'apple-touch-icon.png')
for s in (192, 512):
    master.resize((s, s), Image.LANCZOS).save(web / f'icon-{s}.png')
# Maskable: the pie shrunk into the 80% safe circle on cream.
m = Image.new('RGB', (512, 512), master.getpixel((0, 0)))
m.paste(master.resize((400, 400), Image.LANCZOS), (56, 56))
m.save(web / 'icon-maskable-512.png')
small = Image.open(png / 'mark-small-light.png').convert('RGBA')
small.resize((32, 32), Image.LANCZOS).save(web / 'favicon-32.png')
small.save(web / 'favicon.ico', sizes=[(16, 16), (32, 32), (48, 48)])
shutil.copy(out / 'favicon.svg', web / 'favicon.svg')
opaque('og-light').save(web / 'og.png', optimize=True)
shutil.copy(web / 'og.png', out / 'og.png')
print('appiconsets and web icons written')
PY
echo "done: $out, $web, $here/impressipes-brand-IMP-13.pdf"
