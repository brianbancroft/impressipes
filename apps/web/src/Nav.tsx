import { useQuery } from '@tanstack/react-query';
import { Link, useRouterState } from '@tanstack/react-router';
import { Bot, Heart, Plus, ShoppingCart, Smartphone, Soup, UserRound, Users } from 'lucide-react';
import { useState } from 'react';
import { listLists, type Member } from './api';
import { applyTheme, getTheme, type Theme } from './theme';
import { cn } from './ui/cn';
import { LogoMark, Wordmark } from './ui/logo';
import { Menu, MenuContent, MenuItem, MenuLabel, MenuRadioGroup, MenuRadioItem, MenuSeparator, MenuTrigger } from './ui/menu';

function useActive() {
  const loc = useRouterState({ select: (s) => s.location });
  const fav = (loc.search as { mark?: string }).mark === 'favourite';
  return {
    recipes: loc.pathname.startsWith('/recipes') && !fav,
    faves: loc.pathname === '/recipes' && fav,
    lists: loc.pathname.startsWith('/lists'),
    family: loc.pathname.startsWith('/admin'),
    listId: loc.pathname.match(/^\/lists\/([^/]+)/)?.[1],
  };
}

export function Avatar({ member, className }: { member: Pick<Member, 'name' | 'color'>; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn('grid size-11 flex-none place-items-center rounded-full border-[2.5px] border-outline font-display text-lg font-extrabold text-sticker-ink', className)}
      style={{ background: `color-mix(in oklab, ${member.color} 45%, #fff)` }}
    >
      {member.name.trim().charAt(0).toUpperCase() || '?'}
    </span>
  );
}

