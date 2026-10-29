import { type LucideIcon } from 'lucide-react';
import { Button } from './ui/button';
import { cn } from './ui/cn';

export function Loading({ label = 'Loading…' }: { label?: string }) {
  return (
    <p className="flex items-center gap-3 py-6 font-bold text-ink-muted" role="status">
      <span className="blob-a size-5 animate-spin border-2 border-outline bg-butter [animation-duration:1.4s]" aria-hidden />
      {label}
    </p>
  );
}

export function ErrorState({ onRetry }: { onRetry?: () => void }) {
  return (
    <div role="alert" className="card mx-auto max-w-md p-6 text-center">
      <h1 className="text-2xl">Something went wrong</h1>
      <p className="mt-1 mb-4 text-ink-muted">Check your connection and try again.</p>
      {onRetry && <Button onClick={onRetry}>Retry</Button>}
    </div>
  );
}

export function EmptyState({ icon: Icon, title, body, fill = 'bg-butter', children }: {
  icon: LucideIcon; title: string; body: string; fill?: string; children?: React.ReactNode;
}) {
  return (
    <div className="mx-auto grid max-w-md place-items-center gap-3 rounded-xl border-[2.5px] border-dashed border-outline px-6 py-12 text-center">
      <span className={cn('blob-d grid size-20 -rotate-6 place-items-center border-[2.5px] border-outline text-sticker-ink shadow-pop-sm', fill)} aria-hidden>
        <Icon className="size-9" strokeWidth={2} />
      </span>
      <h2 className="text-2xl">{title}</h2>
      <p className="text-ink-muted">{body}</p>
      {children}
    </div>
  );
}
