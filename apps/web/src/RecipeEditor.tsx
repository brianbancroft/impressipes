import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { getRecipe, saveRecipe, type Ingredient, type RecipeInput, type Step } from './api';
import { ErrorState, Loading } from './states';

type Draft = {
  title: string; summary: string; servings: string; prep: string; cook: string;
  tags: string; sourceUrl: string; notes: string;
  ingredients: { section: string; quantity: string; unit: string; item: string; prepNote: string }[];
  steps: { text: string; minutes: string }[];
};

const blankIng = { section: '', quantity: '', unit: '', item: '', prepNote: '' };
const blank: Draft = { title: '', summary: '', servings: '', prep: '', cook: '', tags: '', sourceUrl: '', notes: '', ingredients: [{ ...blankIng }], steps: [{ text: '', minutes: '' }] };
const num = (s: string) => (s.trim() === '' || Number.isNaN(Number(s)) ? undefined : Number(s));

function toInput(d: Draft): RecipeInput {
  const ingredients: Ingredient[] = d.ingredients.filter((i) => i.item.trim()).map((i) => ({
    item: i.item.trim(), section: i.section.trim() || undefined, quantity: num(i.quantity),
    unit: i.unit.trim() || undefined, prepNote: i.prepNote.trim() || undefined,
  }));
  const steps: Step[] = d.steps.filter((s) => s.text.trim()).map((s) => {
    const m = num(s.minutes);
    return { text: s.text.trim(), timerSeconds: m && m > 0 ? Math.round(m * 60) : undefined };
  });
  return {
    title: d.title.trim(), summary: d.summary.trim(), servings: num(d.servings),
    prepMinutes: num(d.prep), cookMinutes: num(d.cook), sourceUrl: d.sourceUrl.trim() || undefined,
    sourceKind: 'manual', notes: d.notes, ingredients, steps,
    tags: d.tags.split(',').map((t) => t.trim().toLowerCase()).filter(Boolean),
  };
}

