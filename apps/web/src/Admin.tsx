import { useCallback, useEffect, useState } from 'react';
import { Copy, UserPlus } from 'lucide-react';
import { createInvite, listMembers, setMemberStatus, type Member } from './api';
import { Avatar } from './Nav';
import { Button } from './ui/button';
import { Blob, Sticker } from './ui/sticker';
import { Toast } from './ui/toast';

export function Admin() {
  const [members, setMembers] = useState<Member[]>([]);
  const [link, setLink] = useState<{ label: string; url: string } | null>(null);
  const [error, setError] = useState('');
  const [toast, setToast] = useState('');

  const refresh = useCallback(() => listMembers().then(setMembers), []);
  useEffect(() => void refresh(), [refresh]);

  async function invite(m?: Member) {
    setError('');
    const res = await createInvite(m?.id);
    if ('error' in res) return setError(res.error);
    setLink({ label: m ? `Re-invite for ${m.name}` : 'New invite', url: window.location.origin + res.path });
  }

  async function copy(url: string) {
    try {
      await navigator.clipboard.writeText(url);
      setToast('Invite link copied');
    } catch {
      setToast('Select the link and copy it');
    }
    setTimeout(() => setToast(''), 2500);
  }

  async function toggle(m: Member) {
    await setMemberStatus(m.id, m.status === 'active' ? 'disable' : 'enable');
    refresh();
  }

  return (
    <div className="relative isolate max-w-180">
      <Blob shape="blob-b" className="-top-24 -right-20 size-64 bg-lilac" />
      <div className="relative mb-6">
        <p className="scribble md:text-[26px]">the whole crew</p>
        <h1 className="text-[34px] leading-none md:text-[46px]">Family</h1>
        <p className="mt-2 text-ink-muted">Invite people, re-invite after a lost phone, or turn off access.</p>
      </div>
      <div className="card relative mb-6 flex flex-col gap-3 p-5">
        <Button variant="primary" className="self-start" onClick={() => invite()}><UserPlus />Create invite link</Button>
        {error && <p role="alert" className="err">{error}</p>}
        {link && (
          <>
            <p className="text-sm font-medium text-ink-muted">{link.label} · single use · expires in 7 days</p>
            <div className="flex gap-2">
              <input readOnly className="field" value={link.url} aria-label="Invite link" onFocus={(e) => e.target.select()} />
              <Button variant="sticker" onClick={() => copy(link.url)}><Copy />Copy</Button>
            </div>
          </>
        )}
      </div>
      <ul className="card relative m-0 list-none px-5 py-1">
        {members.map((m) => (
          <li className="dashed-row flex flex-wrap items-center gap-3 py-3.5" key={m.id}>
            <Avatar member={m} />
            <div className="min-w-0 flex-1">
              <span className="font-bold">{m.name}</span>
              {m.role === 'admin' && <Sticker tilt={-2} className="ml-2 bg-lilac py-0 text-xs">admin</Sticker>}
            </div>
            <Sticker className={m.status === 'active' ? 'bg-mint' : 'bg-rose'}>{m.status === 'active' ? 'Active' : 'Disabled'}</Sticker>
            {m.status === 'active' && <Button size="sm" variant="flat" onClick={() => invite(m)}>Re-invite</Button>}
            {m.role !== 'admin' && <Button size="sm" variant="flat" onClick={() => toggle(m)}>{m.status === 'active' ? 'Disable' : 'Enable'}</Button>}
          </li>
        ))}
      </ul>
      {toast && <Toast>{toast}</Toast>}
    </div>
  );
}
