import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { Bookmark, ChefHat, ChevronLeft, Clock, Flame, Heart, Minus, MoreHorizontal, Pencil, Plus, ShoppingCart, Timer, Trash2, UtensilsCrossed } from 'lucide-react';
import { useState } from 'react';
import { AddToListDialog } from './AddToListDialog';
import { apiUrl, deleteRecipe, getRecipe, listMembers, setMark, type Recipe, type RecipeMark } from './api';
import { formatMinutes, formatTimer, ingredientParts, sourceLine } from './recipes';
import { ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn, pick, STICKERS } from './ui/cn';
import { Menu, MenuContent, MenuItem, MenuSeparator, MenuTrigger } from './ui/menu';
import { Blob, QtyBadge, Sticker, Tick } from './ui/sticker';
import { Toast } from './ui/toast';

export function RecipePage() {
  const { recipeId } = useParams({ from: '/authed/shell/recipes/$recipeId' });
  const qc = useQueryClient();
  const navigate = useNavigate();
  const recipe = useQuery({ queryKey: ['recipe', recipeId], queryFn: () => getRecipe(recipeId) });
  const members = useQuery({ queryKey: ['members'], queryFn: listMembers, retry: false });
  const [servings, setServings] = useState<number>();
  const [dialog, setDialog] = useState(false);
  const [toast, setToast] = useState('');
  const [tab, setTab] = useState<'ingredients' | 'steps'>('ingredients');
  const [ticked, setTicked] = useState<Set<number>>(new Set());

  const mark = useMutation({
    mutationFn: ({ m, on }: { m: RecipeMark; on: boolean }) => setMark(recipeId, m, on),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['recipe', recipeId] }); qc.invalidateQueries({ queryKey: ['recipes'] }); },
  });
  const del = useMutation({
    mutationFn: () => deleteRecipe(recipeId),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['recipes'] }); navigate({ to: '/recipes' }); },
  });

  if (recipe.isPending) return <Loading label="Loading recipe…" />;
  if (recipe.isError) return <ErrorState onRetry={() => recipe.refetch()} />;
  const r = recipe.data;
  const base = r.servings ?? 0;
  const current = servings ?? base;
  const factor = base ? current / base : 1;
  const fav = r.marks.includes('favourite');
  const want = r.marks.includes('want_to_make');
  const who = sourceLine(r, (id) => members.data?.find((m) => m.id === id)?.name);
  const step = (d: number) => setServings(Math.max(1, Math.round((current + d) * 2) / 2));
  const toggleFav = () => mark.mutate({ m: 'favourite', on: !fav });
  const confirmDelete = () => { if (confirm(`Delete "${r.title}"? This can't be undone.`)) del.mutate(); };

  const more = (
    <Menu>
      <MenuTrigger asChild>
        <Button size="icon" aria-label="More: want to make, edit, delete" className="shadow-none"><MoreHorizontal /></Button>
      </MenuTrigger>
      <MenuContent>
        <MenuItem onSelect={() => mark.mutate({ m: 'want_to_make', on: !want })}>
          <Bookmark fill={want ? 'currentColor' : 'none'} />{want ? 'Remove from want to make' : 'Want to make'}
        </MenuItem>
        <MenuItem asChild><Link to="/recipes/$recipeId/edit" params={{ recipeId }} className="text-ink no-underline hover:text-ink"><Pencil />Edit recipe</Link></MenuItem>
        <MenuSeparator />
        <MenuItem danger onSelect={confirmDelete}><Trash2 />Delete recipe</MenuItem>
      </MenuContent>
    </Menu>
  );
  const favButton = (
    <Button size="icon" aria-pressed={fav} aria-label={fav ? 'Remove from favourites' : 'Add to favourites'} onClick={toggleFav}
      className={cn('shadow-none', fav && 'bg-accent-fill text-white hover:text-white')}>
      <Heart fill={fav ? 'currentColor' : 'none'} />
    </Button>
  );
  const actions = (
    <>
      <Button size="md" onClick={() => setDialog(true)}><ShoppingCart />Add to list</Button>
      <Button asChild variant="primary" size="md"><Link to="/recipes/$recipeId/cook" params={{ recipeId }}><ChefHat />Let's cook!</Link></Button>
    </>
  );

  return (
    <article className="relative isolate -mx-4 -mt-4 md:mx-0 md:mt-0">
      {/* Desktop top bar */}
      <div className="mb-6 hidden items-center gap-2.5 md:flex">
        <Button asChild size="md" className="rounded-full pr-5 pl-3 shadow-none"><Link to="/recipes"><ChevronLeft />Recipe box</Link></Button>
        <span className="flex-1" />
        {favButton}
        <Button asChild size="md" className="rounded-full shadow-none"><Link to="/recipes/$recipeId/edit" params={{ recipeId }}><Pencil />Edit</Link></Button>
        {more}
        <span className="w-2" />
        {actions}
      </div>

      <div className="grid gap-6 md:gap-10 lg:grid-cols-[420px_minmax(0,1fr)] lg:items-start">
        <div className="flex flex-col gap-5 lg:sticky lg:top-8">
          <Hero r={r}>
            <div className="absolute inset-x-4 top-4 flex justify-between md:hidden">
              <Button asChild size="icon" aria-label="Back to recipes" className="shadow-none"><Link to="/recipes"><ChevronLeft strokeWidth={2.6} /></Link></Button>
              <div className="flex gap-2.5">{favButton}{more}</div>
            </div>
          </Hero>
          <div className="hidden lg:block">
            <Ingredients r={r} factor={factor} ticked={ticked} setTicked={setTicked} stepper={base > 0 && <Servings current={current} base={base} step={step} />} />
          </div>
        </div>

        <div className="flex min-w-0 flex-col gap-4 px-4 md:px-0">
          <div>
            {r.tags[0] && <p className="scribble md:text-[26px]">{r.tags[0]}</p>}
            <h1 className="text-[32px] leading-[1.02] md:text-5xl lg:text-[56px] lg:leading-[0.98]">{r.title}</h1>
            {who && <p className="mt-2 text-[15px] font-medium text-ink-muted">{who}</p>}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {r.prepMinutes ? <Sticker tilt={-2}><Clock strokeWidth={2.6} />{formatMinutes(r.prepMinutes)} prep</Sticker> : null}
            {r.cookMinutes ? <Sticker tilt={1} className="bg-tangerine"><Flame strokeWidth={2.4} />{formatMinutes(r.cookMinutes)} cook</Sticker> : null}
            {r.tags.map((t, i) => <Sticker key={t} tilt={i % 2 ? 1.5 : -1} className={pick(STICKERS.slice(2), t)}>{t}</Sticker>)}
          </div>
          {r.summary && <p className="max-w-[65ch] text-[17px] leading-relaxed">{r.summary}</p>}

          {/* Phone: servings + Ingredients / Steps toggle */}
          <div className="flex flex-col gap-4 lg:hidden">
            {base > 0 && (
              <div className="flex items-center justify-between rounded-[18px] border-[2.5px] border-outline bg-surface py-1.5 pr-1.5 pl-4">
                <span className="font-bold">Serves</span>
                <Servings current={current} base={base} step={step} big />
              </div>
            )}
            <div role="tablist" aria-label="Recipe sections" className="grid grid-cols-2 gap-1 rounded-md border-[2.5px] border-outline bg-surface p-1">
              {(['ingredients', 'steps'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  role="tab"
                  aria-selected={tab === t}
                  onClick={() => setTab(t)}
                  className={cn('min-h-10.5 cursor-pointer rounded-sm border-0 font-bold', tab === t ? 'bg-rail text-on-rail' : 'bg-transparent text-ink')}
                >
                  {t === 'ingredients' ? `Ingredients · ${r.ingredients.length}` : `Steps · ${r.steps.length}`}
                </button>
              ))}
            </div>
            {tab === 'ingredients' && <Ingredients r={r} factor={factor} ticked={ticked} setTicked={setTicked} bare />}
          </div>

          <section aria-labelledby="steps" className={cn('mt-2', tab !== 'steps' && 'max-lg:hidden')}>
            <h2 id="steps" className="mb-4 text-2xl max-lg:sr-only">Steps <span className="font-semibold text-ink-muted">· {r.steps.length}</span></h2>
            {r.steps.length === 0 && <p className="text-ink-muted">No steps yet.</p>}
            <ol className="m-0 grid list-none gap-4 p-0">
              {r.steps.map((s, n) => (
                <li key={n} className="flex gap-4">
                  <span className={cn('blob-d grid size-10 flex-none place-items-center border-[2.5px] border-outline font-display text-lg font-extrabold text-sticker-ink', pick(STICKERS, String(n)))} aria-hidden>{n + 1}</span>
                  <p className="m-0 pt-1.5 text-[17px] leading-relaxed">
                    {s.text}
                    {s.timerSeconds && <Sticker tilt={-2} className="ml-2 py-0.5 align-middle"><Timer strokeWidth={2.6} />{formatTimer(s.timerSeconds)}</Sticker>}
                  </p>
                </li>
              ))}
            </ol>
            {r.notes && (
              <div className="mt-8 rotate-[0.5deg] rounded-lg border-[2.5px] border-outline bg-butter p-5 text-sticker-ink shadow-pop">
                <h2 className="mb-1 font-hand text-[28px] font-bold">Notes</h2>
                <p className="whitespace-pre-wrap">{r.notes}</p>
              </div>
            )}
          </section>
        </div>
      </div>

      {/* Phone action bar */}
      <div className="fixed inset-x-0 bottom-0 z-20 grid grid-cols-[1fr_1.25fr] gap-3 border-t-[2.5px] border-outline bg-bg px-4 pt-3.5 pb-5 md:hidden [&>*]:min-h-13.5">
        {actions}
      </div>
      <div className="h-20 md:hidden" aria-hidden />

      {dialog && (
        <AddToListDialog
          recipes={[{ id: r.id, title: r.title, servings: current || undefined }]}
          onClose={() => setDialog(false)}
          onDone={(m) => { setDialog(false); setToast(m); setTimeout(() => setToast(''), 3500); }}
        />
      )}
      {toast && <Toast>{toast}</Toast>}
    </article>
  );
}

