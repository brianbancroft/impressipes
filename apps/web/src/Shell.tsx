import { Blob } from './ui/sticker';
import { LogoMark, Wordmark } from './ui/logo';
import { cn } from './ui/cn';

/** Signed-out / standalone screens: blush ground, blobs, one centred column. */
export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <main className="relative isolate grid min-h-dvh place-items-start justify-center overflow-hidden px-4 pt-[10vh] pb-8">
      <Blob className="-top-24 -right-20 size-80 bg-butter" />
      <Blob shape="blob-b" className="-bottom-28 -left-20 size-72 bg-lilac" />
      <div className={cn('relative w-full', wide ? 'max-w-160' : 'max-w-105')}>
        <div className="mb-6 flex items-center gap-3">
          <LogoMark className="size-12 shadow-pop-sm" />
          <Wordmark className="text-3xl" />
        </div>
        {children}
      </div>
    </main>
  );
}
