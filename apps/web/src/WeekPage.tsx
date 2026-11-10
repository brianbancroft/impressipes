import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { Check, ChevronLeft, ChevronRight, UtensilsCrossed } from 'lucide-react';
import { useMemo, useState } from 'react';
import {
  deleteMeal, listMealPlan, listRecipes, updateMeal, upsertMeal,
  type MealPlanEntry, type RecipeSummary,
} from './api';
import { ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn } from './ui/cn';
import { Dialog, DialogContent } from './ui/dialog';
import { FAMILY_TZ, SLOTS, addDays, dayHeading, rangeLabel, startOfWeek, todayISO, type Slot, weekDates } from './week';

export function WeekPage() {
  const [sunday, setSunday] = useState(() => startOfWeek(todayISO()));
  const dates = useMemo(() => weekDates(sunday), [sunday]);
  const from = dates[0];
  const to = dates[6];
  const today = todayISO();
  const plan = useQuery({ queryKey: ['meal-plan', from, to], queryFn: () => listMealPlan(from, to) });
  const recipes = useQuery({ queryKey: ['recipes', {}], queryFn: () => listRecipes({}) });
  const [focus, setFocus] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const selected = focus && dates.includes(focus) ? focus : dates.includes(today) ? today : dates[0];

  const byDay = useMemo(() => {
    const map = new Map<string, MealPlanEntry[]>();
    for (const d of dates) map.set(d, []);
    for (const e of plan.data ?? []) {
      const list = map.get(e.planDate);
      if (list) list.push(e);
    }
    return map;
  }, [plan.data, dates]);

  const open = (date: string, slot: Slot, entry?: MealPlanEntry) => {
    setDraft({
      date,
      slot,
      entryId: entry?.id,
      title: entry?.title ?? '',
      recipeId: entry?.recipeId ?? '',
    });
  };

  return (
    <div>
      <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
        <div>
          <h1 className="text-4xl md:text-5xl">This week</h1>
          <p className="mt-1 text-ink-muted">{rangeLabel(from, to)} · {FAMILY_TZ}</p>
        </div>
        <div className="flex items-center gap-2">
          <Button size="icon" aria-label="Previous week" onClick={() => setSunday((s) => addDays(s, -7))}><ChevronLeft /></Button>
          <Button variant="sticker" onClick={() => { const t = todayISO(); setSunday(startOfWeek(t)); setFocus(t); }}>Today</Button>
          <Button size="icon" aria-label="Next week" onClick={() => setSunday((s) => addDays(s, 7))}><ChevronRight /></Button>
        </div>
      </div>

      {plan.isPending && <Loading label="Loading the week…" />}
      {plan.isError && <ErrorState onRetry={() => plan.refetch()} />}
      {plan.data && (
        <>
          <div className="mb-4 flex gap-2 overflow-x-auto md:hidden" role="tablist" aria-label="Days">
            {dates.map((d) => {
              const h = dayHeading(d);
              const on = d === selected;
              return (
                <button
                  key={d}
                  type="button"
                  role="tab"
                  aria-selected={on}
                  onClick={() => setFocus(d)}
                  className={cn(
                    'min-w-14 rounded-full border-[2.5px] border-outline px-3 py-2 text-center font-bold',
                    d === today || on ? 'bg-butter text-sticker-ink' : 'bg-surface text-ink',
                    on && d !== today && 'ring-2 ring-accent-fill',
                  )}
                >
                  <span className="block text-xs uppercase">{h.dow.slice(0, 2)}</span>
                  <span className="font-display text-lg">{h.num}</span>
                </button>
              );
            })}
          </div>
          <div className="md:hidden">
            <DayColumn
              date={selected}
              today={today}
              entries={byDay.get(selected) ?? []}
              onOpen={open}
            />
          </div>
          <div className="hidden gap-2 md:grid md:grid-cols-7">
            {dates.map((d) => (
              <DayColumn key={d} date={d} today={today} entries={byDay.get(d) ?? []} onOpen={open} />
            ))}
          </div>
        </>
      )}

      {draft && (
        <MealDialog
          draft={draft}
          recipes={recipes.data ?? []}
          onClose={() => setDraft(null)}
          onChange={setDraft}
        />
      )}
    </div>
  );
}

function DayColumn({ date, today, entries, onOpen }: {
  date: string;
  today: string;
  entries: MealPlanEntry[];
  onOpen: (date: string, slot: Slot, entry?: MealPlanEntry) => void;
}) {
  const h = dayHeading(date);
  const filled = entries.length;
  const done = entries.filter((e) => e.completed).length;
  const bySlot = new Map(entries.map((e) => [e.slot, e]));
  return (
    <section aria-label={h.long} className="flex min-w-0 flex-col gap-2">
      <header className={cn('rounded-xl px-3 py-2 text-center', date === today ? 'bg-butter text-sticker-ink' : 'bg-surface-2 text-ink')}>
        <p className="font-display text-lg leading-none">{h.dow} {h.num}</p>
        <p className={cn('mt-1 text-xs font-bold', date === today ? 'text-sticker-ink/80' : 'text-ink-muted')}>
          {filled === 0 ? '0 done' : `${done}/${filled} done`}
        </p>
      </header>
      {SLOTS.map((slot) => {
        const entry = bySlot.get(slot.id);
        if (!entry) {
          return (
            <button
              key={slot.id}
              type="button"
              onClick={() => onOpen(date, slot.id)}
              className="rounded-xl border-[2.5px] border-dashed border-line bg-surface/40 px-3 py-3 text-left text-sm font-bold text-ink-muted hover:border-outline hover:text-ink"
            >
              <span className="eyebrow block">{slot.label}</span>
              {slot.add}
            </button>
          );
        }
        return <MealCard key={slot.id} entry={entry} onOpen={() => onOpen(date, slot.id, entry)} />;
      })}
    </section>
  );
}