export function RecipeEditor({ recipeId }: { recipeId?: string }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const existing = useQuery({ queryKey: ['recipe', recipeId], queryFn: () => getRecipe(recipeId!), enabled: !!recipeId });
  const [d, setD] = useState<Draft>(blank);

  useEffect(() => {
    const r = existing.data;
    if (!r) return;
    setD({
      title: r.title, summary: r.summary, servings: r.servings?.toString() ?? '', prep: r.prepMinutes?.toString() ?? '',
      cook: r.cookMinutes?.toString() ?? '', tags: r.tags.join(', '), sourceUrl: r.sourceUrl ?? '', notes: r.notes,
      ingredients: r.ingredients.length ? r.ingredients.map((i) => ({ section: i.section ?? '', quantity: i.quantity?.toString() ?? '', unit: i.unit ?? '', item: i.item, prepNote: i.prepNote ?? '' })) : [{ ...blankIng }],
      steps: r.steps.length ? r.steps.map((s) => ({ text: s.text, minutes: s.timerSeconds ? String(s.timerSeconds / 60) : '' })) : [{ text: '', minutes: '' }],
    });
  }, [existing.data]);

  const save = useMutation({
    mutationFn: () => saveRecipe(recipeId, toInput(d)),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['recipes'] });
      qc.invalidateQueries({ queryKey: ['tags'] });
      qc.setQueryData(['recipe', r.id], r);
      navigate({ to: '/recipes/$recipeId', params: { recipeId: r.id } });
    },
  });

  if (recipeId && existing.isPending) return <Loading label="Loading recipe…" />;
  if (recipeId && existing.isError) return <ErrorState onRetry={() => existing.refetch()} />;

  const set = <K extends keyof Draft>(k: K, v: Draft[K]) => setD((s) => ({ ...s, [k]: v }));
  const setIng = (n: number, k: keyof Draft['ingredients'][0], v: string) => set('ingredients', d.ingredients.map((x, i) => (i === n ? { ...x, [k]: v } : x)));
  const setStep = (n: number, k: keyof Draft['steps'][0], v: string) => set('steps', d.steps.map((x, i) => (i === n ? { ...x, [k]: v } : x)));
  const move = <T,>(arr: T[], n: number, to: number) => { const a = [...arr]; const [x] = a.splice(n, 1); a.splice(to, 0, x); return a; };

  return (
    <form className="max-w-3xl" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
      <p className="mb-2"><Link to={recipeId ? '/recipes/$recipeId' : '/recipes'} params={recipeId ? { recipeId } : undefined}>← Cancel</Link></p>
      <h1 className="page-title">{recipeId ? 'Edit recipe' : 'New recipe'}</h1>

      <div className="my-4"><label htmlFor="r-title">Title</label><input id="r-title" required value={d.title} onChange={(e) => set('title', e.target.value)} /></div>
      <div className="mb-4"><label htmlFor="r-sum">Summary</label><textarea id="r-sum" rows={2} value={d.summary} onChange={(e) => set('summary', e.target.value)} /></div>
      <div className="mb-4 grid grid-cols-3 gap-3">
        <div><label htmlFor="r-serv">Servings</label><input id="r-serv" type="number" min={0} step="any" value={d.servings} onChange={(e) => set('servings', e.target.value)} /></div>
        <div><label htmlFor="r-prep">Prep (min)</label><input id="r-prep" type="number" min={0} value={d.prep} onChange={(e) => set('prep', e.target.value)} /></div>
        <div><label htmlFor="r-cook">Cook (min)</label><input id="r-cook" type="number" min={0} value={d.cook} onChange={(e) => set('cook', e.target.value)} /></div>
      </div>
      <div className="mb-4"><label htmlFor="r-tags">Tags (comma separated)</label><input id="r-tags" value={d.tags} onChange={(e) => set('tags', e.target.value)} placeholder="dinner, vegetarian" /></div>

      <h2 className="mb-2 text-xl">Ingredients</h2>
      <div className="grid gap-2">
        {d.ingredients.map((i, n) => (
          <div key={n} className="grid grid-cols-[4.5rem_5rem_1fr] gap-2 rounded-sm border border-line bg-surface p-2 sm:grid-cols-[4.5rem_5rem_1fr_1fr_auto]">
            <input aria-label={`Ingredient ${n + 1} quantity`} placeholder="Qty" inputMode="decimal" value={i.quantity} onChange={(e) => setIng(n, 'quantity', e.target.value)} />
            <input aria-label={`Ingredient ${n + 1} unit`} placeholder="Unit" value={i.unit} onChange={(e) => setIng(n, 'unit', e.target.value)} />
            <input aria-label={`Ingredient ${n + 1} name`} placeholder="Ingredient" value={i.item} onChange={(e) => setIng(n, 'item', e.target.value)} />
            <input aria-label={`Ingredient ${n + 1} note`} placeholder="Note (chopped)" className="col-span-2 sm:col-span-1" value={i.prepNote} onChange={(e) => setIng(n, 'prepNote', e.target.value)} />
            <span className="col-span-1 flex gap-1">
              <button type="button" aria-label={`Move ingredient ${n + 1} up`} disabled={n === 0} onClick={() => set('ingredients', move(d.ingredients, n, n - 1))}>↑</button>
              <button type="button" aria-label={`Remove ingredient ${n + 1}`} onClick={() => set('ingredients', d.ingredients.filter((_, k) => k !== n))}>✕</button>
            </span>
          </div>
        ))}
      </div>
      <button type="button" className="mt-2" onClick={() => set('ingredients', [...d.ingredients, { ...blankIng }])}>+ Add ingredient</button>

      <h2 className="mb-2 mt-8 text-xl">Steps</h2>
      <div className="grid gap-2">
        {d.steps.map((s, n) => (
          <div key={n} className="grid grid-cols-[1fr_5rem] gap-2 rounded-sm border border-line bg-surface p-2">
            <textarea aria-label={`Step ${n + 1}`} rows={2} placeholder={`Step ${n + 1}`} value={s.text} onChange={(e) => setStep(n, 'text', e.target.value)} />
            <input aria-label={`Step ${n + 1} timer in minutes`} placeholder="Timer min" inputMode="decimal" value={s.minutes} onChange={(e) => setStep(n, 'minutes', e.target.value)} />
            <span className="col-span-2 flex gap-1">
              <button type="button" aria-label={`Move step ${n + 1} up`} disabled={n === 0} onClick={() => set('steps', move(d.steps, n, n - 1))}>↑</button>
              <button type="button" aria-label={`Move step ${n + 1} down`} disabled={n === d.steps.length - 1} onClick={() => set('steps', move(d.steps, n, n + 1))}>↓</button>
              <button type="button" aria-label={`Remove step ${n + 1}`} onClick={() => set('steps', d.steps.filter((_, k) => k !== n))}>✕</button>
            </span>
          </div>
        ))}
      </div>
      <button type="button" className="mt-2" onClick={() => set('steps', [...d.steps, { text: '', minutes: '' }])}>+ Add step</button>

      <div className="my-8 grid gap-4">
        <div><label htmlFor="r-src">Source URL</label><input id="r-src" type="url" value={d.sourceUrl} onChange={(e) => set('sourceUrl', e.target.value)} /></div>
        <div><label htmlFor="r-notes">Notes</label><textarea id="r-notes" rows={3} value={d.notes} onChange={(e) => set('notes', e.target.value)} /></div>
      </div>

      {save.isError && <p className="err" role="alert">{(save.error as Error).message}</p>}
      <button className="primary max-w-xs" disabled={!d.title.trim() || save.isPending}>{save.isPending ? 'Saving…' : 'Save recipe'}</button>
    </form>
  );
}
