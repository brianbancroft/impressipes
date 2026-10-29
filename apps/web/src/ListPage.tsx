import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import {
  archiveList, createItem, deleteItem, getList, suggestItems, updateItem,
  type Category, type Item, type ListDetail,
} from './api';
import { parseQuickAdd } from './lists';
import { EmptyState, ErrorState, Loading } from './states';
import { Archive, ChevronDown, ChevronLeft, Eraser, MoreHorizontal, NotebookPen, PartyPopper, Plus, ShoppingBasket, X } from 'lucide-react';
import { ListTiles, NewListButton, NewListDialog } from './ListsPage';
import { splitQuantity } from './recipes';
import { Button } from './ui/button';
import { cn, tint } from './ui/cn';
import { Menu, MenuContent, MenuItem, MenuTrigger } from './ui/menu';
import { Chip, QtyBadge, Sticker, Tick } from './ui/sticker';
import { Toast } from './ui/toast';

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
  const [basket, setBasket] = useState(false);
  const [creating, setCreating] = useState(false);
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
    <div className="grid gap-8 xl:grid-cols-[280px_minmax(0,1fr)] xl:items-start">
      <aside className="hidden flex-col gap-3.5 xl:flex" aria-label="Your lists">
        <p className="scribble">your lists</p>
        <ListTiles activeId={listId} compact />
        <NewListButton onClick={() => setCreating(true)} />
      </aside>

      <div className="flex min-w-0 flex-col gap-5">
        <div
          className="-mx-4 -mt-4 flex flex-col gap-4 border-b-[2.5px] border-outline px-4 pt-4 pb-5 text-sticker-ink md:mx-0 md:mt-0 md:rounded-xl md:border-[2.5px] md:p-5 md:shadow-pop"
          style={{ background: tint(list.data.colour) }}
        >
          <div className="flex items-center gap-3">
            <Button asChild size="icon" aria-label="Back to lists" className="shadow-none xl:hidden"><Link to="/lists"><ChevronLeft strokeWidth={2.6} /></Link></Button>
            <span className="flex-1" />
            <Menu>
              <MenuTrigger asChild><Button size="icon" aria-label="List options" className="shadow-none"><MoreHorizontal /></Button></MenuTrigger>
              <MenuContent>
                {done.length > 0 && <MenuItem onSelect={() => clearChecked.mutate(done)}><Eraser />Clear the basket ({done.length})</MenuItem>}
                <MenuItem danger onSelect={() => { if (confirm(`Archive “${list.data.name}”? Its items are kept.`)) archive.mutate(); }}><Archive />Archive list</MenuItem>
              </MenuContent>
            </Menu>
          </div>
          <div className="flex flex-wrap items-end gap-x-4 gap-y-1">
            <h1 className="text-[34px] leading-none md:text-[40px]"><span aria-hidden>{list.data.emoji}</span> {list.data.name}</h1>
            {active.length > 0 && <span className="-rotate-3 font-hand text-2xl">{active.length} to go!</span>}
          </div>
          <AddBar
            listId={listId}
            categories={categories}
            onAdded={(item) => {
              qc.invalidateQueries({ queryKey: key });
              setToast({ text: `Added ${item.name}`, undo: () => remove.mutate(item.id) });
            }}
          />
        </div>

        {items.length === 0 && <EmptyState icon={NotebookPen} title="Nothing on this list" body="Add your first item above." />}
        {items.length > 0 && active.length === 0 && <EmptyState icon={PartyPopper} fill="bg-mint" title="All done" body="Everything is in the basket." />}

        <div className="grid items-start gap-5 lg:grid-cols-2">
          {groups.map((g, n) => (
            <section key={g.category?.id ?? 'none'} className="flex flex-col gap-2" aria-label={g.category?.name ?? 'Other'}>
              <h2 className="self-start">
                <Sticker tilt={n % 2 ? 1.5 : -2} className="text-[13px] font-extrabold tracking-wide uppercase" style={{ background: tint(g.category?.colour ?? '#7e8896', 45) }}>
                  {g.category?.name ?? 'Other'}
                </Sticker>
              </h2>
              <ul className="m-0 list-none rounded-[18px] border-[2.5px] border-outline bg-surface px-3.5 py-0">
                {g.items.map((i) => <ItemRow key={i.id} item={i} onToggle={() => toggle.mutate(i)} onDelete={() => remove.mutate(i.id)} />)}
              </ul>
            </section>
          ))}
        </div>

        {done.length > 0 && (
          <section aria-label="In the basket" className="flex flex-col gap-2">
            <button
              type="button"
              aria-expanded={basket}
              onClick={() => setBasket(!basket)}
              className="flex min-h-13 cursor-pointer items-center gap-2.5 rounded-[18px] border-[2.5px] border-dashed border-outline bg-transparent px-3.5 font-bold text-ink"
            >
              <span className="grid size-7 place-items-center rounded-full border-2 border-outline bg-accent-fill text-white" aria-hidden><ShoppingBasket className="size-4" strokeWidth={2.6} /></span>
              <span className="flex-1 text-left">In the basket · {done.length}</span>
              <span className="font-hand text-xl text-accent max-sm:hidden">nice work!</span>
              <ChevronDown className={cn('size-5 transition-transform', basket && 'rotate-180')} aria-hidden />
            </button>
            {basket && (
              <>
                <ul className="m-0 list-none rounded-[18px] border-[2.5px] border-outline bg-surface px-3.5 py-0">
                  {done.map((i) => <ItemRow key={i.id} item={i} onToggle={() => toggle.mutate(i)} onDelete={() => remove.mutate(i.id)} />)}
                </ul>
                <Button variant="link" className="self-end" onClick={() => clearChecked.mutate(done)}>Clear the basket</Button>
              </>
            )}
          </section>
        )}
      </div>

      <NewListDialog open={creating} onOpenChange={setCreating} />
      {toast && (
        <Toast>
          {toast.text}
          {toast.undo && <button type="button" className="cursor-pointer border-0 bg-transparent p-0 font-bold text-butter underline" onClick={() => { toast.undo?.(); setToast(undefined); }}>Undo</button>}
        </Toast>
      )}
    </div>
  );
}