export function ProfileMenu({ me, children }: { me: Member; children?: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>(getTheme);
  return (
    <Menu>
      <MenuTrigger aria-label={`${me.name}: profile and theme`} className="rounded-full">
        {children ?? <Avatar member={me} />}
      </MenuTrigger>
      <MenuContent className="w-56">
        <MenuLabel>Signed in as {me.name}</MenuLabel>
        {me.role === 'admin' && (
          <MenuItem asChild><Link to="/admin" className="text-ink no-underline hover:text-ink"><Users />Family</Link></MenuItem>
        )}
        <MenuItem asChild><Link to="/account" className="text-ink no-underline hover:text-ink"><UserRound />Account</Link></MenuItem>
        <MenuItem asChild><Link to="/ai" className="text-ink no-underline hover:text-ink"><Bot />AI integration</Link></MenuItem>
        <MenuItem asChild><Link to="/app-setup" className="text-ink no-underline hover:text-ink"><Smartphone />App setup</Link></MenuItem>
        <MenuSeparator />
        <MenuLabel>Theme</MenuLabel>
        <MenuRadioGroup value={theme} onValueChange={(v) => { applyTheme(v as Theme); setTheme(v as Theme); }}>
          <MenuRadioItem value="system">Match device</MenuRadioItem>
          <MenuRadioItem value="light">Light</MenuRadioItem>
          <MenuRadioItem value="dark">Dark</MenuRadioItem>
        </MenuRadioGroup>
      </MenuContent>
    </Menu>
  );
}

const railLink = 'flex min-h-11.5 items-center gap-3 rounded-md px-3.5 font-bold text-on-rail no-underline hover:bg-white/10 hover:text-on-rail max-lg:justify-center max-lg:px-0 [&_svg]:size-6 [&_svg]:flex-none';
const railOn = 'bg-butter text-sticker-ink hover:bg-butter hover:text-sticker-ink';

/** Dark side rail: icons only from 768px, full labels and lists from 1024px. */
export function SideRail({ me }: { me: Member }) {
  const a = useActive();
  const lists = useQuery({ queryKey: ['lists'], queryFn: listLists });
  return (
    <nav aria-label="Main" className="fixed top-4 bottom-4 left-4 z-20 hidden w-18 flex-col gap-5 overflow-y-auto rounded-xl bg-rail px-2.5 py-5 text-on-rail md:flex lg:w-54 lg:px-3.5">
      <Link to="/recipes" className="flex items-center gap-2.5 px-1.5 text-on-rail no-underline hover:text-on-rail max-lg:justify-center">
        <LogoMark className="border-on-rail" /><Wordmark className="max-lg:sr-only" />
      </Link>
      <Link
        to="/recipes/new"
        aria-label="New recipe"
        className="flex min-h-13 items-center justify-center gap-2 rounded-md border-[2.5px] border-on-rail bg-accent-fill font-bold text-white no-underline shadow-[3px_3px_0_var(--color-butter)] hover:-translate-y-px hover:text-white"
      >
        <Plus className="size-5" strokeWidth={2.8} /><span className="max-lg:sr-only">New recipe</span>
      </Link>
      <div className="flex flex-col gap-1">
        <Link to="/recipes" className={cn(railLink, a.recipes && railOn)} aria-current={a.recipes ? 'page' : undefined}><Soup /><span className="max-lg:sr-only">Recipes</span></Link>
        <Link to="/lists" className={cn(railLink, a.lists && railOn)} aria-current={a.lists ? 'page' : undefined}><ShoppingCart /><span className="max-lg:sr-only">Lists</span></Link>
        {me.role === 'admin' && (
          <Link to="/admin" className={cn(railLink, a.family && railOn)} aria-current={a.family ? 'page' : undefined}><Users /><span className="max-lg:sr-only">Family</span></Link>
        )}
        <Link to="/recipes" search={{ mark: 'favourite' }} className={cn(railLink, a.faves && railOn)} aria-current={a.faves ? 'page' : undefined}><Heart /><span className="max-lg:sr-only">Faves</span></Link>
      </div>
      {!!lists.data?.length && (
        <div className="hidden flex-col gap-0.5 lg:flex">
          <span className="scribble px-3.5 pb-1 text-xl text-butter">your lists</span>
          {lists.data.map((l) => (
            <Link
              key={l.id}
              to="/lists/$listId"
              params={{ listId: l.id }}
              className={cn('flex min-h-10 items-center gap-2.5 rounded-sm px-3.5 font-medium text-on-rail no-underline hover:bg-white/10 hover:text-on-rail', a.listId === l.id && 'bg-white/15')}
            >
              <span className="size-3 flex-none rounded-full border-2 border-on-rail" style={{ background: l.colour }} aria-hidden />
              <span className="truncate">{l.name}</span>
            </Link>
          ))}
        </div>
      )}
      <div className="mt-auto">
        <ProfileMenu me={me}>
          <span className="flex w-full items-center gap-2.5 rounded-lg bg-white/8 p-2 text-left hover:bg-white/14 max-lg:justify-center max-lg:bg-transparent max-lg:p-0">
            <Avatar member={me} className="size-10 border-on-rail" />
            <span className="min-w-0 flex-1 leading-tight max-lg:hidden">
              <span className="block truncate font-bold">{me.name}</span>
              <span className="block text-sm opacity-75">Profile and theme</span>
            </span>
          </span>
        </ProfileMenu>
      </div>
    </nav>
  );
}

const tab = 'flex flex-col items-center gap-0.5 text-[11px] font-bold text-on-rail no-underline hover:text-on-rail [&_svg]:size-6';

/** Phone tab bar with the New recipe button raised in the middle. */
export function TabBar({ admin }: { admin: boolean }) {
  const a = useActive();
  return (
    <nav aria-label="Main" className={cn('fixed inset-x-3.5 bottom-4 z-20 grid h-18 items-center rounded-[26px] bg-rail md:hidden', admin ? 'grid-cols-5' : 'grid-cols-4')}>
      <Link to="/recipes" className={cn(tab, a.recipes && 'text-butter hover:text-butter')} aria-current={a.recipes ? 'page' : undefined}><Soup strokeWidth={2.2} />Recipes</Link>
      <Link to="/lists" className={cn(tab, a.lists && 'text-butter hover:text-butter')} aria-current={a.lists ? 'page' : undefined}><ShoppingCart strokeWidth={2.2} />Lists</Link>
      <Link
        to="/recipes/new"
        aria-label="New recipe"
        className="-mt-9 grid size-15 place-items-center justify-self-center rounded-full border-[3px] border-outline bg-accent-fill text-white shadow-[0_0_0_4px_var(--color-bg)] hover:text-white"
      >
        <Plus className="size-7" strokeWidth={2.8} />
      </Link>
      <Link to="/recipes" search={{ mark: 'favourite' }} className={cn(tab, a.faves && 'text-butter hover:text-butter')} aria-current={a.faves ? 'page' : undefined}><Heart strokeWidth={2.2} />Faves</Link>
      {admin && <Link to="/admin" className={cn(tab, a.family && 'text-butter hover:text-butter')} aria-current={a.family ? 'page' : undefined}><Users strokeWidth={2.2} />Family</Link>}
    </nav>
  );
}
