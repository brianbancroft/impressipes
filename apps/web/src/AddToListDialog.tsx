import { useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { addRecipeToList, listLists } from './api';

type Props = {
  recipes: { id: string; title: string; servings?: number }[];
  onClose: () => void;
  onDone: (message: string) => void;
};

/** Adds one or more recipes to a list. A single recipe can be scaled to a different yield. */
export function AddToListDialog({ recipes, onClose, onDone }: Props) {
  const ref = useRef<HTMLDialogElement>(null);
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  const [listId, setListId] = useState('');
  const single = recipes.length === 1 ? recipes[0] : null;
  const [servings, setServings] = useState(single?.servings ?? 0);

  useEffect(() => ref.current?.showModal(), []);
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

  return (
    <dialog ref={ref} onClose={onClose} className="dialog" aria-labelledby="atl-title">
      <h2 id="atl-title" className="mb-1 text-xl">Add to list</h2>
      <p className="sub">{single ? single.title : `${recipes.length} recipes`}</p>
      {lists.isPending && <p className="sub" role="status">Loading lists…</p>}
      {lists.isError && <p className="err" role="alert">Could not load lists.</p>}
      {lists.data?.length === 0 && <p className="sub">Create a list first (on your phone or in Lists).</p>}
      {!!lists.data?.length && (
        <fieldset>
          <legend>List</legend>
          <div className="grid gap-2">
            {lists.data.map((l) => (
              <label key={l.id} className="choice">
                <input type="radio" name="list" className="size-auto! min-h-0!" checked={listId === l.id} onChange={() => setListId(l.id)} />
                <span aria-hidden>{l.emoji}</span> {l.name}
              </label>
            ))}
          </div>
        </fieldset>
      )}
      {single?.servings ? (
        <div className="mb-4">
          <label htmlFor="atl-servings">Servings (recipe makes {single.servings})</label>
          <input id="atl-servings" type="number" min={1} step="any" value={servings} onChange={(e) => setServings(Number(e.target.value))} />
        </div>
      ) : null}
      {add.isError && <p className="err" role="alert">{(add.error as Error).message}</p>}
      <div className="flex justify-end gap-2">
        <button onClick={() => ref.current?.close()}>Cancel</button>
        <button className="primary w-auto!" disabled={!listId || add.isPending} onClick={() => add.mutate()}>
          {add.isPending ? 'Adding…' : 'Add'}
        </button>
      </div>
    </dialog>
  );
}
