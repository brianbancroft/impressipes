import { useQuery } from '@tanstack/react-query';
import {
  createRootRoute,
  createRoute,
  createRouter,
  Link,
  Outlet,
  redirect,
  useParams,
  useSearch,
  useRouterState,
} from '@tanstack/react-router';
import { Account } from './Account';
import { Admin } from './Admin';
import { AiIntegration } from './AiIntegration';
import { AppSetup } from './AppSetup';
import { getMe, type Me, type RecipeMark } from './api';
import { CookMode } from './CookMode';
import { HomePage } from './HomePage';
import { ListPage } from './ListPage';
import { ListsPage } from './ListsPage';
import { Onboarding } from './Onboarding';
import { RecipeEditor } from './RecipeEditor';
import { RecipePage } from './RecipePage';
import { RecipesPage } from './RecipesPage';
import { SignIn } from './SignIn';
import { VerifyLink } from './VerifyLink';
import { EmptyState, ErrorState, Loading } from './states';
import { Lock } from 'lucide-react';
import { Shell } from './Shell';
import { LogoMark, Wordmark } from './ui/logo';
import { cn } from './ui/cn';
import { ProfileMenu, SideRail, TabBar } from './Nav';

const meQuery = { queryKey: ['me'], queryFn: getMe, staleTime: 60_000 } as const;

const rootRoute = createRootRoute({
  component: Outlet,
  notFoundComponent: () => (
    <Shell>
      <h1 className="text-4xl">Page not found</h1>
      <p className="mt-2 text-ink-muted"><Link to="/home">Back home</Link></p>
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

const signInRoute = createRoute({ getParentRoute: () => rootRoute, path: '/signin', component: SignIn });

// The emailed link lands here; the token in ?t= is redeemed once.
const verifyRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/auth/verify',
  validateSearch: (s: Record<string, unknown>): { t?: string } => (typeof s.t === 'string' ? { t: s.t } : {}),
  component: function VerifyRoute() {
    const { t } = useSearch({ from: '/auth/verify' });
    return t ? <VerifyLink token={t} /> : <SignIn />;
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
  if (isPending) return <div className="grid min-h-dvh place-items-center"><Loading /></div>;
  if (isError) return <Shell><ErrorState onRetry={() => refetch()} /></Shell>;
  if (me.kind === 'disabled') {
    return (
      <Shell>
        <h1 className="text-4xl">Your account is disabled</h1>
        <p className="mt-2 text-ink-muted" role="alert">Ask the family admin to re-enable it.</p>
      </Shell>
    );
  }
  if (me.kind === 'anonymous') return <SignIn />;
  return <Outlet />;
}

const shellRoute = createRoute({ getParentRoute: () => authedRoute, id: 'shell', component: Layout });

function Layout() {
  const { data } = useQuery(meQuery);
  const me = data as Extract<Me, { kind: 'member' }>;
  const path = useRouterState({ select: (st) => st.location.pathname });
  // Detail screens bring their own header and actions, so phones drop the tab bar there.
  const top = ['/home', '/recipes', '/lists', '/admin', '/ai', '/app-setup', '/account'].includes(path);
  return (
    <div className="relative isolate min-h-dvh overflow-x-clip md:pl-24 lg:pl-62">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:top-2 focus:left-2 focus:z-50">Skip to content</a>
      <SideRail me={me.member} />
      {top && (
        <header className="relative z-10 flex items-center justify-between px-4 pt-4 md:hidden">
          <Link to="/home" className="flex items-center gap-2 text-ink no-underline hover:text-ink"><LogoMark /><Wordmark /></Link>
          <ProfileMenu me={me.member} />
        </header>
      )}
      <main id="main" className={cn('page-content mx-auto w-full max-w-6xl px-4 pt-4 md:px-8 md:pt-8', top ? 'pb-32 md:pb-12' : 'pb-12')}>
        <Outlet />
      </main>
      {top && <TabBar />}
    </div>
  );
}

const indexRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/',
  beforeLoad: () => { throw redirect({ to: '/home' }); },
});

const homeRoute = createRoute({ getParentRoute: () => shellRoute, path: '/home', component: HomePage });

const recipesRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/recipes',
  component: RecipesPage,
  validateSearch: (s: Record<string, unknown>): { mark?: RecipeMark } =>
    s.mark === 'favourite' || s.mark === 'want_to_make' ? { mark: s.mark } : {},
});
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

const listsRoute = createRoute({ getParentRoute: () => shellRoute, path: '/lists', component: ListsPage });
const listRoute = createRoute({ getParentRoute: () => shellRoute, path: '/lists/$listId', component: ListPage });

const adminRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/admin',
  component: function AdminRoute() {
    const { data } = useQuery(meQuery);
    if (data?.kind === 'member' && data.member.role !== 'admin') {
      return <EmptyState icon={Lock} title="Admins only" body="Ask the family admin if you need something changed." />;
    }
    return <Admin />;
  },
});

const aiRoute = createRoute({ getParentRoute: () => shellRoute, path: '/ai', component: AiIntegration });
const accountRoute = createRoute({
  getParentRoute: () => shellRoute,
  path: '/account',
  component: Account,
  validateSearch: (s: Record<string, unknown>): { welcome?: boolean; email?: string } => ({
    ...(s.welcome ? { welcome: true } : {}),
    ...(typeof s.email === 'string' ? { email: s.email } : {}),
  }),
});
const appSetupRoute = createRoute({ getParentRoute: () => shellRoute, path: '/app-setup', component: AppSetup });

const routeTree = rootRoute.addChildren([
  inviteRoute,
  signInRoute,
  verifyRoute,
  authedRoute.addChildren([
    shellRoute.addChildren([indexRoute, homeRoute, recipesRoute, newRecipeRoute, recipeRoute, editRecipeRoute, listsRoute, listRoute, adminRoute, aiRoute, appSetupRoute, accountRoute]),
    cookRoute,
  ]),
]);

export const router = createRouter({ routeTree, defaultPreload: 'intent', defaultViewTransition: true });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
