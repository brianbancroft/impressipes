import { useEffect, useState } from 'react';
import { getMe, type Me } from './api';
import { Admin } from './Admin';
import { Onboarding } from './Onboarding';
import './styles.css';

export function Shell({ children, wide }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <main className="shell">
      <div className={wide ? 'stack wide' : 'stack'}>
        <div className="mark" aria-hidden>I</div>
        {children}
      </div>
    </main>
  );
}

export function App() {
  const path = window.location.pathname;
  const invite = path.match(/^\/invite\/([^/]+)$/);
  return invite ? <Onboarding token={invite[1]} /> : <Authed path={path} />;
}

function Authed({ path }: { path: string }) {
  const [me, setMe] = useState<Me | null>(null);
  useEffect(() => void getMe().then(setMe), []);

  if (!me) return <Shell><p className="sub">Loading…</p></Shell>;
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
        <h1>Impressipies</h1>
        <p className="sub">You need an invite link from the family admin to get in.</p>
      </Shell>
    );
  }
  if (path === '/admin' && me.member.role === 'admin') return <Admin />;

  return (
    <Shell>
      <h1><span style={{ color: me.member.color }}>●</span> Main list</h1>
      <p className="sub">Hi {me.member.name}. Your lists will show up here.</p>
      {me.member.role === 'admin' && <a href="/admin">Manage family</a>}
    </Shell>
  );
}
