import { Soup } from 'lucide-react';
import { cn } from './cn';

export function LogoMark({ className }: { className?: string }) {
  return (
    <span aria-hidden className={cn('blob-c grid size-9 flex-none -rotate-6 place-items-center border-2 border-outline bg-accent-fill text-white', className)}>
      <Soup className="size-[55%]" strokeWidth={2.4} />
    </span>
  );
}

export function Wordmark({ className }: { className?: string }) {
  return <span className={cn('font-display text-[22px] font-extrabold tracking-tight', className)}>impressipes</span>;
}
