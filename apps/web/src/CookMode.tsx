import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { getRecipe } from './api';
import { ArrowLeft, ArrowRight, Timer, X } from 'lucide-react';
import { formatTimer, ingredientParts } from './recipes';
import { Button } from './ui/button';
import { cn } from './ui/cn';
import { Dialog, DialogContent } from './ui/dialog';
import { Blob, QtyBadge, Tick } from './ui/sticker';
import { ErrorState, Loading } from './states';

/** Step 0 is always "gather the ingredients"; recipe step n is cook step n + 1. `got` holds ticked ingredient indexes. */
type Progress = { step: number; done: number[]; got: number[]; timers: { id: string; label: string; endsAt: number }[]; all: boolean };
const empty: Progress = { step: 0, done: [], got: [], timers: [], all: false };
// v2: step indexes shifted by one when the gather step was added.
const key = (id: string) => `imp-cook-v2-${id}`;

function load(id: string): Progress | null {
  try {
    const raw = localStorage.getItem(key(id));
    return raw ? { ...empty, ...JSON.parse(raw) } : null;
  } catch { return null; }
}

/** Keep the screen awake while cooking. Silently does nothing where unsupported. */
function useWakeLock() {
  useEffect(() => {
    let lock: WakeLockSentinel | undefined;
    const request = () => navigator.wakeLock?.request('screen').then((l) => (lock = l)).catch(() => {});
    request();
    const onVisible = () => document.visibilityState === 'visible' && request();
    document.addEventListener('visibilitychange', onVisible);
    return () => { document.removeEventListener('visibilitychange', onVisible); lock?.release().catch(() => {}); };
  }, []);
}