function MealCard({ entry, onOpen }: { entry: MealPlanEntry; onOpen: () => void }) {
  const qc = useQueryClient();
  const toggle = useMutation({
    mutationFn: () => updateMeal(entry.id, { completed: !entry.completed }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['meal-plan'] }),
  });
  return (
    <article className={cn('rounded-xl border-[2.5px] border-outline bg-surface p-3 shadow-pop-sm', entry.completed && 'opacity-80')}>
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <button type="button" onClick={onOpen} className="w-full border-0 bg-transparent p-0 text-left text-ink">
            <span className="eyebrow">{entry.slot}</span>
            <span className={cn('mt-1 block font-display text-lg leading-tight', entry.completed && 'text-success')}>{entry.title}</span>
          </button>
          {entry.recipe ? (
            <Link
              to="/recipes/$recipeId"
              params={{ recipeId: entry.recipe.id }}
              className="mt-1 inline-flex items-center gap-1 text-xs font-bold text-warning no-underline hover:text-warning"
            >
              <UtensilsCrossed className="size-3.5" />{entry.recipe.title === entry.title ? 'recipe' : entry.recipe.title}
            </Link>
          ) : (
            <span className="mt-1 block text-xs font-medium text-ink-muted">free-text</span>
          )}
        </div>
        <button
          type="button"
          aria-pressed={entry.completed}
          aria-label={entry.completed ? `Mark ${entry.title} not done` : `Mark ${entry.title} done`}
          onClick={() => toggle.mutate()}
          className={cn(
            'grid size-7 flex-none place-items-center rounded-full border-2',
            entry.completed ? 'border-mint bg-mint text-sticker-ink' : 'border-ink-muted bg-transparent text-transparent',
          )}
        >
          <Check className="size-4" strokeWidth={3} />
        </button>
      </div>
    </article>
  );
}

type Draft = { date: string; slot: Slot; entryId?: string; title: string; recipeId: string };

function MealDialog({ draft, recipes, onClose, onChange }: {
  draft: Draft;
  recipes: RecipeSummary[];
  onClose: () => void;
  onChange: (d: Draft) => void;
}) {
  const qc = useQueryClient();
  const [q, setQ] = useState('');
  const [formError, setFormError] = useState('');
  const slot = SLOTS.find((s) => s.id === draft.slot)!;
  const matches = recipes
    .filter((r) => r.title.toLowerCase().includes(q.trim().toLowerCase()))
    .slice(0, 8);
  const save = useMutation({
    mutationFn: () => upsertMeal({
      planDate: draft.date,
      slot: draft.slot,
      title: draft.title.trim(),
      recipeId: draft.recipeId,
    }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['meal-plan'] }); onClose(); },
  });
  const remove = useMutation({
    mutationFn: () => deleteMeal(draft.entryId!),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['meal-plan'] }); onClose(); },
  });
  const pick = (r: RecipeSummary | null) => {
    onChange({
      ...draft,
      recipeId: r?.id ?? '',
      title: draft.title.trim() === '' || recipes.some((x) => x.title === draft.title) ? (r?.title ?? draft.title) : draft.title,
    });
  };

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent
        title={draft.entryId ? `Edit ${slot.label.toLowerCase()}` : `Add ${slot.label.toLowerCase()}`}
        description={`${dayHeading(draft.date).long}. Link a recipe, or leave it blank for a meal you make from memory.`}
      >
        <label className="field-label" htmlFor="meal-title">Title</label>
        <input id="meal-title" className="field mb-4" value={draft.title} onChange={(e) => onChange({ ...draft, title: e.target.value })} placeholder="Name this meal" />
        <label className="field-label" htmlFor="meal-recipe">Link a recipe</label>
        <input id="meal-recipe" className="field" value={q} onChange={(e) => setQ(e.target.value)} placeholder="Search recipes" />
        <div className="mt-2 flex max-h-48 flex-col gap-1 overflow-y-auto">
          <button type="button" onClick={() => pick(null)} className={cn('rounded-md px-3 py-2 text-left font-bold', draft.recipeId === '' ? 'bg-butter text-sticker-ink' : 'bg-surface-2 text-ink')}>
            No recipe · free text
          </button>
          {matches.map((r) => (
            <button key={r.id} type="button" onClick={() => pick(r)} className={cn('rounded-md px-3 py-2 text-left font-bold', draft.recipeId === r.id ? 'bg-butter text-sticker-ink' : 'bg-surface text-ink')}>
              {r.title}
            </button>
          ))}
          {matches.length === 0 && <p className="px-1 py-2 text-sm text-ink-muted">No recipes match.</p>}
        </div>
        {(formError || save.isError || remove.isError) && <p className="err mt-3" role="alert">{formError || ((save.error || remove.error) as Error).message}</p>}
        <div className="mt-5 flex flex-wrap gap-2">
          <Button variant="primary" className="bg-accent-fill text-white" disabled={save.isPending} onClick={() => {
            if (!draft.title.trim()) { setFormError('Type a name for this meal, then save. A recipe is optional.'); return; }
            setFormError('');
            save.mutate();
          }}>Save</Button>
          {draft.entryId && (
            <Button variant="ghost" className="text-danger" disabled={remove.isPending} onClick={() => remove.mutate()}>Remove</Button>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
