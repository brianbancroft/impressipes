import { useEffect, useState } from 'react';
import { acceptInvite, getInvite, type InviteInfo } from './api';
import { Avatar } from './Nav';
import { Shell } from './Shell';
import { Loading } from './states';
import { Button } from './ui/button';
import { cn } from './ui/cn';

const COLORS = ['#e11d48', '#f97316', '#eab308', '#16a34a', '#0891b2', '#2563eb', '#7c3aed', '#db2777'];

const blocked: Record<string, { title: string; body: string }> = {
  used: { title: 'This invite was already used', body: 'Ask the admin for a new link.' },
  expired: { title: 'This invite has expired', body: 'Ask the admin for a new link.' },
  disabled: { title: 'This account is disabled', body: 'Ask the admin to re-enable it.' },
  invalid: { title: "We can't find that invite", body: 'Check the link, or ask the admin for a new one.' },
};

export function Onboarding({ token }: { token: string }) {
  const [invite, setInvite] = useState<InviteInfo | null>(null);
  const [name, setName] = useState('');
  const [color, setColor] = useState(COLORS[0]);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    getInvite(token).then((i) => {
      setInvite(i);
      if (i.name) setName(i.name);
      if (i.color) setColor(i.color);
    });
  }, [token]);

  if (!invite) return <Shell><Loading /></Shell>;
  if (invite.state !== 'valid') {
    const b = blocked[invite.state];
    return (
      <Shell>
        <h1 className="text-4xl">{b.title}</h1>
        <p className="mt-2 text-ink-muted" role="alert">{b.body}</p>
      </Shell>
    );
  }
  async function save() {
    setBusy(true);
    setError('');
    const res = await acceptInvite(token, {
      name: name.trim(),
      color,
      // The server still wants a credential id; sign-in is by email and password now.
      credentialId: `web-${crypto.randomUUID()}`,
      ...(email.trim() ? { email: email.trim() } : {}),
      ...(password ? { password } : {}),
    });
    if (res.ok) {
      // A confirmation code is on its way when an email was given; the account page takes it.
      window.location.assign(email.trim() ? `/account?email=${encodeURIComponent(email.trim().toLowerCase())}` : '/');
      return;
    }
    setError(
      res.status === 410
        ? 'This invite was just used or expired. Ask the admin for a new link.'
        : res.status === 403
          ? 'This account is disabled.'
          : res.status === 400 || res.status === 409
            ? res.message
            : 'Something went wrong. Try again.',
    );
    setBusy(false);
  }

  return (
    <Shell>
      <p className="scribble">{invite.reinvite ? 'good to see you' : 'pull up a chair'}</p>
      <h1 className="mt-1 text-4xl leading-none">{invite.reinvite ? `Welcome back, ${invite.name}` : 'Join the family'}</h1>
      <p className="mt-2 mb-6 text-ink-muted">
        {invite.reinvite
          ? 'Confirm your name and colour, and add an email and password if you like. Your lists and history stay as they were.'
          : 'Pick a name and colour. Add an email and password to sign in on other devices.'}
      </p>
      <div className="card flex flex-col gap-5 p-5">
        <div className="flex items-center gap-3">
          <Avatar member={{ name: name || '?', color }} className="size-14 text-2xl" />
          <div className="min-w-0 flex-1">
            <label htmlFor="name" className="field-label">Name</label>
            <input id="name" className="field" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} autoFocus />
          </div>
        </div>
        <fieldset className="m-0 border-0 p-0">
          <legend className="field-label">Colour</legend>
          <div className="flex flex-wrap gap-2.5">
            {COLORS.map((c) => (
              <button
                key={c}
                type="button"
                className={cn('size-10 cursor-pointer rounded-full border-[2.5px] border-outline p-0 transition-transform', c === color && 'scale-110 shadow-pop-sm ring-4 ring-accent-soft')}
                aria-label={c}
                aria-pressed={c === color}
                onClick={() => setColor(c)}
                style={{ background: c }}
              />
            ))}
          </div>
        </fieldset>
        <div>
          <label htmlFor="email" className="field-label">Email <span className="font-normal text-ink-muted">(optional)</span></label>
          <input id="email" className="field" type="email" autoComplete="email" value={email} onChange={(e) => setEmail(e.target.value)} />
          <p className="mt-1 text-sm text-ink-muted">We’ll send a code to confirm it. Once confirmed you can sign in with an emailed link.</p>
        </div>
        <div>
          <label htmlFor="password" className="field-label">Password <span className="font-normal text-ink-muted">(optional)</span></label>
          <input id="password" className="field" type="password" autoComplete="new-password" minLength={10} value={password} onChange={(e) => setPassword(e.target.value)} />
          <p className="mt-1 text-sm text-ink-muted">At least 10 characters.</p>
        </div>
        {error && <p role="alert" className="err">{error}</p>}
        <Button variant="primary" size="lg" className="w-full" disabled={busy || !name.trim() || (!!password && password.length < 10)} onClick={save}>
          {busy ? 'Saving…' : 'Let’s go'}
        </Button>
      </div>
    </Shell>
  );
}