function ItemRow({ item, onToggle, onDelete }: { item: Item; onToggle: () => void; onDelete: () => void }) {
  const q = splitQuantity(item.quantityText);
  return (
    <li className="dashed-row group flex min-h-15 items-center gap-3">
      <button
        type="button"
        role="checkbox"
        aria-checked={item.checked}
        onClick={onToggle}
        aria-label={`${item.name}${item.quantityText ? `, ${item.quantityText}` : ''}`}
        className="flex min-w-0 flex-1 cursor-pointer items-center gap-3 border-0 bg-transparent py-2 text-left text-ink"
      >
        <Tick checked={item.checked} />
        <QtyBadge qty={q.qty} unit={q.unit} className={cn(item.checked && 'opacity-50')} />
        <span className="min-w-0 flex-1">
          <span className={cn('block text-[17px] font-bold', item.checked && 'text-ink-muted line-through decoration-accent-fill decoration-2')}>{item.name}</span>
          {item.note && <span className="block text-xs font-medium text-ink-muted">{item.note}</span>}
        </span>
      </button>
      <button
        type="button"
        aria-label={`Remove ${item.name}`}
        onClick={onDelete}
        className="grid size-9 flex-none cursor-pointer place-items-center rounded-full border-2 border-transparent bg-transparent text-ink-muted hover:border-outline hover:text-ink md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100"
      >
        <X className="size-4" strokeWidth={2.6} />
      </button>
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
  const preview = splitQuantity(parsed.quantityText);

  return (
    <div className="relative flex flex-col gap-2.5">
      <form onSubmit={(e) => { e.preventDefault(); submit(); }} className="flex min-h-14 items-center gap-2 rounded-md border-[2.5px] border-outline bg-surface py-1.5 pr-1.5 pl-4 text-ink shadow-pop-sm">
        <label htmlFor="quick-add" className="sr-only">Add an item</label>
        <input
          id="quick-add"
          placeholder="Try “2 L milk” or “3 cans tomatoes”"
          value={text}
          onChange={(e) => setText(e.target.value)}
          autoComplete="off"
          className="min-w-0 flex-1 border-0 bg-transparent text-[17px] font-medium text-ink outline-none placeholder:text-ink-muted"
        />
        <span className="text-[13px] font-medium whitespace-nowrap text-ink-muted max-md:hidden"><kbd>⏎</kbd> to add</span>
        <Button type="submit" variant="primary" size="sm" className="bg-accent-fill text-white shadow-none" disabled={!parsed.name || add.isPending}>
          <Plus strokeWidth={2.8} /><span className="max-sm:sr-only">Add</span>
        </Button>
      </form>
      {parsed.name && (
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Aisle">
          <span className="flex items-center gap-2 rounded-full border-2 border-outline bg-surface py-1 pr-3 pl-1 text-sm font-bold text-ink" aria-label="Preview">
            <QtyBadge qty={preview.qty} unit={preview.unit} className="w-auto px-2 py-1" />{parsed.name}
          </span>
          <Chip on={!categoryId} onClick={() => setCategoryId(undefined)}>Auto aisle</Chip>
          {categories.map((c) => (
            <Chip key={c.id} on={categoryId === c.id} onClick={() => setCategoryId(c.id)}>
              <span className="size-2.5 rounded-full border border-outline" style={{ background: c.colour }} aria-hidden />{c.name}
            </Chip>
          ))}
        </div>
      )}
      {!!suggestions.data?.length && parsed.name && (
        <ul className="m-0 list-none rounded-md border-[2.5px] border-outline bg-surface px-3.5 py-0 text-ink" aria-label="Suggestions">
          {suggestions.data.map((s) => (
            <li key={s.name} className="dashed-row flex min-h-12 items-center gap-3">
              <span className="min-w-0 flex-1 font-bold">{s.name}{s.quantityText && <span className="ml-2 text-sm font-medium text-ink-muted">{s.quantityText}</span>}</span>
              <Button size="icon-sm" variant="sticker" aria-label={`Add ${s.name}`} onClick={() => add.mutate(s)}><Plus strokeWidth={2.8} /></Button>
            </li>
          ))}
        </ul>
      )}
      {add.isError && <p className="err" role="alert">{(add.error as Error).message}</p>}
    </div>
  );
}
