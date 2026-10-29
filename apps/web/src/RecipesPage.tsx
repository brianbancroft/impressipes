import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import { Bookmark, Check, Heart, Plus, Search, Soup, UtensilsCrossed } from 'lucide-react';
import { useEffect, useRef, useState } from 'react';
import { AddToListDialog } from './AddToListDialog';
import { apiUrl, getMe, listRecipes, listTags, setMark, type RecipeMark, type RecipeSummary } from './api';
import { formatMinutes, totalMinutes } from './recipes';
import { EmptyState, ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn, pick, STICKERS } from './ui/cn';
import { Blob, Chip } from './ui/sticker';
import { Toast } from './ui/toast';

const TILTS = ['-rotate-1', 'rotate-1', 'rotate-0', '-rotate-[0.5deg]'];

export function RecipesPage() {
  const qc = useQueryClient();
  const navigate = useNavigate({ from: '/recipes' });
  const { mark } = useSearch({ from: '/authed/shell/recipes' });
  const me = useQuery({ queryKey: ['me'], queryFn: getMe, staleTime: 60_000 });
  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  const [tag, setTag] = useState<string>();
  const [selecting, setSelecting] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [dialog, setDialog] = useState(false);
  const [toast, setToast] = useState('');
  const search = useRef<HTMLInputElement>(null);

  useEffect(() => {
    const t = setTimeout(() => setQ(text.trim()), 250);
    return () => clearTimeout(t);
  }, [text]);

  // "/" focuses search, unless you're already typing somewhere.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const el = e.target as HTMLElement;
      if (e.key === '/' && !['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName) && !el.isContentEditable) {
        e.preventDefault();
        search.current?.focus();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, []);

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

  const setMarkFilter = (m?: RecipeMark) => navigate({ search: m ? { mark: m } : {} });
  const filtered = !!(q || tag || mark);
  const clearAll = () => { setText(''); setQ(''); setTag(undefined); setMarkFilter(undefined); };
  const chosen = recipes.data?.filter((r) => selected.has(r.id)) ?? [];
  const flash = (m: string) => { setToast(m); setTimeout(() => setToast(''), 3500); };
  const name = me.data?.kind === 'member' ? me.data.member.name : '';

  return (
    <div className="relative isolate">
      <Blob className="-top-24 -right-28 size-72 bg-butter md:-top-36 md:size-96" />

      <div className="relative mb-5 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between lg:gap-8">
        <div>
          <p className="scribble md:text-[28px]">{mark === 'favourite' ? 'the good stuff' : `hey ${name || 'there'}!`}</p>
          <h1 className="text-[34px] leading-[1.02] md:text-5xl">
            {mark === 'favourite' ? 'Family faves' : 'What are we cooking tonight?'}
          </h1>
        </div>
        <label className="flex min-h-13 w-full flex-none items-center gap-2.5 rounded-[18px] border-[2.5px] border-outline bg-surface px-4 shadow-pop-sm lg:w-100">
          <Search className="size-5.5 flex-none" strokeWidth={2.4} aria-hidden />
          <span className="sr-only">Search recipes</span>
          <input
            ref={search}
            type="search"
            placeholder="Search recipes, ingredients, tags"
            value={text}
            onChange={(e) => setText(e.target.value)}
            className="min-w-0 flex-1 border-0 bg-transparent py-3 text-base font-medium text-ink outline-none placeholder:text-ink-muted"
          />
          <kbd className="max-md:hidden" aria-hidden>/</kbd>
        </label>
      </div>

      <div className="relative -mx-4 mb-6 flex items-center gap-2 overflow-x-auto px-4 py-1 md:mx-0 md:flex-wrap md:overflow-visible md:px-0" role="group" aria-label="Filters">
        <Chip on={!filtered} onClick={clearAll}>All{!filtered && recipes.data ? ` ${recipes.data.length}` : ''}</Chip>
        <Chip on={mark === 'favourite'} fill="bg-rose" onClick={() => setMarkFilter(mark === 'favourite' ? undefined : 'favourite')}><Heart />Faves</Chip>
        <Chip on={mark === 'want_to_make'} fill="bg-mint" onClick={() => setMarkFilter(mark === 'want_to_make' ? undefined : 'want_to_make')}><Bookmark />Want to make</Chip>
        {tags.data?.map((t) => (
          <Chip key={t} on={tag === t} fill={pick(STICKERS, t)} onClick={() => setTag(tag === t ? undefined : t)}>{t}</Chip>
        ))}
        <span className="ml-auto flex flex-none items-center gap-3 pl-2">
          {filtered && <span className="text-sm font-bold text-ink-muted" role="status">{recipes.data?.length ?? 0} found</span>}
          <Button size="sm" variant={selecting ? 'primary' : 'flat'} aria-pressed={selecting} onClick={() => { setSelecting(!selecting); setSelected(new Set()); }}>
            {selecting ? 'Done' : 'Select'}
          </Button>
        </span>
      </div>

      {recipes.isPending && <Loading label="Loading recipes…" />}
      {recipes.isError && <ErrorState onRetry={() => recipes.refetch()} />}
      {recipes.data?.length === 0 && (
        filtered
          ? <EmptyState icon={Search} fill="bg-sky" title="Nothing matches" body="Try a different search or clear the filters."><Button variant="link" onClick={clearAll}>Clear all</Button></EmptyState>
          : (
            <EmptyState icon={Soup} title="No recipes yet" body="Ask your AI to add one, or write one yourself.">
              <Button asChild variant="primary"><Link to="/recipes/new"><Plus />New recipe</Link></Button>
            </EmptyState>
          )
      )}

      <ul className="relative m-0 grid list-none grid-cols-2 gap-4 p-0 md:grid-cols-3 md:gap-6 xl:grid-cols-4">
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
        <div className="fixed inset-x-4 bottom-28 z-30 flex justify-center md:bottom-8 md:left-28 lg:left-66">
          <Button variant="primary" size="lg" onClick={() => setDialog(true)}>Add {selected.size} to a list</Button>
        </div>
      )}
      {dialog && (
        <AddToListDialog
          recipes={chosen}
          onClose={() => setDialog(false)}
          onDone={(m) => { setDialog(false); setSelecting(false); setSelected(new Set()); flash(m); }}
        />
      )}
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}

