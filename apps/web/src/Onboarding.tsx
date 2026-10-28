import { useEffect, useState } from 'react';
import { acceptInvite, getInvite, type InviteInfo } from './api';
import { Shell } from './App';
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

  if (!invite) return <Shell><p className="sub">Loading…</p></Shell>;
  if (invite.state !== 'valid') {
    const b = blocked[invite.state];
    return (
      <Shell>
        <h1>{b.title}</h1>
        <p className="sub" role="alert">{b.body}</p>
      </Shell>
    );
  }
  if (!passkeysSupported()) {
    return (
      <Shell>
        <h1>Passkeys aren’t available here</h1>
        <p className="sub" role="alert">Open this link in a recent Safari, Chrome or Edge on a device with Face ID, Touch ID or a screen lock.</p>
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
      <h1>{invite.reinvite ? `Welcome back, ${invite.name}` : 'Join the family'}</h1>
      <p className="sub">
        {invite.reinvite
          ? 'Save a new passkey on this device. Your lists and history stay as they were.'
          : 'Pick a name and colour, then save a passkey. No password needed.'}
      </p>
      <div className="card">
        <label htmlFor="name">Name</label>
        <input id="name" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} autoFocus />
        <fieldset>
          <legend>Colour</legend>
          <div className="swatches">
            {COLORS.map((c) => (
              <button
                key={c}
                type="button"
                className="swatch"
                aria-label={c}
                aria-pressed={c === color}
                onClick={() => setColor(c)}
                style={{ background: c }}
              />
            ))}
          </div>
        </fieldset>
        {error && <p role="alert" className="err">{error}</p>}
        <button className="primary" disabled={busy || !name.trim()} onClick={save}>
          {busy ? 'Waiting for passkey…' : 'Save passkey & continue'}
        </button>
      </div>
    </Shell>
  );
}
