import { useCallback, useEffect, useState } from 'react';
import { createInvite, listMembers, setMemberStatus, type Member } from './api';

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
    <div className="max-w-160">
      <h1 className="page-title">Family</h1>
      <p className="sub">Invite people, re-invite after a lost phone, or disable access.</p>
      <div className="card" style={{ marginBottom: 16 }}>
        <button className="primary" onClick={() => invite()}>Create invite link</button>
        {error && <p role="alert" className="err">{error}</p>}
        {link && (
          <>
            <p className="sub" style={{ margin: '14px 0 0' }}>{link.label} · single use · expires in 7 days</p>
            <div className="linkbox">
              <input readOnly value={link.url} aria-label="Invite link" onFocus={(e) => e.target.select()} />
              <button onClick={() => copy(link.url)}>Copy</button>
            </div>
          </>
        )}
      </div>
      <div className="card">
        {members.map((m) => (
          <div className="row" key={m.id}>
            <span className="dot" style={{ background: m.color }} />
            <div className="grow">
              {m.name} {m.role === 'admin' && <span className="sub">· admin</span>}
            </div>
            <span className={m.status === 'active' ? 'pill' : 'pill off'}>
              {m.status === 'active' ? 'Active' : 'Disabled'}
            </span>
            {m.status === 'active' && <button onClick={() => invite(m)}>Re-invite</button>}
            {m.role !== 'admin' && <button onClick={() => toggle(m)}>{m.status === 'active' ? 'Disable' : 'Enable'}</button>}
          </div>
        ))}
      </div>
      {toast && <div className="toast" role="status">{toast}</div>}
    </div>
  );
}
