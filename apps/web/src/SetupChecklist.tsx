import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@tanstack/react-router';
import { ArrowRight, Check, Mail, Smartphone, Sparkles, X } from 'lucide-react';
import { useState } from 'react';
import { getMe, setEmail, verifyCode, type Member } from './api';
import { Button } from './ui/button';
import { cn } from './ui/cn';

const hiddenKey = (id: string) => `setup-hidden:${id}`;

function readHidden(id: string) {
  try {
    return localStorage.getItem(hiddenKey(id)) === '1';
  } catch {
    return false;
  }
}

/**
 * Whether getting started should show: until the email is confirmed, the phone app
 * and an AI assistant are connected (each ticks itself off from /me), unless hidden.
 */
export function useSetupChecklist() {
  // Refetch on focus, so a step finished in another tab or on the phone ticks off on return.
  const me = useQuery({ queryKey: ['me'], queryFn: getMe, staleTime: 60_000, refetchOnWindowFocus: 'always' });
  const member = me.data?.kind === 'member' ? me.data.member : undefined;
  const [hidden, setHidden] = useState(() => (member ? readHidden(member.id) : false));
  const steps = member?.setup ? [!!member.email, member.setup.appConnected, member.setup.aiConnected] : [];
  const complete = steps.length > 0 && steps.every(Boolean);

  function hide() {
    try {
      localStorage.setItem(hiddenKey(member!.id), '1');
    } catch {
      // Private mode: it stays hidden until the page reloads.
    }
    setHidden(true);
  }

  return { member, steps, show: !!member?.setup && !complete && !hidden, loading: me.isPending, hide };
}

export function SetupChecklist({ state }: { state: ReturnType<typeof useSetupChecklist> }) {
  const { member, steps, show, hide } = state;
  if (!member?.setup || !show) return null;
  const done = steps.filter(Boolean).length;
  const next = steps.indexOf(false);

  return (
    <section aria-labelledby="setup-title" className="card relative mb-8 p-5 sm:p-6">
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <p className="scribble">let’s get you settled</p>
          <h2 id="setup-title" className="text-2xl md:text-3xl">Getting started</h2>
          <p className="mt-1 text-sm text-ink-muted">{done} of {steps.length} done. Each one takes a minute or two.</p>
        </div>
        <Button variant="ghost" size="icon-sm" aria-label="Hide getting started" title="Hide for now" onClick={hide}><X /></Button>
      </div>
      <div className="mb-5 h-2.5 overflow-hidden rounded-full border-2 border-outline bg-surface-2" aria-hidden>
        <div className="h-full bg-accent-fill transition-[width]" style={{ width: `${(done / steps.length) * 100}%` }} />
      </div>
      <ol className="m-0 flex list-none flex-col gap-3 p-0">
        <Step n={1} done={steps[0]} current={next === 0} icon={Mail} title="Confirm your email">
          <EmailStep member={member} />
        </Step>
        <Step n={2} done={steps[1]} current={next === 1} icon={Smartphone} title="Get the phone app">
          <p className="text-ink-muted">Take your recipes and the shopping list to the store. Scan a code and the app sets itself up.</p>
          <Button asChild size="sm" variant={next === 1 ? 'primary' : 'flat'} className="mt-3 self-start">
            <Link to="/app-setup">Set up the app<ArrowRight /></Link>
          </Button>
        </Step>
        <Step n={3} done={steps[2]} current={next === 2} icon={Sparkles} title="Connect an AI assistant">
          <p className="text-ink-muted">Snap a recipe card and your assistant types it in. Works with Claude, ChatGPT, Grok and more. It’s one copy and paste.</p>
          <Button asChild size="sm" variant={next === 2 ? 'primary' : 'flat'} className="mt-3 self-start">
            <Link to="/ai">Connect an assistant<ArrowRight /></Link>
          </Button>
        </Step>
      </ol>
    </section>
  );
}

function Step({ n, done, current, icon: Icon, title, children }: {
  n: number; done: boolean; current: boolean; icon: React.ComponentType<{ className?: string }>; title: string; children: React.ReactNode;
}) {
  return (
    <li className={cn('flex gap-3.5 rounded-md border-2 p-4', current ? 'border-outline bg-surface' : 'border-line bg-surface-2/50', done && 'opacity-75')}>
      <span
        className={cn(
          'grid size-9 flex-none place-items-center rounded-full border-2 border-outline font-display text-lg font-extrabold',
          done ? 'bg-mint text-sticker-ink' : current ? 'bg-butter text-sticker-ink' : 'bg-surface text-ink-muted',
        )}
        aria-hidden
      >
        {done ? <Check className="size-5" /> : n}
      </span>
      <div className="flex min-w-0 flex-1 flex-col">
        <h3 className="flex items-center gap-2 text-lg leading-tight">
          <Icon className="size-4.5 flex-none text-accent" />
          <span className={cn(done && 'line-through decoration-2')}>{title}</span>
          <span className="sr-only">{done ? '(done)' : '(to do)'}</span>
        </h3>
        {!done && <div className="mt-1 flex flex-col">{children}</div>}
      </div>
    </li>
  );
}

function EmailStep({ member }: { member: Member }) {
  const qc = useQueryClient();
  const pending = member.setup?.pendingEmail;
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState('');
  const [error, setError] = useState('');

  if (!pending) {
    return (
      <>
        <p className="text-ink-muted">Add an email so you can sign in on new devices and reset your password.</p>
        <Button asChild size="sm" variant="primary" className="mt-3 self-start"><Link to="/account">Add your email<ArrowRight /></Link></Button>
      </>
    );
  }

  async function confirm(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError('');
    const res = await verifyCode(pending!, code);
    setBusy(false);
    if (!res.ok) return setError(res.message);
    await qc.invalidateQueries({ queryKey: ['me'] });
  }

  async function resend() {
    setError('');
    const res = await setEmail(pending!);
    setNote(res.ok ? 'Sent. Check your inbox (and spam).' : res.message);
  }

  return (
    <form onSubmit={confirm} className="flex flex-col gap-3">
      <p className="text-ink-muted">
        We sent a 6-digit code to <b className="break-all text-ink">{pending}</b>. Tap the link in that email, or type the code here.
      </p>
      <div className="flex flex-wrap items-end gap-2">
        <div className="w-44">
          <label htmlFor="setup-code" className="sr-only">6-digit code</label>
          <input
            id="setup-code" className="field text-center text-xl tracking-[0.35em]" inputMode="numeric" autoComplete="one-time-code"
            placeholder="000000" pattern="[0-9]{6}" maxLength={6} value={code} onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))} required
          />
        </div>
        <Button type="submit" variant="primary" disabled={busy || code.length !== 6}>Confirm</Button>
      </div>
      {error && <p role="alert" className="err">{error}</p>}
      <p className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-muted" role="status">
        <Button variant="link" className="min-h-0 text-sm" onClick={resend}>Send a new code</Button>
        <Link to="/account" className="text-sm">Use a different email</Link>
        {note && <span>{note}</span>}
      </p>
    </form>
  );
}