function RecipeCard(p: { r: RecipeSummary; selecting: boolean; selected: boolean; onSelect: () => void; onMark: (m: RecipeMark) => void }) {
  const { r } = p;
  const fav = r.marks.includes('favourite');
  const want = r.marks.includes('want_to_make');
  const time = totalMinutes(r);
  const body = (
    <>
      <div className={cn('relative grid aspect-[4/3] place-items-center overflow-hidden border-b-[2.5px] border-outline', pick(STICKERS, r.id))}>
        {r.coverUrl
          ? <img src={apiUrl(r.coverUrl)} alt="" loading="lazy" className="absolute inset-0 size-full object-cover" />
          : (
            <span className={cn('grid size-[58%] max-w-28 place-items-center border-[2.5px] border-outline bg-white text-sticker-ink', pick(['blob-a', 'blob-b', 'blob-c', 'blob-d'], r.id))} aria-hidden>
              <UtensilsCrossed className="size-1/2" strokeWidth={1.8} />
            </span>
          )}
      </div>
      <div className="flex flex-col gap-1 px-3 pt-2.5 pb-3 md:px-3.5 md:pb-3.5">
        <h2 className="text-[17px] leading-[1.1] md:text-[19px]">{r.title}</h2>
        <p className="text-[13px] font-medium text-ink-muted md:text-sm">{time ? formatMinutes(time) : 'No time set'}</p>
      </div>
    </>
  );
  return (
    <li
      className={cn(
        'group relative overflow-hidden rounded-lg border-[2.5px] border-outline bg-surface shadow-pop transition-transform duration-200 hover:z-10 hover:-translate-y-1 hover:rotate-0 hover:shadow-pop-lg',
        pick(TILTS, r.id),
        p.selected && 'ring-4 ring-accent-fill',
      )}
    >
      {p.selecting ? (
        <button type="button" className="block w-full cursor-pointer border-0 bg-transparent p-0 text-left text-ink" aria-pressed={p.selected} onClick={p.onSelect}>
          {body}
          <span className={cn('absolute top-2 right-2 grid size-9 place-items-center rounded-full border-2 border-outline', p.selected ? 'bg-mint text-sticker-ink' : 'bg-surface')} aria-hidden>
            {p.selected && <Check className="size-5" strokeWidth={3.2} />}
          </span>
        </button>
      ) : (
        <>
          <Link to="/recipes/$recipeId" params={{ recipeId: r.id }} className="block text-ink no-underline hover:text-ink">{body}</Link>
          <div className="absolute top-2 right-2 flex gap-1.5">
            <MarkButton on={want} label={want ? 'Remove from want to make' : 'Want to make'} onClick={() => p.onMark('want_to_make')} fill="bg-mint" icon={Bookmark} />
            <MarkButton on={fav} label={fav ? 'Remove from favourites' : 'Add to favourites'} onClick={() => p.onMark('favourite')} fill="bg-accent-fill text-white" icon={Heart} />
          </div>
        </>
      )}
    </li>
  );
}

function MarkButton({ on, label, onClick, fill, icon: Icon }: { on: boolean; label: string; onClick: () => void; fill: string; icon: typeof Heart }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      aria-label={label}
      onClick={onClick}
      className={cn(
        'grid size-9 cursor-pointer place-items-center rounded-full border-2 border-outline text-sticker-ink transition hover:scale-110',
        on ? fill : 'bg-white/90 opacity-100 md:opacity-0 md:group-focus-within:opacity-100 md:group-hover:opacity-100',
      )}
    >
      <Icon className="size-4" strokeWidth={2.4} fill={on ? 'currentColor' : 'none'} />
    </button>
  );
}
