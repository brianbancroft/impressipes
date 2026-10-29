import { Copy } from 'lucide-react';
import QRCode from 'qrcode';
import { useEffect, useState } from 'react';
import { Button } from './ui/button';
import { Blob } from './ui/sticker';
import { Toast } from './ui/toast';

export function AppSetup() {
  const server = window.location.origin;
  const [svg, setSvg] = useState('');
  const [toast, setToast] = useState('');

  useEffect(() => {
    // Black on white at any theme: scanners need the contrast.
    QRCode.toString(server, { type: 'svg', margin: 1, errorCorrectionLevel: 'M', color: { dark: '#000000', light: '#ffffff' } }).then(setSvg);
  }, [server]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(server);
      setToast('Server address copied');
    } catch {
      setToast('Select the address and copy it');
    }
    setTimeout(() => setToast(''), 2500);
  }

  return (
    <div className="relative isolate max-w-180">
      <Blob shape="blob-b" className="-top-24 -right-20 size-64 bg-lilac" />
      <div className="relative mb-6">
        <p className="scribble md:text-[26px]">point the phone here</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">App setup</h1>
        <p className="mt-2 text-ink-muted">
          Scan this code in the iOS app so it knows which server to talk to. You won't have to type the address.
        </p>
      </div>

      <section className="card relative mb-6 flex flex-col items-center gap-4 p-5">
        <div
          role="img"
          aria-label={`QR code for ${server}`}
          className="size-60 max-w-full rounded-md border-[2.5px] border-outline bg-white p-2 [&_svg]:size-full"
          dangerouslySetInnerHTML={{ __html: svg }}
        />
        <p className="max-w-full text-center font-bold [overflow-wrap:anywhere]">{server}</p>
        <Button variant="sticker" onClick={copy}><Copy />Copy address</Button>
      </section>

      <section className="card relative flex flex-col gap-2 p-5">
        <h2 className="text-2xl">In the app</h2>
        <ol className="m-0 flex list-decimal flex-col gap-1.5 pl-5 text-ink-muted [overflow-wrap:anywhere]">
          <li>Open Impressipes. On first launch, tap <b>Scan QR code</b>.</li>
          <li>Already set up? Go to <b>Settings → Server → Scan QR code</b>. You'll be signed out, then use a new invite link.</li>
          <li>Point the camera at the code above.</li>
        </ol>
        <p className="mt-2 text-sm text-ink-muted">
          Running your own copy? Open this page from your own server, so the code carries your address.
        </p>
      </section>
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}
