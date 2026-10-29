import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from '@tanstack/react-router';
import { useState } from 'react';
import { AddToListDialog } from './AddToListDialog';
import { apiUrl, deleteRecipe, getRecipe, listMembers, setMark, type RecipeMark } from './api';
import { formatMinutes, formatTimer, scaleIngredient, sourceLine, totalMinutes } from './recipes';
import { ErrorState, Loading } from './states';

export function RecipePage() {
  const { recipeId } = useParams({ from: '/authed/shell/recipes/$recipeId' });
  const qc = useQueryClient();
  const navigate = useNavigate();
  const recipe = useQuery({ queryKey: ['recipe', recipeId], queryFn: () => getRecipe(recipeId) });
  const members = useQuery({ queryKey: ['members'], queryFn: listMembers, retry: false });
  const [servings, setServings] = useState<number>();
  const [dialog, setDialog] = useState(false);
  const [toast, setToast] = useState('');

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
  const time = totalMinutes(r);
  const fav = r.marks.includes('favourite');
  const want = r.marks.includes('want_to_make');
  const who = sourceLine(r, (id) => members.data?.find((m) => m.id === id)?.name);
  const step = (d: number) => setServings(Math.max(1, Math.round((current + d) * 2) / 2));

  return (
    <article>
      <p className="mb-2"><Link to="/recipes">← Recipes</Link></p>
      {r.images[0] && <img src={apiUrl(r.images[0].largeUrl)} alt="" className="mb-4 max-h-96 w-full rounded-md object-cover" />}
      <h1 className="page-title">{r.title}</h1>
      {who && <p className="sub mb-1!">{who}</p>}
      {r.summary && <p className="mb-3">{r.summary}</p>}
      <p className="sub">
        {[r.prepMinutes ? `Prep ${formatMinutes(r.prepMinutes)}` : '', r.cookMinutes ? `Cook ${formatMinutes(r.cookMinutes)}` : '', time ? `Total ${formatMinutes(time)}` : '']
          .filter(Boolean).join(' · ')}
      </p>
      {r.tags.length > 0 && <p className="mb-4 flex flex-wrap gap-2">{r.tags.map((t) => <span key={t} className="pill">{t}</span>)}</p>}

      <div className="sticky top-0 z-10 -mx-4 mb-6 flex flex-wrap items-center gap-2 border-b border-line bg-bg px-4 py-3">
        {base > 0 && (
          <div className="flex items-center gap-1" role="group" aria-label="Servings">
            <button aria-label="Fewer servings" onClick={() => step(-1)}>−</button>
            <span className="min-w-20 text-center" aria-live="polite">
              {current} serving{current === 1 ? '' : 's'}
              {current !== base && <small className="block text-xs text-ink-muted">makes {base}</small>}
            </span>
            <button aria-label="More servings" onClick={() => step(1)}>+</button>
          </div>
        )}
        <span className="grow" />
        <button aria-pressed={fav} onClick={() => mark.mutate({ m: 'favourite', on: !fav })}>{fav ? '♥ Favourite' : '♡ Favourite'}</button>
        <button aria-pressed={want} onClick={() => mark.mutate({ m: 'want_to_make', on: !want })}>{want ? '🔖 Want to make' : 'Want to make'}</button>
        <button onClick={() => setDialog(true)}>Add to list</button>
        <Link to="/recipes/$recipeId/cook" params={{ recipeId }} className="btn-primary">Cook</Link>
      </div>

      <div className="grid gap-8 lg:grid-cols-[minmax(0,2fr)_minmax(0,3fr)]">
        <section aria-labelledby="ing">
          <h2 id="ing" className="mb-3 text-xl">Ingredients</h2>
          {r.ingredients.length === 0 && <p className="sub">No ingredients yet.</p>}
          <ul className="m-0 list-none p-0">
            {r.ingredients.map((i, n) => (
              <li key={n}>
                {i.section && i.section !== r.ingredients[n - 1]?.section && <h3 className="mb-1 mt-4 text-sm text-ink-muted">{i.section}</h3>}
                <div className="border-t border-line py-2">{scaleIngredient(i, factor)}</div>
              </li>
            ))}
          </ul>
        </section>
        <section aria-labelledby="steps">
          <h2 id="steps" className="mb-3 text-xl">Steps</h2>
          {r.steps.length === 0 && <p className="sub">No steps yet.</p>}
          <ol className="m-0 grid list-none gap-4 p-0">
            {r.steps.map((s, n) => (
              <li key={n} className="flex gap-3">
                <span className="grid size-8 flex-none place-items-center rounded-full bg-accent-soft font-bold" aria-hidden>{n + 1}</span>
                <p className="m-0">
                  {s.text}
                  {s.timerSeconds && <span className="pill ml-2">⏱ {formatTimer(s.timerSeconds)}</span>}
                </p>
              </li>
            ))}
          </ol>
          {r.notes && (<><h2 className="mb-2 mt-8 text-xl">Notes</h2><p className="whitespace-pre-wrap">{r.notes}</p></>)}
        </section>
      </div>

      <div className="mt-10 flex gap-2">
        <Link to="/recipes/$recipeId/edit" params={{ recipeId }} className="nav-link border border-line">Edit</Link>
        <button onClick={() => { if (confirm(`Delete "${r.title}"? This can't be undone.`)) del.mutate(); }}>Delete</button>
      </div>

      {dialog && (
        <AddToListDialog
          recipes={[{ id: r.id, title: r.title, servings: current || undefined }]}
          onClose={() => setDialog(false)}
          onDone={(m) => { setDialog(false); setToast(m); setTimeout(() => setToast(''), 3500); }}
        />
      )}
      {toast && <div className="toast" role="status">{toast}</div>}
    </article>
  );
}