export function CookMode() {
  const { recipeId } = useParams({ from: '/authed/recipes/$recipeId/cook' });
  const recipe = useQuery({ queryKey: ['recipe', recipeId], queryFn: () => getRecipe(recipeId) });
  const saved = useRef(load(recipeId));
  const [resume, setResume] = useState(!!saved.current && (saved.current.step > 0 || saved.current.done.length > 0 || saved.current.got.length > 0 || saved.current.timers.length > 0));
  const [p, setP] = useState<Progress>(saved.current ?? empty);
  const [now, setNow] = useState(Date.now());
  const [panel, setPanel] = useState(false);
  useWakeLock();

  useEffect(() => { try { localStorage.setItem(key(recipeId), JSON.stringify(p)); } catch {} }, [p, recipeId]);
  useEffect(() => { const t = setInterval(() => setNow(Date.now()), 500); return () => clearInterval(t); }, []);

  if (recipe.isPending) return <main className="p-6"><Loading label="Loading recipe…" /></main>;
  if (recipe.isError) return <main className="p-6"><ErrorState onRetry={() => recipe.refetch()} /></main>;
  const r = recipe.data;
  const total = r.steps.length + 1;
  const at = Math.min(p.step, total - 1);
  const gathering = at === 0;
  const step = gathering ? undefined : r.steps[at - 1];
  const patch = (x: Partial<Progress>) => setP((s) => ({ ...s, ...x }));
  const ingredients = (
    <ul className="m-0 list-none p-0">
      {r.ingredients.map((i, n) => {
        const q = ingredientParts(i, 1);
        return (
          <li key={n} className="dashed-row flex min-h-12 items-center gap-3 py-1.5">
            {q.qty ? <QtyBadge qty={q.qty} unit={q.unit} /> : null}
            <span className="min-w-0 flex-1 break-words uppercase">{q.text}</span>
          </li>
        );
      })}
    </ul>
  );
  const toggleGot = (n: number) => patch({ got: p.got.includes(n) ? p.got.filter((g) => g !== n) : [...p.got, n] });
  const checklist = (
    <ul className="m-0 list-none p-0">
      {r.ingredients.length === 0 && <li className="py-3 text-ink-muted">No ingredients listed. On to the steps!</li>}
      {r.ingredients.map((i, n) => {
        const q = ingredientParts(i, 1);
        const on = p.got.includes(n);
        return (
          <li key={n}>
            {i.section && i.section !== r.ingredients[n - 1]?.section && <h3 className="mt-4 mb-1 eyebrow">{i.section}</h3>}
            <button type="button" aria-pressed={on} onClick={() => toggleGot(n)} className="dashed-row flex min-h-14 w-full cursor-pointer items-center gap-3 border-x-0 border-t-0 bg-transparent py-2 text-left text-ink">
              <Tick checked={on} />
              {q.qty ? <QtyBadge qty={q.qty} unit={q.unit} className={cn(on && 'opacity-50')} /> : null}
              <span className={cn('min-w-0 flex-1 text-xl break-words uppercase', on && 'text-ink-muted line-through decoration-accent-fill decoration-2')}>{q.text}</span>
            </button>
          </li>
        );
      })}
    </ul>
  );
  const gotLine = r.ingredients.length > 0 && <span className="font-bold text-ink-muted">{p.got.length} of {r.ingredients.length} ready</span>;
  const addTimer = (label: string, seconds: number) =>
    patch({ timers: [...p.timers, { id: crypto.randomUUID(), label, endsAt: Date.now() + seconds * 1000 }] });

  if (resume) {
    return (
      <main className="relative isolate grid min-h-dvh place-items-center overflow-hidden p-6 text-center">
        <Blob className="-top-20 -right-16 size-72 bg-butter" />
        <Blob shape="blob-b" className="-bottom-24 -left-16 size-64 bg-mint" />
        <div className="card relative max-w-md p-8">
          <p className="scribble">welcome back, chef</p>
          <h1 className="mt-1 mb-2 text-3xl">Pick up where you left off?</h1>
          <p className="mb-6 text-ink-muted">{r.title}: step {at + 1} of {total}</p>
          <div className="flex justify-center gap-3">
            <Button onClick={() => { setP(empty); setResume(false); }}>Start over</Button>
            <Button variant="primary" onClick={() => setResume(false)}>Resume</Button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-dvh flex-col">
      <header className="flex items-center gap-2.5 bg-rail px-4 py-3 text-on-rail">
        <Button asChild size="icon" aria-label="Close cook mode" className="border-on-rail shadow-none"><Link to="/recipes/$recipeId" params={{ recipeId }}><X strokeWidth={2.8} /></Link></Button>
        <h1 className="mr-auto min-w-0 truncate text-lg">{r.title}</h1>
        <Button size="sm" variant="sticker" className="shadow-none" aria-pressed={p.all} onClick={() => patch({ all: !p.all })}>{p.all ? 'One step' : 'All steps'}</Button>
        {(p.all || !gathering) && <Button size="sm" className="shadow-none lg:hidden" onClick={() => setPanel(true)}>Ingredients</Button>}
      </header>

      {p.timers.length > 0 && (
        <div className="flex flex-wrap gap-2 border-b-[2.5px] border-outline bg-surface-2 px-4 py-2.5" aria-label="Timers">
          {p.timers.map((t) => {
            const left = Math.max(0, Math.ceil((t.endsAt - now) / 1000));
            return (
              <button
                key={t.id}
                type="button"
                className={cn('inline-flex min-h-11 cursor-pointer items-center gap-2 rounded-full border-[2.5px] border-outline px-4 font-bold text-sticker-ink', left === 0 ? 'animate-pulse bg-accent-fill text-white' : 'bg-butter')}
                onClick={() => patch({ timers: p.timers.filter((x) => x.id !== t.id) })}
                aria-label={left === 0 ? `${t.label} timer finished. Dismiss` : `${t.label}: ${formatTimer(left)} left. Cancel`}
              >
                <Timer className="size-4.5" strokeWidth={2.6} />{t.label} <span className="font-display text-lg font-extrabold tabular-nums">{left === 0 ? 'Done!' : formatTimer(left)}</span>
              </button>
            );
          })}
        </div>
      )}

      <div className="mx-auto flex w-full max-w-6xl flex-1 gap-10 p-4 sm:p-8">
        <section className="flex min-w-0 flex-1 flex-col" aria-live="polite">
          {!p.all && (
            <>
              <div className="flex items-center gap-3">
                <span className="blob-d grid size-14 -rotate-6 place-items-center border-[2.5px] border-outline bg-butter font-display text-2xl font-extrabold text-sticker-ink shadow-pop-sm" aria-hidden>{at + 1}</span>
                <span className="font-bold text-ink-muted">Step {at + 1} of {total}</span>
              </div>
              <div className="mt-4 flex gap-1.5" aria-hidden>
                {Array.from({ length: total }, (_, n) => <span key={n} className={cn('h-2 flex-1 rounded-full border-[1.5px] border-outline', n <= at ? 'bg-accent-fill' : 'bg-surface')} />)}
              </div>
              {gathering ? (
                <div className="my-8 flex-1">
                  <div className="mb-2 flex flex-wrap items-baseline justify-between gap-x-4">
                    <p className="m-0 text-[1.75rem]/[2.4rem] font-medium sm:text-4xl/snug">Gather your ingredients</p>
                    {gotLine}
                  </div>
                  {checklist}
                </div>
              ) : step && (
                <>
                  <p className="my-8 flex-1 text-[1.75rem]/[2.4rem] font-medium sm:text-4xl/snug">{step.text}</p>
                  {step.timerSeconds && (
                    <p><Button variant="sticker" size="lg" onClick={() => addTimer(`Step ${at + 1}`, step.timerSeconds!)}><Timer strokeWidth={2.6} />Start {formatTimer(step.timerSeconds)} timer</Button></p>
                  )}
                </>
              )}
              {r.steps.length === 0 && !gathering && <p className="my-8 flex-1 text-ink-muted">This recipe has no steps yet.</p>}
              <div className="mt-8 grid grid-cols-2 gap-4">
                <Button size="lg" className="min-h-16 text-xl" disabled={at === 0} onClick={() => patch({ step: at - 1 })}><ArrowLeft />Previous</Button>
                <Button variant="primary" size="lg" className="min-h-16 text-xl" disabled={at >= total - 1} onClick={() => patch({ step: at + 1 })}>Next<ArrowRight /></Button>
              </div>
            </>
          )}
          {p.all && (
            <ol className="m-0 grid list-none gap-3 p-0">
              <li className="card flex flex-col gap-2 p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-x-4 text-xl">
                  <span><b className="font-display">1.</b> Gather your ingredients</span>
                  {gotLine}
                </div>
                {checklist}
              </li>
              {r.steps.map((s, n) => {
                const done = p.done.includes(n);
                return (
                  <li key={n} className={cn('card flex flex-col gap-2 p-4', done && 'opacity-50 shadow-none')}>
                    <button type="button" className="flex cursor-pointer items-start gap-3 border-0 bg-transparent p-0 text-left text-xl text-ink" aria-pressed={done}
                      onClick={() => patch({ done: done ? p.done.filter((d) => d !== n) : [...p.done, n] })}>
                      <Tick checked={done} className="mt-0.5" />
                      <span><b className="font-display">{n + 2}.</b> {s.text}</span>
                    </button>
                    {s.timerSeconds && <Button size="sm" variant="sticker" className="ml-10 self-start shadow-none" onClick={() => addTimer(`Step ${n + 2}`, s.timerSeconds!)}><Timer />Start {formatTimer(s.timerSeconds)} timer</Button>}
                  </li>
                );
              })}
            </ol>
          )}
        </section>
        <aside className={cn('card hidden w-80 flex-none self-start px-5 pt-4 pb-2', !p.all && !gathering && 'lg:block')} aria-label="Ingredients">
          <h2 className="mb-1 text-2xl">Ingredients</h2>
          {ingredients}
        </aside>
      </div>

      <Dialog open={panel} onOpenChange={setPanel}>
        <DialogContent title="Ingredients">{ingredients}</DialogContent>
      </Dialog>
    </main>
  );
}
