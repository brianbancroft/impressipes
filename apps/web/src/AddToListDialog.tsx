import { useMutation, useQuery } from '@tanstack/react-query';
import { Minus, Plus, ShoppingCart } from 'lucide-react';
import { useEffect, useState } from 'react';
import { addRecipeToList, listLists } from './api';
import { Button } from './ui/button';
import { cn, tint } from './ui/cn';
import { Dialog, DialogClose, DialogContent } from './ui/dialog';
import { Tick } from './ui/sticker';

type Props = {
  recipes: { id: string; title: string; servings?: number }[];
  onClose: () => void;
  onDone: (message: string) => void;
};

/** Adds one or more recipes to a list. A single recipe can be scaled to a different yield. */
export function AddToListDialog({ recipes, onClose, onDone }: Props) {
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  const [listId, setListId] = useState('');
  const single = recipes.length === 1 ? recipes[0] : null;
  const [servings, setServings] = useState(single?.servings ?? 0);

  useEffect(() => {
    if (!listId && lists.data?.length) setListId(lists.data[0].id);
  }, [lists.data, listId]);

  const add = useMutation({
    mutationFn: async () => {
      let added = 0;
      let merged = 0;
      for (const r of recipes) {
        const res = await addRecipeToList(listId, r.id, single && servings ? servings : undefined);
        added += res.added.length;
        merged += res.merged.length;
      }
      return { added, merged };
    },
    onSuccess: ({ added, merged }) => {
      const list = lists.data?.find((l) => l.id === listId);
      onDone(`Added ${added} item${added === 1 ? '' : 's'}${merged ? `, merged ${merged}` : ''} to ${list?.emoji ?? ''} ${list?.name ?? 'list'}`);
    },
  });

  const step = (d: number) => setServings((v) => Math.max(1, Math.round((v + d) * 2) / 2));

  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent title="Add to list" description={single ? single.title : `${recipes.length} recipes`}>
        {lists.isPending && <p className="text-ink-muted" role="status">Loading lists…</p>}
        {lists.isError && <p className="err" role="alert">Could not load lists.</p>}
        {lists.data?.length === 0 && <p className="text-ink-muted">Create a list first, in Lists.</p>}
        {!!lists.data?.length && (
          <fieldset className="m-0 mb-5 border-0 p-0">
            <legend className="field-label">Which list?</legend>
            <div className="grid gap-2.5">
              {lists.data.map((l) => {
                const on = listId === l.id;
                return (
                  <label
                    key={l.id}
                    className={cn('flex min-h-14 cursor-pointer items-center gap-3 rounded-md border-[2.5px] border-outline px-3 font-bold text-sticker-ink transition-shadow', on ? 'shadow-pop' : 'opacity-85')}
                    style={{ background: tint(l.colour, on ? 45 : 22) }}
                  >
                    <input type="radio" name="list" className="sr-only" checked={on} onChange={() => setListId(l.id)} />
                    <span className="blob-d grid size-9 flex-none place-items-center border-2 border-outline bg-white text-lg" aria-hidden>{l.emoji}</span>
                    <span className="flex-1 font-display text-lg font-extrabold">{l.name}</span>
                    <Tick checked={on} />
                  </label>
                );
              })}
            </div>
          </fieldset>
        )}
        {single?.servings ? (
          <div className="mb-5 flex items-center justify-between rounded-[18px] border-[2.5px] border-outline bg-surface py-1.5 pr-1.5 pl-4">
            <span className="font-bold">Servings <span className="text-sm font-medium text-ink-muted">(makes {single.servings})</span></span>
            <span className="flex items-center gap-1" role="group" aria-label="Servings">
              <Button size="icon-sm" variant="flat" aria-label="Fewer servings" onClick={() => step(-1)} className="rounded-[14px]"><Minus strokeWidth={2.8} /></Button>
              <span className="min-w-10 text-center font-display text-xl font-extrabold" aria-live="polite">{servings}</span>
              <Button size="icon-sm" variant="flat" aria-label="More servings" onClick={() => step(1)} className="rounded-[14px] bg-mint text-sticker-ink hover:bg-mint hover:text-sticker-ink"><Plus strokeWidth={2.8} /></Button>
            </span>
          </div>
        ) : null}
        {add.isError && <p className="err mb-3" role="alert">{(add.error as Error).message}</p>}
        <div className="flex justify-end gap-3">
          <DialogClose asChild><Button variant="flat">Cancel</Button></DialogClose>
          <Button variant="primary" disabled={!listId || add.isPending} onClick={() => add.mutate()}>
            <ShoppingCart />{add.isPending ? 'Adding…' : 'Add to list'}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
