import { useEffect, useState } from 'react';
import { acceptInvite, getInvite, type InviteInfo } from './api';
import { KeyRound } from 'lucide-react';
import { Avatar } from './Nav';
import { Shell } from './Shell';
import { Loading } from './states';
import { Button } from './ui/button';
import { cn } from './ui/cn';
import { createPasskey, passkeysSupported } from './passkey';

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
  if (!passkeysSupported()) {
    return (
      <Shell>
        <h1 className="text-4xl">Passkeys aren’t available here</h1>
        <p className="mt-2 text-ink-muted" role="alert">Open this link in a recent Safari, Chrome or Edge on a device with Face ID, Touch ID or a screen lock.</p>
      </Shell>
    );
  }

  async function save() {
    setBusy(true);
    setError('');
    try {
      const credentialId = await createPasskey(name.trim());
      const res = await acceptInvite(token, { name: name.trim(), color, credentialId });
      if (res.ok) {
        window.location.assign('/');
        return;
      }
      setError(
        res.reason === 'gone'
          ? 'This invite was just used or expired. Ask the admin for a new link.'
          : res.reason === 'disabled'
            ? 'This account is disabled.'
            : 'Something went wrong. Try again.',
      );
    } catch {
      setError('Passkey wasn’t saved. You can try again.');
    }
    setBusy(false);
  }

  return (
    <Shell>
      <p className="scribble">{invite.reinvite ? 'good to see you' : 'pull up a chair'}</p>
      <h1 className="mt-1 text-4xl leading-none">{invite.reinvite ? `Welcome back, ${invite.name}` : 'Join the family'}</h1>
      <p className="mt-2 mb-6 text-ink-muted">
        {invite.reinvite
          ? 'Save a new passkey on this device. Your lists and history stay as they were.'
          : 'Pick a name and colour, then save a passkey. No password needed.'}
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
        {error && <p role="alert" className="err">{error}</p>}
        <Button variant="primary" size="lg" className="w-full" disabled={busy || !name.trim()} onClick={save}>
          <KeyRound />{busy ? 'Waiting for passkey…' : 'Save passkey & continue'}
        </Button>
      </div>
    </Shell>
  );
}
