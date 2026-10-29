import { useEffect, useState } from 'react';
import { acceptInvite, getInvite, type InviteInfo } from './api';
import { LogIn, UserPlus } from 'lucide-react';
import { Avatar } from './Nav';
import { Shell } from './Shell';
import { SignInForm } from './SignIn';
import { Loading } from './states';
import { Button } from './ui/button';
import { cn } from './ui/cn';
import { Segmented } from './ui/segmented';

const COLORS = ['#e11d48', '#f97316', '#eab308', '#16a34a', '#0891b2', '#2563eb', '#7c3aed', '#db2777'];
const COLOR_NAMES = ['Red', 'Orange', 'Yellow', 'Green', 'Teal', 'Blue', 'Violet', 'Pink'];

const blocked: Record<string, { title: string; body: string }> = {
  used: { title: 'This invite was already used', body: 'Ask the admin for a new link.' },
  expired: { title: 'This invite has expired', body: 'Ask the admin for a new link.' },
  disabled: { title: 'This account is disabled', body: 'Ask the admin to re-enable it.' },
  invalid: { title: "We can't find that invite", body: 'Check the link, or ask the admin for a new one.' },
};

export function Onboarding({ token }: { token: string }) {
  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [tab, setTab] = useState<'join' | 'signin'>('join');

  useEffect(() => {
    getInvite(token).then(setInvite);
  }, [token]);

  if (!invite) return <Shell><Loading /></Shell>;
  if (invite.state !== 'valid') {
    const b = blocked[invite.state];
    return (
      <Shell>
        <h1 className="text-4xl">{b.title}</h1>
        <p className="mt-2 text-ink-muted" role="alert">{b.body}</p>
        <p className="mt-4">Already a member? <a href="/signin">Sign in</a></p>
      </Shell>
    );
  }

  return (
    <Shell>
      <p className="scribble">{invite.reinvite ? 'good to see you' : 'pull up a chair'}</p>
      <h1 className="mt-1 text-4xl leading-none md:text-5xl">
        {tab === 'signin' ? 'Sign in' : invite.reinvite ? `Welcome back, ${invite.name}` : 'Join the family'}
      </h1>
      <p className="mt-2 mb-5 text-ink-muted">
        {tab === 'signin'
          ? 'Already have an account? Sign in and you’re in. This invite stays unused.'
          : invite.reinvite
            ? 'Set a password for this device. Your lists and history stay as they were.'
            : 'You’ve been invited to the family recipe box. Make your account to get started.'}
      </p>
      <Segmented
        label="Join or sign in"
        value={tab}
        onChange={setTab}
        options={[
          { value: 'join', label: <><UserPlus />{invite.reinvite ? 'Set up again' : 'Create account'}</> },
          { value: 'signin', label: <><LogIn />I have an account</> },
        ]}
      />
      {tab === 'join'
        ? <JoinForm token={token} invite={invite} onSignIn={() => setTab('signin')} />
        : <SignInForm className="mt-4" />}
    </Shell>
  );
}

function JoinForm({ token, invite, onSignIn }: { token: string; invite: InviteInfo; onSignIn: () => void }) {
  const [name, setName] = useState(invite.name ?? '');
  const [color, setColor] = useState(invite.color ?? COLORS[0]);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [taken, setTaken] = useState(false);

  const shortPassword = password.length > 0 && password.length < 10;
  const mismatch = confirm.length > 0 && confirm !== password;
  const ready = !!name.trim() && !!email.trim() && password.length >= 10 && confirm === password;

  async function save() {
    if (!ready) return;
    setBusy(true);
    setError('');
    setTaken(false);
    const res = await acceptInvite(token, {
      name: name.trim(),
      color,
      // The server still wants a credential id; sign-in is by email and password now.
      credentialId: `web-${crypto.randomUUID()}`,
      email: email.trim(),
      password,
    });
    // Home shows the getting-started checklist, starting with the emailed code.
    if (res.ok) return window.location.assign('/');
    setTaken(res.status === 409);
    setError(
      res.status === 410
        ? 'This invite was just used or expired. Ask the admin for a new link.'
        : res.status === 403
          ? 'This account is disabled.'
          : res.status === 409
            ? 'That email already has an account.'
            : res.status === 400
              ? res.message
              : 'Something went wrong. Try again.',
    );
    setBusy(false);
  }

  return (
    <form className="card mt-4 flex flex-col gap-5 p-5 sm:p-6" onSubmit={(e) => { e.preventDefault(); save(); }}>
      <div className="flex items-end gap-4">
        <Avatar member={{ name: name || '?', color }} className="size-16 text-3xl shadow-pop-sm" />
        <div className="min-w-0 flex-1">
          <label htmlFor="join-name" className="field-label">Username</label>
          <input id="join-name" className="field" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} autoComplete="nickname" required autoFocus />
        </div>
      </div>
      <fieldset className="m-0 -mt-1 min-w-0 border-0 p-0">
        <legend className="field-label">Colour</legend>
        <div className="grid grid-cols-8 gap-2 sm:gap-2.5">
          {COLORS.map((c, i) => (
            <button
              key={c}
              type="button"
              className={cn('aspect-square w-full cursor-pointer rounded-full border-[2.5px] border-outline p-0 transition-transform', c === color && 'scale-110 shadow-pop-sm ring-4 ring-accent-soft')}
              aria-label={COLOR_NAMES[i]}
              aria-pressed={c === color}
              onClick={() => setColor(c)}
              style={{ background: c }}
            />
          ))}
        </div>
        <p className="mt-1.5 text-sm text-ink-muted">Your name and colour show next to everything you add.</p>
      </fieldset>

      <div className="flex flex-col gap-4 border-t-[1.5px] border-dashed border-line pt-5">
        <div>
          <label htmlFor="join-email" className="field-label">Email</label>
          <input id="join-email" className="field" type="email" autoComplete="email" inputMode="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          <p className="mt-1 text-sm text-ink-muted">You’ll sign in with this. We’ll send a code to confirm it.</p>
        </div>
        <div>
          <label htmlFor="join-password" className="field-label">Password</label>
          <input
            id="join-password" className="field" type="password" autoComplete="new-password" minLength={10} required
            value={password} onChange={(e) => setPassword(e.target.value)} aria-invalid={shortPassword || undefined} aria-describedby="join-password-hint"
          />
          <p id="join-password-hint" className={cn('mt-1 text-sm', shortPassword ? 'text-danger' : 'text-ink-muted')}>At least 10 characters.</p>
        </div>
        <div>
          <label htmlFor="join-confirm" className="field-label">Confirm password</label>
          <input
            id="join-confirm" className="field" type="password" autoComplete="new-password" required
            value={confirm} onChange={(e) => setConfirm(e.target.value)} aria-invalid={mismatch || undefined} aria-describedby="join-confirm-hint"
          />
          {mismatch && <p id="join-confirm-hint" className="mt-1 text-sm text-danger">The passwords don’t match yet.</p>}
        </div>
      </div>
      {error && (
        <p role="alert" className="err">
          {error}
          {taken && <> <Button variant="link" className="min-h-0 align-baseline" onClick={onSignIn}>Sign in instead</Button></>}
        </p>
      )}
      <Button type="submit" variant="primary" size="lg" className="w-full" disabled={busy || !ready}>
        {busy ? 'Creating your account…' : invite.reinvite ? 'Save and continue' : 'Create account'}
      </Button>
    </form>
  );
}
