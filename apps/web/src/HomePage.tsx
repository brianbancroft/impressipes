import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowRight, ChevronRight, Plus, ShoppingCart, Soup } from 'lucide-react';
import { listLists, listRecipes, setMark, type RecipeMark, type RecipeSummary } from './api';
import { RecipeCard } from './RecipesPage';
import { SetupChecklist, useSetupChecklist } from './SetupChecklist';
import { EmptyState, ErrorState, Loading } from './states';
import { Button } from './ui/button';
import { cn, tint } from './ui/cn';
import { Blob } from './ui/sticker';

const RECIPES_SHOWN = 4;
const LISTS_SHOWN = 3;

/**
 * The signed-in landing page. Getting started leads until it is done (or hidden);
 * after that, a few recent recipes and the lists, each with a way into the full page.
 */
export function HomePage() {
  const setup = useSetupChecklist();
  const name = setup.member?.name;

  return (
    <div className="relative isolate">
      <Blob className="-top-24 -right-28 size-72 bg-butter md:-top-36 md:size-96" />
      <div className="relative mb-6">
        <p className="scribble md:text-[28px]">hey {name || 'there'}!</p>
        <h1 className="text-[34px] leading-[1.02] md:text-5xl">{setup.show ? 'Welcome to Impressipes' : 'What are we cooking tonight?'}</h1>
      </div>

      {setup.loading && <Loading />}
      <SetupChecklist state={setup} />
      {!setup.loading && !setup.show && (
        <>
          <RecentRecipes />
          <YourLists />
        </>
      )}
    </div>
  );
}

function SectionHead({ title, to, label }: { title: string; to: '/recipes' | '/lists'; label: string }) {
  return (
    <div className="mb-4 flex items-end justify-between gap-3">
      <h2 className="text-2xl md:text-3xl">{title}</h2>
      <Link to={to} className="inline-flex items-center gap-1 font-bold">{label}<ArrowRight className="size-4" /></Link>
    </div>
  );
}

function RecentRecipes() {
  const qc = useQueryClient();
  const recipes = useQuery({ queryKey: ['recipes', {}], queryFn: () => listRecipes({}) });
  const toggleMark = useMutation({
    mutationFn: ({ r, m }: { r: RecipeSummary; m: RecipeMark }) => setMark(r.id, m, !r.marks.includes(m)),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['recipes'] }),
  });
  const recent = recipes.data
    ? [...recipes.data].sort((a, b) => b.updatedAt.localeCompare(a.updatedAt)).slice(0, RECIPES_SHOWN)
    : [];

  return (
    <section aria-labelledby="home-recipes" className="relative mb-10">
      <SectionHead title="Recent recipes" to="/recipes" label="View all recipes" />
      <span id="home-recipes" className="sr-only">Recent recipes</span>
      {recipes.isPending && <Loading label="Loading recipes…" />}
      {recipes.isError && <ErrorState onRetry={() => recipes.refetch()} />}
      {recipes.data?.length === 0 && (
        <EmptyState icon={Soup} title="No recipes yet" body="Ask your AI to add one, or write one yourself.">
          <Button asChild variant="primary"><Link to="/recipes/new"><Plus />New recipe</Link></Button>
        </EmptyState>
      )}
      <ul className="m-0 grid list-none grid-cols-2 gap-4 p-0 md:grid-cols-4 md:gap-6">
        {recent.map((r) => (
          <RecipeCard key={r.id} r={r} selecting={false} selected={false} onSelect={() => {}} onMark={(m) => toggleMark.mutate({ r, m })} />
        ))}
      </ul>
    </section>
  );
}

function YourLists() {
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  return (
    <section aria-label="Your lists" className="relative mb-10">
      <SectionHead title="Your lists" to="/lists" label="View all lists" />
      {lists.isPending && <Loading label="Loading lists…" />}
      {lists.isError && <ErrorState onRetry={() => lists.refetch()} />}
      {lists.data?.length === 0 && (
        <EmptyState icon={ShoppingCart} fill="bg-mint" title="No lists yet" body="Make one for each shop.">
          <Button asChild variant="primary"><Link to="/lists"><Plus />New list</Link></Button>
        </EmptyState>
      )}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {lists.data?.slice(0, LISTS_SHOWN).map((l) => (
          <Link
            key={l.id}
            to="/lists/$listId"
            params={{ listId: l.id }}
            className={cn('flex min-h-19 items-center gap-3 rounded-lg border-[2.5px] border-outline p-4 text-sticker-ink no-underline shadow-pop-sm hover:text-sticker-ink')}
            style={{ background: tint(l.colour) }}
          >
            <span className="text-2xl" aria-hidden>{l.emoji}</span>
            <span className="min-w-0 flex-1 truncate font-display text-xl font-extrabold">{l.name}</span>
            <ChevronRight className="size-5 flex-none" aria-hidden />
          </Link>
        ))}
      </div>
    </section>
  );
}
