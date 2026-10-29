import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { getRecipe } from './api';
import { formatTimer, scaleIngredient } from './recipes';
import { ErrorState, Loading } from './states';

type Progress = { step: number; done: number[]; timers: { id: string; label: string; endsAt: number }[]; all: boolean };
const empty: Progress = { step: 0, done: [], timers: [], all: false };
const key = (id: string) => `imp-cook-${id}`;

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
  const [resume, setResume] = useState(!!saved.current && (saved.current.step > 0 || saved.current.done.length > 0 || saved.current.timers.length > 0));
  const [p, setP] = useState<Progress>(saved.current ?? empty);
  const [now, setNow] = useState(Date.now());
  const [panel, setPanel] = useState(false);
  useWakeLock();

  useEffect(() => { try { localStorage.setItem(key(recipeId), JSON.stringify(p)); } catch {} }, [p, recipeId]);
  useEffect(() => { const t = setInterval(() => setNow(Date.now()), 500); return () => clearInterval(t); }, []);

  if (recipe.isPending) return <main className="p-6"><Loading label="Loading recipe…" /></main>;
  if (recipe.isError) return <main className="p-6"><ErrorState onRetry={() => recipe.refetch()} /></main>;
  const r = recipe.data;
  const last = Math.max(r.steps.length - 1, 0);
  const step = r.steps[Math.min(p.step, last)];
  const patch = (x: Partial<Progress>) => setP((s) => ({ ...s, ...x }));
  const ingredients = (
    <ul className="m-0 list-none p-0">
      {r.ingredients.map((i, n) => <li key={n} className="border-t border-line py-2 first:border-t-0">{scaleIngredient(i, 1)}</li>)}
    </ul>
  );
  const addTimer = (label: string, seconds: number) =>
    patch({ timers: [...p.timers, { id: crypto.randomUUID(), label, endsAt: Date.now() + seconds * 1000 }] });

  if (resume) {
    return (
      <main className="grid min-h-screen place-items-center p-6 text-center">
        <div>
          <h1 className="mb-2 text-2xl">Pick up where you left off?</h1>
          <p className="sub">{r.title}: step {p.step + 1} of {r.steps.length}</p>
          <div className="flex justify-center gap-2">
            <button onClick={() => { setP(empty); setResume(false); }}>Start over</button>
            <button className="primary w-auto!" onClick={() => setResume(false)}>Resume</button>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="flex min-h-screen flex-col">
      <header className="flex items-center gap-2 border-b border-line bg-surface px-4 py-2">
        <Link to="/recipes/$recipeId" params={{ recipeId }} className="btn-primary" aria-label="Close cook mode">✕ Close</Link>
        <h1 className="m-0 mr-auto truncate text-base">{r.title}</h1>
        <button aria-pressed={p.all} onClick={() => patch({ all: !p.all })}>{p.all ? 'One step' : 'All steps'}</button>
        <button className="lg:hidden" onClick={() => setPanel(true)}>Ingredients</button>
      </header>

      {p.timers.length > 0 && (
        <div className="flex flex-wrap gap-2 border-b border-line bg-accent-soft px-4 py-2" aria-label="Timers">
          {p.timers.map((t) => {
            const left = Math.max(0, Math.ceil((t.endsAt - now) / 1000));
            return (
              <button key={t.id} className={left === 0 ? 'chip border-danger! font-bold' : 'chip'} onClick={() => patch({ timers: p.timers.filter((x) => x.id !== t.id) })}
                aria-label={left === 0 ? `${t.label} timer finished. Dismiss` : `${t.label}: ${formatTimer(left)} left. Cancel`}>
                ⏱ {t.label} {left === 0 ? 'Done!' : formatTimer(left)}
              </button>
            );
          })}
        </div>
      )}

      <div className="mx-auto flex w-full max-w-6xl flex-1 gap-8 p-4 sm:p-8">
        <section className="flex min-w-0 flex-1 flex-col" aria-live="polite">
          {r.steps.length === 0 && <p className="sub">This recipe has no steps yet.</p>}
          {!p.all && step && (
            <>
              <p className="text-ink-muted">Step {p.step + 1} of {r.steps.length}</p>
              <p className="my-4 flex-1 text-[1.75rem]/[2.375rem] sm:text-4xl/snug">{step.text}</p>
              {step.timerSeconds && (
                <p><button className="chip text-base!" onClick={() => addTimer(`Step ${p.step + 1}`, step.timerSeconds!)}>⏱ Start {formatTimer(step.timerSeconds)} timer</button></p>
              )}
              <div className="mt-6 grid grid-cols-2 gap-3">
                <button className="min-h-16! text-xl" disabled={p.step === 0} onClick={() => patch({ step: p.step - 1 })}>← Previous</button>
                <button className="primary min-h-16! text-xl" disabled={p.step >= last} onClick={() => patch({ step: p.step + 1 })}>Next →</button>
              </div>
            </>
          )}
          {p.all && (
            <ol className="m-0 grid list-none gap-3 p-0">
              {r.steps.map((s, n) => {
                const done = p.done.includes(n);
                return (
                  <li key={n}>
                    <button className={`w-full text-left text-xl ${done ? 'opacity-40' : ''}`} aria-pressed={done}
                      onClick={() => patch({ done: done ? p.done.filter((d) => d !== n) : [...p.done, n] })}>
                      <b>{n + 1}.</b> {s.text}
                      {s.timerSeconds && <span className="pill ml-2">⏱ {formatTimer(s.timerSeconds)}</span>}
                    </button>
                    {s.timerSeconds && <button className="chip mt-1" onClick={() => addTimer(`Step ${n + 1}`, s.timerSeconds!)}>Start timer</button>}
                  </li>
                );
              })}
            </ol>
          )}
        </section>
        <aside className="hidden w-72 flex-none lg:block" aria-label="Ingredients">
          <h2 className="mb-2 text-lg">Ingredients</h2>
          {ingredients}
        </aside>
      </div>

      {panel && (
        <div className="fixed inset-0 z-20 grid items-end bg-black/50" onClick={() => setPanel(false)}>
          <div className="max-h-[70vh] overflow-auto rounded-t-lg bg-surface p-5" onClick={(e) => e.stopPropagation()} role="dialog" aria-label="Ingredients">
            <div className="mb-2 flex items-center justify-between"><h2 className="text-lg">Ingredients</h2><button onClick={() => setPanel(false)}>Close</button></div>
            {ingredients}
          </div>
        </div>
      )}
    </main>
  );
}
