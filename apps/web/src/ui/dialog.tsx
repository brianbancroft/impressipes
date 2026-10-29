import * as D from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { cn } from './cn';

export const Dialog = D.Root;
export const DialogClose = D.Close;

/** Bottom sheet on phones, centred card from 640px up. */
export function DialogContent({ title, description, className, children }: {
  title: React.ReactNode; description?: React.ReactNode; className?: string; children: React.ReactNode;
}) {
  return (
    <D.Portal>
      <D.Overlay className="fixed inset-0 z-40 bg-[#2a1630]/55" />
      <D.Content
        className={cn(
          'fixed inset-x-0 bottom-0 z-50 max-h-[90dvh] overflow-y-auto rounded-t-xl border-[2.5px] border-b-0 border-outline bg-bg p-5 pb-8 text-ink outline-none',
          'sm:inset-auto sm:top-1/2 sm:left-1/2 sm:w-[min(92vw,30rem)] sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-xl sm:border-b-[2.5px] sm:p-6 sm:shadow-pop-lg',
          className,
        )}
      >
        <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-line sm:hidden" aria-hidden />
        <div className="mb-4 flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <D.Title className="font-display text-2xl leading-tight font-extrabold tracking-tight">{title}</D.Title>
            {description ? <D.Description className="mt-1 text-ink-muted">{description}</D.Description> : <D.Description className="sr-only">{title}</D.Description>}
          </div>
          <D.Close aria-label="Close" className="grid size-10 flex-none place-items-center rounded-full border-[2.5px] border-outline bg-surface text-ink">
            <X className="size-4.5" strokeWidth={2.8} />
          </D.Close>
        </div>
        {children}
      </D.Content>
    </D.Portal>
  );
}
