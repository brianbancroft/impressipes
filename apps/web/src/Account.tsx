import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useState } from 'react';
import { getMe, setEmail, setPassword, verifyCode } from './api';
import { Button } from './ui/button';
import { Blob } from './ui/sticker';
import { Toast } from './ui/toast';

export function Account() {
  const me = useQuery({ queryKey: ['me'], queryFn: getMe, staleTime: 60_000 }).data;
  const member = me?.kind === 'member' ? me.member : undefined;
  const { welcome, email: pending } = useSearch({ from: '/authed/shell/account' });
  const [toast, setToast] = useState('');

  function say(msg: string) {
    setToast(msg);
    setTimeout(() => setToast(''), 2500);
  }

  return (
    <div className="relative isolate max-w-180">
      <Blob shape="blob-b" className="-top-24 -right-20 size-64 bg-lilac" />
      <div className="relative mb-6">
        <p className="scribble md:text-[26px]">{welcome ? 'one more thing' : 'just for you'}</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">Account</h1>
        {welcome && (
          <p className="mt-2 text-ink-muted">You’re signed in. Set a password so next time you can skip the email.</p>
        )}
      </div>
      <PasswordCard hasPassword={!!member?.hasPassword} onSaved={() => say('Password saved')} />
      <EmailCard current={member?.email} pending={pending} onConfirmed={() => say('Email confirmed')} />
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}

function PasswordCard({ hasPassword, onSaved }: { hasPassword: boolean; onSaved: () => void }) {
  const qc = useQueryClient();
  const [pw, setPw] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function save(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    const res = await setPassword(pw);
    setBusy(false);
    if (!res.ok) return setError(res.message);
    setPw('');
    await qc.invalidateQueries({ queryKey: ['me'] });
    onSaved();
  }

  return (
    <form onSubmit={save} className="card relative mb-6 flex flex-col gap-3 p-5">
      <h2 className="text-2xl">Password</h2>
      <p className="text-ink-muted">{hasPassword ? 'You have a password. Set a new one to replace it.' : 'No password yet. You sign in with an emailed link.'}</p>
      <div>
        <label htmlFor="new-password" className="field-label">New password</label>
        <input
          id="new-password" className="field" type="password" autoComplete="new-password" minLength={10}
          value={pw} onChange={(e) => setPw(e.target.value)} required
        />
        <p className="mt-1 text-sm text-ink-muted">At least 10 characters.</p>
      </div>
      {error && <p role="alert" className="err">{error}</p>}
      <Button type="submit" variant="primary" className="self-start" disabled={busy || pw.length < 10}>Save password</Button>
    </form>
  );
}

function EmailCard({ current, pending, onConfirmed }: { current?: string; pending?: string; onConfirmed: () => void }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [email, setEmailText] = useState(pending ?? '');
  const [sentTo, setSentTo] = useState(pending ?? '');
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  async function send(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    const res = await setEmail(email);
    setBusy(false);
    if (res.ok) setSentTo(email.trim().toLowerCase());
    else setError(res.message);
  }

  async function confirm(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    const res = await verifyCode(sentTo, code);
    setBusy(false);
    if (!res.ok) return setError(res.message);
    setSentTo('');
    setEmailText('');
    setCode('');
    await qc.invalidateQueries({ queryKey: ['me'] });
    await navigate({ to: '/account', search: {}, replace: true });
    onConfirmed();
  }

  return (
    <section className="card relative flex flex-col gap-3 p-5">
      <h2 className="text-2xl">Email</h2>
      <p className="text-ink-muted">
        {current ? <>Signed-in links and codes go to <b>{current}</b>.</> : 'No confirmed email yet. Add one to sign in on other devices.'}
      </p>
      {sentTo ? (
        <form onSubmit={confirm} className="flex flex-col gap-3">
          <p role="status">We sent a link and a 6-digit code to <b>{sentTo}</b>. Open the link, or enter the code here.</p>
          <div>
            <label htmlFor="confirm-code" className="field-label">6-digit code</label>
            <input
              id="confirm-code" className="field text-center text-2xl tracking-[0.4em]" inputMode="numeric" autoComplete="one-time-code"
              pattern="[0-9]{6}" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} required
            />
          </div>
          {error && <p role="alert" className="err">{error}</p>}
          <div className="flex flex-wrap items-center gap-3">
            <Button type="submit" variant="primary" disabled={busy || code.length !== 6}>Confirm email</Button>
            <Button variant="link" onClick={() => { setSentTo(''); setCode(''); setError(''); }}>Use a different email</Button>
          </div>
        </form>
      ) : (
        <form onSubmit={send} className="flex flex-col gap-3">
          <div>
            <label htmlFor="account-email" className="field-label">{current ? 'Change email' : 'Email'}</label>
            <input id="account-email" className="field" type="email" autoComplete="email" value={email} onChange={(e) => setEmailText(e.target.value)} required />
          </div>
          {error && <p role="alert" className="err">{error}</p>}
          <Button type="submit" variant="primary" className="self-start" disabled={busy || !email.trim()}>Send confirmation</Button>
        </form>
      )}
    </section>
  );
}
