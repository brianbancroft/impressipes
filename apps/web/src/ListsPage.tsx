import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { createList, listLists } from './api';
import { LIST_COLOURS } from './lists';
import { EmptyState, ErrorState, Loading } from './states';

export function ListsPage() {
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  const [creating, setCreating] = useState(false);

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <h1 className="page-title mb-0! mr-auto">Lists</h1>
        <button className="btn-primary" onClick={() => setCreating(true)}>New list</button>
      </div>
      {lists.isPending && <Loading label="Loading lists…" />}
      {lists.isError && <ErrorState onRetry={() => lists.refetch()} />}
      {lists.data?.length === 0 && <EmptyState emoji="🛒" title="No lists yet" body="Make one for each shop." />}
      <ul className="m-0 grid list-none gap-3 p-0 sm:grid-cols-2 lg:grid-cols-3">
        {lists.data?.map((l) => (
          <li key={l.id}>
            <Link
              to="/lists/$listId"
              params={{ listId: l.id }}
              className="flex min-h-16 items-center gap-3 rounded-md border border-line border-l-8 bg-surface p-4 text-ink no-underline hover:bg-surface-2"
              style={{ borderLeftColor: l.colour }}
            >
              <span className="text-3xl" aria-hidden>{l.emoji}</span>
              <span className="font-display text-lg font-extrabold">{l.name}</span>
            </Link>
          </li>
        ))}
      </ul>
      {creating && <NewListDialog onClose={() => setCreating(false)} />}
    </>
  );
}

function NewListDialog({ onClose }: { onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState('');
  const [emoji, setEmoji] = useState('🛒');
  const [colour, setColour] = useState(LIST_COLOURS[0]);

  useEffect(() => ref.current?.showModal(), []);

  const create = useMutation({
    mutationFn: () => createList({ name: name.trim(), emoji, colour }),
    onSuccess: (list) => {
      qc.invalidateQueries({ queryKey: ['lists'] });
      navigate({ to: '/lists/$listId', params: { listId: list.id } });
    },
  });

  return (
    <dialog ref={ref} onClose={onClose} className="dialog" aria-labelledby="nl-title">
      <form onSubmit={(e) => { e.preventDefault(); if (name.trim()) create.mutate(); }}>
        <h2 id="nl-title" className="mb-4 text-xl">New list</h2>
        <div className="mb-4 flex gap-2">
          <div className="w-20">
            <label htmlFor="nl-emoji">Icon</label>
            <input id="nl-emoji" value={emoji} maxLength={4} onChange={(e) => setEmoji(e.target.value)} className="text-center" />
          </div>
          <div className="grow">
            <label htmlFor="nl-name">Name</label>
            <input id="nl-name" value={name} autoFocus placeholder="Costco, Weekly shop…" onChange={(e) => setName(e.target.value)} />
          </div>
        </div>
        <fieldset>
          <legend>Colour</legend>
          <div className="swatches">
            {LIST_COLOURS.map((c) => (
              <button type="button" key={c} className="swatch" style={{ background: c }} aria-label={c} aria-pressed={colour === c} onClick={() => setColour(c)} />
            ))}
          </div>
        </fieldset>
        {create.isError && <p className="err" role="alert">{(create.error as Error).message}</p>}
        <div className="flex justify-end gap-2">
          <button type="button" onClick={() => ref.current?.close()}>Cancel</button>
          <button type="submit" className="primary w-auto!" disabled={!name.trim() || create.isPending}>
            {create.isPending ? 'Creating…' : 'Create'}
          </button>
        </div>
      </form>
    </dialog>
  );
}
