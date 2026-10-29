import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import {
  archiveList, createItem, deleteItem, getList, suggestItems, updateItem,
  type Category, type Item, type ListDetail,
} from './api';
import { parseQuickAdd } from './lists';
import { EmptyState, ErrorState, Loading } from './states';

/** Refetches this list whenever the server announces an item change (SSE). */
function useLiveList(listId: string) {
  const qc = useQueryClient();
  useEffect(() => {
    const es = new EventSource('/api/events');
    es.addEventListener('item', (e) => {
      try {
        const { listId: changed } = JSON.parse((e as MessageEvent).data);
        if (changed === listId) qc.invalidateQueries({ queryKey: ['list', listId] });
      } catch {
        qc.invalidateQueries({ queryKey: ['list', listId] });
      }
    });
    return () => es.close();
  }, [listId, qc]);
}

export function ListPage() {
  const { listId } = useParams({ from: '/authed/shell/lists/$listId' });
  const qc = useQueryClient();
  const navigate = useNavigate();
  const key = ['list', listId];
  const list = useQuery({ queryKey: key, queryFn: () => getList(listId) });
  const [toast, setToast] = useState<{ text: string; undo?: () => void }>();
  useLiveList(listId);

  useEffect(() => {
    if (!toast) return;
    const t = setTimeout(() => setToast(undefined), 5000);
    return () => clearTimeout(t);
  }, [toast]);

  const toggle = useMutation({
    mutationFn: (i: Item) => updateItem(i.id, { checked: !i.checked }),
    onMutate: async (i) => {
      await qc.cancelQueries({ queryKey: key });
      qc.setQueryData<ListDetail>(key, (d) => d && { ...d, items: d.items.map((x) => (x.id === i.id ? { ...x, checked: !i.checked } : x)) });
    },
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => deleteItem(id),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  const clearChecked = useMutation({
    mutationFn: (items: Item[]) => Promise.all(items.map((i) => deleteItem(i.id))),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  const archive = useMutation({
    mutationFn: () => archiveList(listId),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['lists'] }); navigate({ to: '/lists' }); },
  });

  if (list.isPending) return <Loading label="Loading list…" />;
  if (list.isError) return <ErrorState onRetry={() => list.refetch()} />;

  const { items, categories } = list.data;
  const active = items.filter((i) => !i.checked);
  const done = items.filter((i) => i.checked);
  const groups: { category?: Category; items: Item[] }[] = [
    ...categories.map((c) => ({ category: c, items: active.filter((i) => i.categoryId === c.id) })),
    { items: active.filter((i) => !i.categoryId || !categories.some((c) => c.id === i.categoryId)) },
  ].filter((g) => g.items.length);

  return (
    <>
      <p className="mb-2"><Link to="/lists">← Lists</Link></p>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <h1 className="page-title mb-0! mr-auto"><span aria-hidden>{list.data.emoji}</span> {list.data.name}</h1>
        <button onClick={() => { if (confirm(`Archive “${list.data.name}”? Its items are kept.`)) archive.mutate(); }}>Archive</button>
      </div>

      <AddBar
        listId={listId}
        categories={categories}
        onAdded={(item) => {
          qc.invalidateQueries({ queryKey: key });
          setToast({ text: `Added ${item.name}`, undo: () => remove.mutate(item.id) });
        }}
      />

      {items.length === 0 && <EmptyState emoji="📝" title="Nothing on this list" body="Add your first item above." />}
      {items.length > 0 && active.length === 0 && <EmptyState emoji="🎉" title="All done" body="Everything is checked off." />}

      {groups.map((g) => (
        <section key={g.category?.id ?? 'none'} className="mb-5" aria-label={g.category?.name ?? 'Other'}>
          <h2 className="mb-1 flex items-center gap-2 text-sm uppercase tracking-wide text-ink-muted">
            <span className="dot size-3!" style={{ background: g.category?.colour ?? 'var(--color-cat-slate)' }} aria-hidden />
            {g.category?.name ?? 'Other'}
          </h2>
          <ul className="m-0 list-none p-0">
            {g.items.map((i) => <ItemRow key={i.id} item={i} onToggle={() => toggle.mutate(i)} onDelete={() => remove.mutate(i.id)} />)}
          </ul>
        </section>
      ))}

      {done.length > 0 && (
        <section aria-label="Got it" className="mb-5">
          <div className="flex items-center gap-2">
            <h2 className="mb-1 mr-auto text-sm uppercase tracking-wide text-ink-muted">Got it ({done.length})</h2>
            <button className="border-0! bg-transparent! text-accent underline" onClick={() => clearChecked.mutate(done)}>Clear</button>
          </div>
          <ul className="m-0 list-none p-0">
            {done.map((i) => <ItemRow key={i.id} item={i} onToggle={() => toggle.mutate(i)} onDelete={() => remove.mutate(i.id)} />)}
          </ul>
        </section>
      )}

      {toast && (
        <div className="toast flex items-center gap-3" role="status">
          {toast.text}
          {toast.undo && <button className="min-h-0! border-0! bg-transparent! p-0! font-semibold text-accent-soft underline" onClick={() => { toast.undo?.(); setToast(undefined); }}>Undo</button>}
        </div>
      )}
    </>
  );
}

function ItemRow({ item, onToggle, onDelete }: { item: Item; onToggle: () => void; onDelete: () => void }) {
  return (
    <li className="row">
      <input
        type="checkbox"
        id={`i-${item.id}`}
        className="size-6! min-h-0! flex-none"
        checked={item.checked}
        onChange={onToggle}
        aria-label={`${item.name}${item.quantityText ? `, ${item.quantityText}` : ''}`}
      />
      <label htmlFor={`i-${item.id}`} className={`grow mb-0! cursor-pointer text-base! font-normal! ${item.checked ? 'text-ink-muted line-through' : ''}`}>
        {item.name}
        {item.quantityText && <span className="ml-2 text-sm text-ink-muted">{item.quantityText}</span>}
        {item.note && <span className="block text-xs text-ink-muted">{item.note}</span>}
      </label>
      <button className="icon-btn" aria-label={`Delete ${item.name}`} onClick={onDelete}>✕</button>
    </li>
  );
}

function AddBar({ listId, categories, onAdded }: { listId: string; categories: Category[]; onAdded: (item: Item) => void }) {
  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  const [categoryId, setCategoryId] = useState<string>();

  useEffect(() => {
    const t = setTimeout(() => setQ(parseQuickAdd(text).name), 200);
    return () => clearTimeout(t);
  }, [text]);

  const suggestions = useQuery({
    queryKey: ['suggestions', listId, q],
    queryFn: () => suggestItems(listId, q),
    enabled: q.length > 0,
  });

  const add = useMutation({
    mutationFn: (body: { name: string; quantityText?: string; categoryId?: string }) =>
      createItem(listId, { id: crypto.randomUUID(), ...body }),
    onSuccess: (item) => { setText(''); setCategoryId(undefined); onAdded(item); },
  });

  const parsed = parseQuickAdd(text);
  const submit = () => {
    if (!parsed.name) return;
    add.mutate({ name: parsed.name, quantityText: parsed.quantityText || undefined, categoryId });
  };

  return (
    <div className="mb-6">
      <form onSubmit={(e) => { e.preventDefault(); submit(); }} className="flex gap-2">
        <input
          aria-label="Add an item"
          placeholder="Add an item — try “milk 2L” or “2 cans chickpeas”"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
        <button type="submit" className="primary w-auto!" disabled={!parsed.name || add.isPending}>Add</button>
      </form>
      {parsed.name && (
        <div className="mt-2 flex flex-wrap items-center gap-2" role="group" aria-label="Category">
          {parsed.quantityText && <span className="pill text-ink!">{parsed.quantityText}</span>}
          <button type="button" className="chip" aria-pressed={!categoryId} onClick={() => setCategoryId(undefined)}>Auto</button>
          {categories.map((c) => (
            <button type="button" key={c.id} className="chip" aria-pressed={categoryId === c.id} onClick={() => setCategoryId(c.id)}>
              <span className="dot mr-1.5 inline-block size-2.5! align-middle" style={{ background: c.colour }} aria-hidden />{c.name}
            </button>
          ))}
        </div>
      )}
      {!!suggestions.data?.length && parsed.name && (
        <ul className="mt-2 list-none rounded-md border border-line bg-surface p-0" aria-label="Suggestions">
          {suggestions.data.map((s) => (
            <li key={s.name} className="row px-3">
              <span className="grow">{s.name}{s.quantityText && <span className="ml-2 text-sm text-ink-muted">{s.quantityText}</span>}</span>
              <button type="button" className="icon-btn" aria-label={`Add ${s.name}`} onClick={() => add.mutate(s)}>＋</button>
            </li>
          ))}
        </ul>
      )}
      {add.isError && <p className="err" role="alert">{(add.error as Error).message}</p>}
    </div>
  );
}
