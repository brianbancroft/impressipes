export type Member = {
  id: string;
  name: string;
  color: string;
  role: 'admin' | 'member';
  status: 'active' | 'disabled';
};

export type InviteInfo = {
  state: 'valid' | 'used' | 'expired' | 'disabled' | 'invalid';
  reinvite?: boolean;
  name?: string;
  color?: string;
};

async function send(method: string, path: string, body?: unknown) {
  return fetch(`/api${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
}

export type Me = { kind: 'member'; member: Member } | { kind: 'anonymous' } | { kind: 'disabled' };

export async function getMe(): Promise<Me> {
  const r = await send('GET', '/me');
  if (r.ok) return { kind: 'member', member: await r.json() };
  return { kind: r.status === 403 ? 'disabled' : 'anonymous' };
}

export async function getInvite(token: string): Promise<InviteInfo> {
  const r = await send('GET', `/invites/${token}`);
  return r.json();
}

export async function acceptInvite(
  token: string,
  body: { name: string; color: string; credentialId: string },
): Promise<{ ok: true; member: Member } | { ok: false; reason: 'gone' | 'disabled' | 'error' }> {
  const r = await send('POST', `/invites/${token}/accept`, body);
  if (r.ok) return { ok: true, member: await r.json() };
  return { ok: false, reason: r.status === 410 ? 'gone' : r.status === 403 ? 'disabled' : 'error' };
}

export async function listMembers(): Promise<Member[]> {
  const r = await send('GET', '/members');
  return r.ok ? r.json() : [];
}

export async function createInvite(memberId?: string): Promise<{ path: string } | { error: string }> {
  const r = await send('POST', '/invites', { memberId });
  const data = await r.json();
  return r.ok ? data : { error: data.error ?? 'Could not create invite' };
}

export async function setMemberStatus(id: string, action: 'disable' | 'enable') {
  await send('POST', `/members/${id}/${action}`);
}
