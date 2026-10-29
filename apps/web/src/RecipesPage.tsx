import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { AddToListDialog } from './AddToListDialog';
import { apiUrl, listRecipes, listTags, setMark, type RecipeMark, type RecipeSummary } from './api';
import { formatMinutes, totalMinutes } from './recipes';
import { EmptyState, ErrorState, Loading } from './states';

export function RecipesPage() {
  const qc = useQueryClient();
  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  const [tag, setTag] = useState<string>();
  const [mark, setMarkFilter] = useState<RecipeMark>();
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [dialog, setDialog] = useState(false);
  const [toast, setToast] = useState('');

  useEffect(() => {
    const t = setTimeout(() => setQ(text.trim()), 250);
    return () => clearTimeout(t);
  }, [text]);

  const recipes = useQuery({
    queryKey: ['recipes', { q, tag, mark }],
    queryFn: () => listRecipes({ q: q || undefined, tag, mark }),
    placeholderData: (prev) => prev,
  });
  const tags = useQuery({ queryKey: ['tags'], queryFn: listTags });

  const toggleMark = useMutation({
    mutationFn: ({ r, m }: { r: RecipeSummary; m: RecipeMark }) => setMark(r.id, m, !r.marks.includes(m)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['recipes'] }),
  });

  const filtered = !!(q || tag || mark);
  const clearAll = () => { setText(''); setQ(''); setTag(undefined); setMarkFilter(undefined); };
  const chosen = recipes.data?.filter((r) => selected.has(r.id)) ?? [];
  const flash = (m: string) => { setToast(m); setTimeout(() => setToast(''), 3500); };

  return (
    <>
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <h1 className="page-title mb-0! mr-auto">Recipes</h1>
        <button onClick={() => { setSelecting(!selecting); setSelected(new Set()); }} aria-pressed={selecting}>
          {selecting ? 'Done' : 'Select'}
        </button>
        <Link to="/recipes/new" className="btn-primary">New recipe</Link>
      </div>

      <input type="search" aria-label="Search recipes" placeholder="Search by name, ingredient or tag" value={text} onChange={(e) => setText(e.target.value)} />

      <div className="my-3 flex flex-wrap items-center gap-2" role="group" aria-label="Filters">
        <Chip on={mark === 'favourite'} onClick={() => setMarkFilter(mark === 'favourite' ? undefined : 'favourite')}>♥ Favourites</Chip>
        <Chip on={mark === 'want_to_make'} onClick={() => setMarkFilter(mark === 'want_to_make' ? undefined : 'want_to_make')}>🔖 Want to make</Chip>
        {tags.data?.map((t) => (
          <Chip key={t} on={tag === t} onClick={() => setTag(tag === t ? undefined : t)}>{t}</Chip>
        ))}
        {filtered && (
          <>
            <span className="text-sm text-ink-muted" role="status">{recipes.data?.length ?? 0} found</span>
            <button className="border-0! bg-transparent! text-accent underline" onClick={clearAll}>Clear all</button>
          </>
        )}
      </div>

      {recipes.isPending && <Loading label="Loading recipes…" />}
      {recipes.isError && <ErrorState onRetry={() => recipes.refetch()} />}
      {recipes.data?.length === 0 && (
        filtered
          ? <EmptyState emoji="🔍" title="Nothing matches" body="Try a different search or clear the filters." />
          : <EmptyState emoji="🥘" title="No recipes yet" body="Ask your AI to add one, or write one yourself." />
      )}

      <ul className="m-0 grid list-none grid-cols-2 gap-4 p-0 sm:grid-cols-3 lg:grid-cols-4">
        {recipes.data?.map((r) => (
          <RecipeCard
            key={r.id}
            r={r}
            selecting={selecting}
            selected={selected.has(r.id)}
            onSelect={() => setSelected((s) => { const n = new Set(s); n.has(r.id) ? n.delete(r.id) : n.add(r.id); return n; })}
            onMark={(m) => toggleMark.mutate({ r, m })}
          />
        ))}
      </ul>

      {selecting && selected.size > 0 && (
        <div className="fixed inset-x-0 bottom-0 z-10 flex justify-center border-t border-line bg-surface p-3">
          <button className="primary max-w-sm" onClick={() => setDialog(true)}>Add to list ({selected.size})</button>
        </div>
      )}
      {dialog && (
        <AddToListDialog
          recipes={chosen}
          onClose={() => setDialog(false)}
          onDone={(m) => { setDialog(false); setSelecting(false); setSelected(new Set()); flash(m); }}
        />
      )}
      {toast && <div className="toast" role="status">{toast}</div>}
    </>
  );
}

function Chip({ on, onClick, children }: { on: boolean; onClick: () => void; children: React.ReactNode }) {
  return <button className="chip" aria-pressed={on} onClick={onClick}>{children}</button>;
}

function RecipeCard(p: { r: RecipeSummary; selecting: boolean; selected: boolean; onSelect: () => void; onMark: (m: RecipeMark) => void }) {
  const { r } = p;
  const fav = r.marks.includes('favourite');
  const want = r.marks.includes('want_to_make');
  const time = totalMinutes(r);
  const body = (
    <>
      {r.coverUrl
        ? <img src={apiUrl(r.coverUrl)} alt="" loading="lazy" className="aspect-4/3 w-full object-cover" />
        : <div className="grid aspect-4/3 w-full place-items-center bg-accent-soft text-4xl" aria-hidden>🍽️</div>}
      <div className="p-3">
        <h2 className="m-0 text-base leading-snug">{r.title}</h2>
        <p className="m-0 mt-1 text-sm text-ink-muted">{time ? formatMinutes(time) : 'No time set'}</p>
      </div>
    </>
  );
  return (
    <li className="relative overflow-hidden rounded-md border border-line bg-surface">
      {p.selecting ? (
        <button className="block! w-full border-0! bg-transparent! p-0! text-left" aria-pressed={p.selected} onClick={p.onSelect}>
          {body}
          <span className={p.selected ? 'select-badge on' : 'select-badge'} aria-hidden>{p.selected ? '✓' : ''}</span>
        </button>
      ) : (
        <>
          <Link to="/recipes/$recipeId" params={{ recipeId: r.id }} className="block text-ink no-underline">{body}</Link>
          <div className="absolute right-2 top-2 flex gap-1">
            <button className="icon-btn" aria-pressed={fav} aria-label={fav ? 'Remove from favourites' : 'Add to favourites'} onClick={() => p.onMark('favourite')}>{fav ? '♥' : '♡'}</button>
            <button className="icon-btn" aria-pressed={want} aria-label={want ? 'Remove from want to make' : 'Want to make'} onClick={() => p.onMark('want_to_make')}>{want ? '🔖' : '＋'}</button>
          </div>
        </>
      )}
    </li>
  );
}