function Hero({ r, children }: { r: Recipe; children?: React.ReactNode }) {
  const img = r.images[0];
  return (
    <div className="relative h-72 overflow-hidden border-b-[2.5px] border-outline bg-butter md:h-60 md:rounded-xl md:border-[2.5px] md:shadow-pop-lg">
      {img ? (
        <img src={apiUrl(img.largeUrl)} alt="" className="absolute inset-0 size-full object-cover" />
      ) : (
        <>
          <Blob className="-bottom-16 -left-10 h-44 w-52 bg-tangerine" />
          <Blob shape="blob-b" className="-top-5 -right-8 size-32 bg-rose" />
          <span className="blob-c absolute top-1/2 left-1/2 grid h-44 w-48 -translate-1/2 place-items-center border-[3px] border-outline bg-white text-sticker-ink shadow-pop-lg" aria-hidden>
            <UtensilsCrossed className="size-20" strokeWidth={1.4} />
          </span>
        </>
      )}
      {children}
    </div>
  );
}

function Servings({ current, base, step, big }: { current: number; base: number; step: (d: number) => void; big?: boolean }) {
  const btn = cn('grid cursor-pointer place-items-center border-outline p-0 text-sticker-ink', big ? 'size-11 rounded-[14px] border-2' : 'size-8.5 rounded-[10px]');
  return (
    <div className={cn('flex items-center gap-1', !big && 'rounded-[14px] border-2 border-outline bg-bg p-0.5')} role="group" aria-label="Servings">
      <button type="button" aria-label="Fewer servings" onClick={() => step(-1)} className={cn(btn, big ? 'bg-bg text-ink' : 'border-0 bg-transparent text-ink')}><Minus className="size-4.5" strokeWidth={2.8} /></button>
      <span className="min-w-14 text-center leading-none" aria-live="polite">
        {!big && <span className="text-sm font-bold">serves </span>}
        <b className="font-display text-xl font-extrabold">{current}</b>
        {current !== base && <small className="block text-[11px] font-bold text-ink-muted">makes {base}</small>}
      </span>
      <button type="button" aria-label="More servings" onClick={() => step(1)} className={cn(btn, 'border-2 bg-mint')}><Plus className="size-4.5" strokeWidth={2.8} /></button>
    </div>
  );
}

