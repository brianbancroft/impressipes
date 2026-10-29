import { Heart, ShoppingCart, Soup } from 'lucide-react';
import { Blob, Sticker } from './ui/sticker';
import { LogoMark, Wordmark } from './ui/logo';

/**
 * Signed-out / standalone screens. Phones get one column with the logo on top;
 * from 1024px a plum brand panel sits on the left and the form centres on the right.
 */
export function Shell({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative isolate flex min-h-dvh overflow-x-clip">
      <aside className="relative isolate hidden w-[42%] max-w-152 flex-none flex-col justify-between overflow-hidden bg-rail p-12 text-on-rail lg:sticky lg:top-0 lg:flex lg:h-dvh xl:p-16">
        <Blob className="-top-28 -right-24 size-96 bg-accent-fill opacity-90" />
        <Blob shape="blob-b" className="-bottom-32 -left-24 size-104 bg-lilac/25" />
        <div className="relative flex items-center gap-3">
          <LogoMark className="size-12 border-on-rail shadow-[3px_3px_0_var(--color-butter)]" />
          <Wordmark className="text-3xl" />
        </div>
        <div className="relative">
          <p className="scribble text-[28px] text-butter">for the whole family</p>
          <p className="mt-2 font-display text-5xl leading-[1.02] font-extrabold tracking-tight xl:text-6xl">
            Recipes to impress. One shopping list.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Sticker tilt={-3} className="bg-butter"><Soup />Recipes</Sticker>
            <Sticker tilt={2} className="bg-mint"><ShoppingCart />Shared lists</Sticker>
            <Sticker tilt={-1} className="bg-rose"><Heart />Family faves</Sticker>
          </div>
        </div>
        <p className="relative text-sm opacity-70">impressipes.bancroft.io</p>
      </aside>

      <main className="relative isolate flex min-w-0 flex-1 flex-col items-center px-4 pt-10 pb-12 sm:px-8 md:justify-center md:py-16">
        <Blob className="-top-24 -right-20 size-72 bg-butter lg:size-80" />
        <Blob shape="blob-b" className="-bottom-28 -left-20 size-64 bg-lilac lg:hidden" />
        <div className="relative w-full max-w-110">
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <LogoMark className="size-11 shadow-pop-sm" />
            <Wordmark className="text-[28px]" />
          </div>
          {children}
        </div>
      </main>
    </div>
  );
}
