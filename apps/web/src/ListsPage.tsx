import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { ChevronRight, Plus, ShoppingCart } from 'lucide-react';
import { useState } from 'react';
import { createList, listLists } from './api';
import { LIST_COLOURS } from './lists';
import { EmptyState, ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn, tint } from './ui/cn';
import { Dialog, DialogClose, DialogContent } from './ui/dialog';
import { Blob } from './ui/sticker';

export function ListsPage() {
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  const [creating, setCreating] = useState(false);

  return (
    <div className="relative isolate">
      <Blob className="-top-28 -left-24 h-56 w-64 bg-mint" />
      <div className="relative mb-6">
        <p className="scribble md:text-[26px]">what do we need?</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">Lists</h1>
      </div>
      {lists.isPending && <Loading label="Loading lists…" />}
      {lists.isError && <ErrorState onRetry={() => lists.refetch()} />}
      {lists.data?.length === 0 && (
        <EmptyState icon={ShoppingCart} fill="bg-mint" title="No lists yet" body="Make one for each shop.">
          <Button variant="primary" onClick={() => setCreating(true)}><Plus />New list</Button>
        </EmptyState>
      )}
      {!!lists.data?.length && (
        <div className="relative grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <ListTiles />
          <NewListButton onClick={() => setCreating(true)} />
        </div>
      )}
      <p className="relative mt-6 -rotate-1 text-center font-hand text-[22px] text-ink-muted">everyone in the family sees changes live</p>
      <NewListDialog open={creating} onOpenChange={setCreating} />
    </div>
  );
}

export function NewListButton({ onClick }: { onClick: () => void }) {
  return (
    <Button variant="dashed" onClick={onClick} className="min-h-19 rounded-lg text-base"><Plus />New list</Button>
  );
}

const TILTS = ['rotate-0', '-rotate-1', 'rotate-1'];

/** Big colour tiles, one per list. `activeId` marks the open list in the split view. */
export function ListTiles({ activeId, compact }: { activeId?: string; compact?: boolean }) {
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  return lists.data?.map((l, n) => {
    const on = l.id === activeId;
    return (
      <Link
        key={l.id}
        to="/lists/$listId"
        params={{ listId: l.id }}
        aria-current={on ? 'page' : undefined}
        className={cn(
          'flex items-center gap-3.5 rounded-lg border-[2.5px] border-outline px-4 py-3.5 text-sticker-ink no-underline transition-transform hover:-translate-y-0.5 hover:rotate-0 hover:text-sticker-ink',
          on ? 'border-[3px] shadow-pop' : cn('shadow-pop-sm', TILTS[n % TILTS.length]),
          !compact && 'min-h-19',
        )}
        style={{ background: tint(l.colour) }}
      >
        <span className="blob-d grid size-11.5 flex-none place-items-center border-[2.5px] border-outline bg-white text-2xl" aria-hidden>{l.emoji}</span>
        <span className="min-w-0 flex-1 truncate font-display text-[21px] leading-none font-extrabold">{l.name}</span>
        {on && <ChevronRight className="size-5 flex-none" strokeWidth={2.6} aria-hidden />}
      </Link>
    );
  });
}

export function NewListDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [emoji, setEmoji] = useState('🛒');
  const [colour, setColour] = useState(LIST_COLOURS[0]);

  const create = useMutation({
    mutationFn: () => createList({ name: name.trim(), emoji, colour }),
    onSuccess: (list) => {
      qc.invalidateQueries({ queryKey: ['lists'] });
      onOpenChange(false);
      setName('');
      navigate({ to: '/lists/$listId', params: { listId: list.id } });
    },
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title="New list" description="One for each shop works well.">
        <form onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate(); }} className="flex flex-col gap-5">
          <div
            className="flex items-center gap-3 rounded-lg border-[2.5px] border-outline p-3 text-sticker-ink shadow-pop-sm"
            style={{ background: tint(colour) }}
            aria-hidden
          >
            <span className="blob-d grid size-11.5 flex-none place-items-center border-[2.5px] border-outline bg-white text-2xl">{emoji || '🛒'}</span>
            <span className="truncate font-display text-[21px] font-extrabold">{name || 'Your new list'}</span>
          </div>
          <div className="flex gap-3">
            <div className="w-20">
              <label htmlFor="nl-emoji" className="field-label">Icon</label>
              <input id="nl-emoji" value={emoji} maxLength={4} onChange={(e) => setEmoji(e.target.value)} className="field text-center text-xl" />
            </div>
            <div className="min-w-0 flex-1">
              <label htmlFor="nl-name" className="field-label">Name</label>
              <input id="nl-name" value={name} autoFocus placeholder="Costco, Weekly shop…" onChange={(e) => setName(e.target.value)} className="field" />
            </div>
          </div>
          <fieldset className="m-0 border-0 p-0">
            <legend className="field-label">Colour</legend>
            <div className="flex flex-wrap gap-2.5">
              {LIST_COLOURS.map((c) => (
                <button
                  type="button"
                  key={c}
                  className={cn('size-10 cursor-pointer rounded-full border-[2.5px] border-outline p-0 transition-transform', colour === c && 'scale-110 shadow-pop-sm ring-4 ring-accent-soft')}
                  style={{ background: tint(c, 55) }}
                  aria-label={c}
                  aria-pressed={colour === c}
                  onClick={() => setColour(c)}
                />
              ))}
            </div>
          </fieldset>
          {create.isError && <p className="err" role="alert">{(create.error as Error).message}</p>}
          <div className="flex justify-end gap-3">
            <DialogClose asChild><Button variant="flat">Cancel</Button></DialogClose>
            <Button type="submit" variant="primary" disabled={!name.trim() || create.isPending}>{create.isPending ? 'Creating…' : 'Create list'}</Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