function Ingredients({ r, factor, ticked, setTicked, stepper, bare }: {
  r: Recipe; factor: number; ticked: Set<number>; setTicked: (s: Set<number>) => void; stepper?: React.ReactNode; bare?: boolean;
}) {
  const toggle = (n: number) => { const s = new Set(ticked); s.has(n) ? s.delete(n) : s.add(n); setTicked(s); };
  return (
    <section aria-label="Ingredients" className={cn(!bare && 'rounded-lg border-[2.5px] border-outline bg-surface px-4.5 pt-3.5 pb-2')}>
      {!bare && (
        <div className="flex items-center justify-between pb-1.5">
          <h2 className="text-2xl">Ingredients <span className="font-semibold text-ink-muted">· {r.ingredients.length}</span></h2>
          {stepper}
        </div>
      )}
      {r.ingredients.length === 0 && <p className="py-3 text-ink-muted">No ingredients yet.</p>}
      <ul className="m-0 list-none p-0">
        {r.ingredients.map((i, n) => {
          const p = ingredientParts(i, factor);
          const on = ticked.has(n);
          return (
            <li key={n}>
              {i.section && i.section !== r.ingredients[n - 1]?.section && <h3 className="mt-4 mb-1 eyebrow">{i.section}</h3>}
              <button type="button" aria-pressed={on} onClick={() => toggle(n)} className="dashed-row flex min-h-12 w-full cursor-pointer items-center gap-3 border-x-0 border-t-0 bg-transparent py-1.5 text-left text-ink">
                <Tick checked={on} className="size-6" />
                {p.qty ? <QtyBadge qty={p.qty} unit={p.unit} className={cn(on && 'opacity-50')} /> : null}
                <span className={cn('min-w-0 flex-1 text-base break-words uppercase', on && 'text-ink-muted line-through decoration-accent-fill decoration-2')}>{p.text}</span>
              </button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
