import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { getRecipe, saveRecipe, type Ingredient, type RecipeInput, type Step } from './api';
import { ArrowDown, ArrowUp, Check, Plus, Timer, X } from 'lucide-react';
import { ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn } from './ui/cn';
import { Blob } from './ui/sticker';

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
    <form className="relative isolate mx-auto max-w-3xl" onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
      <Blob shape="blob-b" className="-top-20 -right-24 size-56 bg-lilac" />
      <div className="relative mb-6 flex items-center gap-3">
        <Button asChild size="icon" aria-label="Cancel" className="shadow-none">
          <Link to={recipeId ? '/recipes/$recipeId' : '/recipes'} params={recipeId ? { recipeId } : undefined}><X strokeWidth={2.8} /></Link>
        </Button>
      </div>
      <div className="relative mb-6">
        <p className="scribble text-[26px]">{recipeId ? 'a little tweak' : 'ooh, something new!'}</p>
        <h1 className="text-[38px] leading-none md:text-5xl">{recipeId ? 'Edit recipe' : 'New recipe'}</h1>
      </div>

      <div className="relative flex flex-col gap-6">
        <Section n={1} title="The basics" fill="bg-accent-fill text-white">
          <div><label htmlFor="r-title" className="field-label">Title</label><input id="r-title" className="field text-lg font-bold" required value={d.title} onChange={(e) => set('title', e.target.value)} placeholder="Lemon ricotta pancakes" /></div>
          <div><label htmlFor="r-sum" className="field-label">Summary</label><textarea id="r-sum" className="field" rows={2} value={d.summary} onChange={(e) => set('summary', e.target.value)} placeholder="What makes it good?" /></div>
          <div className="grid grid-cols-3 gap-3">
            <div><label htmlFor="r-serv" className="field-label">Serves</label><input id="r-serv" className="field" type="number" min={0} step="any" value={d.servings} onChange={(e) => set('servings', e.target.value)} /></div>
            <div><label htmlFor="r-prep" className="field-label">Prep (min)</label><input id="r-prep" className="field" type="number" min={0} value={d.prep} onChange={(e) => set('prep', e.target.value)} /></div>
            <div><label htmlFor="r-cook" className="field-label">Cook (min)</label><input id="r-cook" className="field" type="number" min={0} value={d.cook} onChange={(e) => set('cook', e.target.value)} /></div>
          </div>
          <div><label htmlFor="r-tags" className="field-label">Tags <span className="font-medium text-ink-muted">(comma separated)</span></label><input id="r-tags" className="field" value={d.tags} onChange={(e) => set('tags', e.target.value)} placeholder="dinner, vegetarian" /></div>
        </Section>

        <Section n={2} title="Ingredients" fill="bg-butter">
          <div className="grid gap-3">
            {d.ingredients.map((i, n) => (
              <div key={n} className="grid grid-cols-[4.5rem_5rem_1fr] gap-2 rounded-md border-2 border-dashed border-line p-2 sm:grid-cols-[4.5rem_5rem_1fr_1fr_auto]">
                <input aria-label={`Ingredient ${n + 1} quantity`} className="field bg-butter px-2 text-center font-display font-extrabold text-sticker-ink" placeholder="Qty" inputMode="decimal" value={i.quantity} onChange={(e) => setIng(n, 'quantity', e.target.value)} />
                <input aria-label={`Ingredient ${n + 1} unit`} className="field px-3" placeholder="Unit" value={i.unit} onChange={(e) => setIng(n, 'unit', e.target.value)} />
                <input aria-label={`Ingredient ${n + 1} name`} className="field px-3 font-bold" placeholder="Ingredient" value={i.item} onChange={(e) => setIng(n, 'item', e.target.value)} />
                <input aria-label={`Ingredient ${n + 1} note`} className="field col-span-2 px-3 sm:col-span-1" placeholder="Note (chopped)" value={i.prepNote} onChange={(e) => setIng(n, 'prepNote', e.target.value)} />
                <span className="flex items-center gap-1">
                  <Button size="icon-sm" variant="ghost" aria-label={`Move ingredient ${n + 1} up`} disabled={n === 0} onClick={() => set('ingredients', move(d.ingredients, n, n - 1))}><ArrowUp /></Button>
                  <Button size="icon-sm" variant="ghost" aria-label={`Remove ingredient ${n + 1}`} onClick={() => set('ingredients', d.ingredients.filter((_, k) => k !== n))}><X /></Button>
                </span>
              </div>
            ))}
          </div>
          <Button variant="dashed" className="self-start" onClick={() => set('ingredients', [...d.ingredients, { ...blankIng }])}><Plus />Add ingredient</Button>
        </Section>

        <Section n={3} title="Steps" fill="bg-mint">
          <div className="grid gap-3">
            {d.steps.map((s, n) => (
              <div key={n} className="flex gap-3">
                <span className="blob-d mt-1 grid size-10 flex-none place-items-center border-[2.5px] border-outline bg-sky font-display text-lg font-extrabold text-sticker-ink" aria-hidden>{n + 1}</span>
                <div className="grid min-w-0 flex-1 grid-cols-[1fr_6rem] gap-2">
                  <textarea aria-label={`Step ${n + 1}`} className="field" rows={2} placeholder={`Step ${n + 1}`} value={s.text} onChange={(e) => setStep(n, 'text', e.target.value)} />
                  <label className="relative">
                    <span className="sr-only">Step {n + 1} timer in minutes</span>
                    <Timer className="pointer-events-none absolute top-3.5 left-2.5 size-4.5 text-sticker-ink" aria-hidden />
                    <input className="field bg-butter pl-8 font-bold text-sticker-ink" placeholder="min" inputMode="decimal" value={s.minutes} onChange={(e) => setStep(n, 'minutes', e.target.value)} />
                  </label>
                  <span className="col-span-2 flex gap-1">
                    <Button size="icon-sm" variant="ghost" aria-label={`Move step ${n + 1} up`} disabled={n === 0} onClick={() => set('steps', move(d.steps, n, n - 1))}><ArrowUp /></Button>
                    <Button size="icon-sm" variant="ghost" aria-label={`Move step ${n + 1} down`} disabled={n === d.steps.length - 1} onClick={() => set('steps', move(d.steps, n, n + 1))}><ArrowDown /></Button>
                    <Button size="icon-sm" variant="ghost" aria-label={`Remove step ${n + 1}`} onClick={() => set('steps', d.steps.filter((_, k) => k !== n))}><X /></Button>
                  </span>
                </div>
              </div>
            ))}
          </div>
          <Button variant="dashed" className="self-start" onClick={() => set('steps', [...d.steps, { text: '', minutes: '' }])}><Plus />Add step</Button>
        </Section>

        <Section n={4} title="Extras" fill="bg-sky">
          <div><label htmlFor="r-src" className="field-label">Source URL</label><input id="r-src" className="field" type="url" value={d.sourceUrl} onChange={(e) => set('sourceUrl', e.target.value)} placeholder="https://" /></div>
          <div><label htmlFor="r-notes" className="field-label">Notes</label><textarea id="r-notes" className="field" rows={3} value={d.notes} onChange={(e) => set('notes', e.target.value)} placeholder="Tips, swaps, who loved it" /></div>
        </Section>

        {save.isError && <p className="err" role="alert">{(save.error as Error).message}</p>}
        <div className="flex items-center justify-end gap-3">
          <p className="mr-auto -rotate-1 font-hand text-xl text-ink-muted max-sm:hidden">nothing's set in stone, edit any time</p>
          <Button type="submit" variant="primary" size="lg" disabled={!d.title.trim() || save.isPending}><Check strokeWidth={2.8} />{save.isPending ? 'Saving…' : 'Save recipe'}</Button>
        </div>
      </div>
    </form>
  );
}

function Section({ n, title, fill, children }: { n: number; title: string; fill: string; children: React.ReactNode }) {
  return (
    <section className="card flex flex-col gap-4 p-5 md:p-6">
      <h2 className="flex items-center gap-3 text-2xl">
        <span className={cn('blob-d grid size-9 flex-none -rotate-6 place-items-center border-[2.5px] border-outline text-base text-sticker-ink', fill)} aria-hidden>{n}</span>
        {title}
      </h2>
      {children}
    </section>
  );
}
