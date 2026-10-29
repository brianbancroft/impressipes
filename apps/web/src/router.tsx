import { useQuery } from '@tanstack/react-query';
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  redirect,
  useParams,
} from '@tanstack/react-router';
import { useState } from 'react';
import { Admin } from './Admin';
import { getMe, type Me } from './api';
import { CookMode } from './CookMode';
import { Onboarding } from './Onboarding';
import { RecipeEditor } from './RecipeEditor';
import { RecipePage } from './RecipePage';
import { RecipesPage } from './RecipesPage';
import { EmptyState, ErrorState, Loading } from './states';
import { Shell } from './Shell';
import { applyTheme, getTheme, type Theme } from './theme';

const meQuery = { queryKey: ['me'], queryFn: getMe, staleTime: 60_000 } as const;

const rootRoute = createRootRoute({
  component: Outlet,
  notFoundComponent: () => (
    <Shell>
      <h1>Page not found</h1>
      <p className="sub"><Link to="/recipes">Back to recipes</Link></p>
    </Shell>
  ),
});

const inviteRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/invite/$token',
  component: function InviteRoute() {
    const { token } = useParams({ from: '/invite/$token' });
    return <Onboarding token={token} />;
  },
});

// Auth guard: everything under here needs a signed-in, enabled member.
const authedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authed',
  component: AuthedLayout,
});

function AuthedLayout() {
  const { data: me, isPending, isError, refetch } = useQuery(meQuery);
  if (isPending) return <Shell><Loading /></Shell>;
  if (isError) return <Shell><ErrorState onRetry={() => refetch()} /></Shell>;
  if (me.kind === 'disabled') {
    return (
      <Shell>
        <h1>Your account is disabled</h1>
        <p className="sub" role="alert">Ask the family admin to re-enable it.</p>
      </Shell>
    );
  }
  if (me.kind === 'anonymous') {
    return (
      <Shell>
        <h1>Impressipes</h1>
        <p className="sub">You need an invite link from the family admin to get in.</p>
      </Shell>
    );
  }
  return <Outlet />;
}

const shellRoute = createRoute({ getParentRoute: () => authedRoute, id: 'shell', component: Layout });

function Layout() {
  const { data } = useQuery(meQuery);
  const me = data as Extract<Me, { kind: 'member' }>;
  const [theme, setTheme] = useState<Theme>(getTheme);
  const next: Record<Theme, Theme> = { system: 'light', light: 'dark', dark: 'system' };
  return (
    <div className="min-h-screen">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2">Skip to content</a>
      <header className="border-b border-line bg-surface">
        <nav aria-label="Main" className="mx-auto flex max-w-5xl flex-wrap items-center gap-1 px-4 py-2">
          <Link to="/recipes" className="mr-3 font-display text-xl font-extrabold text-accent no-underline">
            Impressipes
          </Link>
          <Link to="/recipes" className="nav-link">Recipes</Link>
          <Link to="/lists" className="nav-link">Lists</Link>
          {me.member.role === 'admin' && <Link to="/admin" className="nav-link">Family</Link>}
          <span className="grow" />
          <button
            aria-label={`Theme: ${theme}. Switch to ${next[theme]}`}
            onClick={() => { applyTheme(next[theme]); setTheme(next[theme]); }}
          >
            {{ system: 'Auto', light: 'Light', dark: 'Dark' }[theme]}
          </button>
          <span className="dot ml-2 size-8!" style={{ background: me.member.color }} title={me.member.name} />
        </nav>
      </header>
      <main id="main" className="mx-auto max-w-5xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}

const indexRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/',
  beforeLoad: () => { throw redirect({ to: '/recipes' }); },
});

const recipesRoute = createRoute({ getParentRoute: () => shellRoute, path: '/recipes', component: RecipesPage });
const newRecipeRoute = createRoute({ getParentRoute: () => shellRoute, path: '/recipes/new', component: () => <RecipeEditor /> });
const recipeRoute = createRoute({ getParentRoute: () => shellRoute, path: '/recipes/$recipeId', component: RecipePage });
const editRecipeRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/recipes/$recipeId/edit',
  component: function EditRoute() {
    const { recipeId } = useParams({ from: '/authed/shell/recipes/$recipeId/edit' });
    return <RecipeEditor recipeId={recipeId} />;
  },
});
const cookRoute = createRoute({ getParentRoute: () => authedRoute, path: '/recipes/$recipeId/cook', component: CookMode });

const listsRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/lists',
  component: () => (
    <>
      <h1 className="page-title">Lists</h1>
      <EmptyState emoji="🛒" title="No lists yet" body="Your shopping lists will show up here." />
    </>
  ),
});

const adminRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/admin',
  component: function AdminRoute() {
    const { data } = useQuery(meQuery);
    if (data?.kind === 'member' && data.member.role !== 'admin') {
      return <EmptyState emoji="🔒" title="Admins only" body="Ask the family admin if you need something changed." />;
    }
    return <Admin />;
  },
});

const routeTree = rootRoute.addChildren([
  inviteRoute,
  authedRoute.addChildren([
    shellRoute.addChildren([indexRoute, recipesRoute, newRecipeRoute, recipeRoute, editRecipeRoute, listsRoute, adminRoute]),
    cookRoute,
  ]),
]);

export const router = createRouter({ routeTree, defaultPreload: 'intent' });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
