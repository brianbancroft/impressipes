import { useState } from 'react';
import { KeyRound, Mail } from 'lucide-react';
import { login, requestMagicLink, verifyCode, type AuthResult } from './api';
import { Shell } from './Shell';
import { Button } from './ui/button';
import { cn } from './ui/cn';

/** Where to go after signing in: back to the page that asked, or home from /signin itself. */
export function afterSignIn(member: { hasPassword?: boolean }) {
  if (!member.hasPassword) return '/account?welcome=1';
  const { pathname, search } = window.location;
  return pathname === '/signin' ? '/' : pathname + search;
}

export function SignIn() {
  const [mode, setMode] = useState<'password' | 'link'>('password');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  function finish(res: AuthResult) {
    if (res.ok) return window.location.assign(afterSignIn(res.member));
    setError(res.message);
    setBusy(false);
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    if (mode === 'password') return finish(await login(email, password));
    if (sent) return finish(await verifyCode(email, code));
    const res = await requestMagicLink(email);
    if (res.ok) setSent(true);
    else setError(res.message);
    setBusy(false);
  }

  function switchMode(next: 'password' | 'link') {
    setMode(next);
    setError('');
    setSent(false);
    setCode('');
  }

  return (
    <Shell>
      <p className="scribble">welcome back</p>
      <h1 className="mt-1 text-4xl leading-none md:text-5xl">Sign in</h1>
      <p className="mt-2 text-ink-muted">Use your password, or get a one-time link by email.</p>

      <form onSubmit={submit} className="card mt-6 flex flex-col gap-4 p-5 sm:p-6">
        <div className="grid grid-cols-2 gap-1 rounded-md border-2 border-outline bg-surface-2 p-1" role="group" aria-label="Sign-in method">
          {(['password', 'link'] as const).map((m) => (
            <button
              key={m}
              type="button"
              aria-pressed={mode === m}
              onClick={() => mode !== m && switchMode(m)}
              className={cn(
                'inline-flex min-h-10 items-center justify-center gap-2 rounded-sm border-0 text-sm font-bold [&_svg]:size-4',
                mode === m ? 'bg-surface text-ink shadow-[0_0_0_2px_var(--color-outline)]' : 'bg-transparent text-ink-muted hover:text-ink',
              )}
            >
              {m === 'password' ? <><KeyRound />Password</> : <><Mail />Email link</>}
            </button>
          ))}
        </div>

        <div>
          <label htmlFor="email" className="field-label">Email</label>
          <input
            id="email" className="field" type="email" autoComplete="username email" inputMode="email"
            value={email} onChange={(e) => setEmail(e.target.value)} readOnly={sent} required autoFocus
          />
        </div>

        {mode === 'password' && (
          <div>
            <div className="mb-1.5 flex items-baseline justify-between gap-3">
              <label htmlFor="password" className="field-label mb-0">Password</label>
              <Button variant="link" className="min-h-0 text-sm" onClick={() => switchMode('link')}>Forgot it?</Button>
            </div>
            <input
              id="password" className="field" type="password" autoComplete="current-password"
              value={password} onChange={(e) => setPassword(e.target.value)} required
            />
          </div>
        )}

        {mode === 'link' && !sent && (
          <p className="text-sm text-ink-muted">New device or forgot your password? We’ll email a link and a 6-digit code.</p>
        )}

        {mode === 'link' && sent && (
          <div>
            <p role="status" className="mb-3 rounded-md bg-surface-2 p-3 text-sm text-ink-muted">
              If <b className="break-all text-ink">{email}</b> belongs to a member, a link and a 6-digit code are on their way. Open the link, or enter the code here.
            </p>
            <label htmlFor="code" className="field-label">6-digit code</label>
            <input
              id="code" className="field text-center text-2xl tracking-[0.4em]" inputMode="numeric" autoComplete="one-time-code"
              pattern="[0-9]{6}" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} required autoFocus
            />
          </div>
        )}

        {error && <p role="alert" className="err">{error}</p>}

        <Button type="submit" variant="primary" size="lg" className="w-full" disabled={busy || !email.trim()}>
          {mode === 'password' ? 'Sign in' : sent ? 'Sign in with code' : <><Mail />Email me a link</>}
        </Button>

        {sent && (
          <Button variant="link" className="min-h-0 self-center text-sm" onClick={() => { setSent(false); setCode(''); setError(''); }}>Use a different email</Button>
        )}
      </form>
      <p className="mt-5 text-center text-sm text-ink-muted">Joining the family? Ask the admin for an invite link.</p>
    </Shell>
  );
}
