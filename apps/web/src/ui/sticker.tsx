import { Check } from 'lucide-react';
import { cn } from './cn';

/** Small outlined pill. Fills are sticker colours, so the text stays sticker-ink. */
export function Sticker({ className, tilt = 0, children, style }: { className?: string; tilt?: number; children: React.ReactNode; style?: React.CSSProperties }) {
  return (
    <span
      className={cn('inline-flex items-center gap-1.5 rounded-full border-2 border-outline bg-butter px-3 py-1 text-sm font-bold text-sticker-ink [&_svg]:size-4', className)}
      style={{ transform: tilt ? `rotate(${tilt}deg)` : undefined, ...style }}
    >
      {children}
    </span>
  );
}

/** Filter chip: a sticker you can press. */
export function Chip({ on, onClick, fill = 'bg-surface', children }: { on: boolean; onClick: () => void; fill?: string; children: React.ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={cn(
        'inline-flex min-h-10 flex-none items-center gap-1.5 rounded-full border-2 border-outline px-4 text-sm font-bold text-sticker-ink transition-transform hover:-translate-y-px [&_svg]:size-4',
        on ? 'bg-accent-fill text-white shadow-pop-sm' : fill,
        fill === 'bg-surface' && !on && 'text-ink',
      )}
    >
      {children}
    </button>
  );
}

/** Round tick-box. Checked = mint with a plum tick. */
export function Tick({ checked, className }: { checked: boolean; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'grid size-7 flex-none place-items-center rounded-full border-[2.5px] border-outline transition-colors',
        checked ? 'bg-mint text-sticker-ink' : 'bg-surface',
        className,
      )}
    >
      {checked && <Check className="size-4" strokeWidth={3.4} />}
    </span>
  );
}

/** Butter quantity badge: big number, small unit. */
export function QtyBadge({ qty, unit, className }: { qty: string; unit?: string; className?: string }) {
  return (
    <span className={cn('inline-flex w-16 flex-none items-baseline justify-center gap-1 rounded-sm border-2 border-outline bg-butter px-1 py-1.5 leading-none text-sticker-ink', className)}>
      <span className="font-display text-lg font-extrabold">{qty}</span>
      {unit && <span className="truncate text-xs font-bold">{unit}</span>}
    </span>
  );
}

/** Decorative organic blob for page backgrounds. */
export function Blob({ className, shape = 'blob-a' }: { className?: string; shape?: 'blob-a' | 'blob-b' | 'blob-c' | 'blob-d' }) {
  return <div aria-hidden className={cn('pointer-events-none absolute -z-0', shape, className)} />;
}
