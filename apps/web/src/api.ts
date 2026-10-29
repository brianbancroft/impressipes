import createClient from 'openapi-fetch';
import type { components, paths } from './api-gen/schema.gen';

export type Member = components['schemas']['Member'];
export type InviteInfo = components['schemas']['InviteInfo'];

const client = createClient<paths>({ baseUrl: '/api' });

export type Me = { kind: 'member'; member: Member } | { kind: 'anonymous' } | { kind: 'disabled' };

export async function getMe(): Promise<Me> {
  const { data, response } = await client.GET('/me');
  if (data) return { kind: 'member', member: data };
  return { kind: response.status === 403 ? 'disabled' : 'anonymous' };
}

export async function getInvite(token: string): Promise<InviteInfo> {
  const { data, error } = await client.GET('/invites/{token}', { params: { path: { token } } });
  return (data ?? error)!;
}

export async function acceptInvite(
  token: string,
  body: { name: string; color: string; credentialId: string },
): Promise<{ ok: true; member: Member } | { ok: false; reason: 'gone' | 'disabled' | 'error' }> {
  const { data, response } = await client.POST('/invites/{token}/accept', {
    params: { path: { token } },
    body,
  });
  if (data) return { ok: true, member: data };
  return { ok: false, reason: response.status === 410 ? 'gone' : response.status === 403 ? 'disabled' : 'error' };
}

export async function listMembers(): Promise<Member[]> {
  const { data } = await client.GET('/members');
  return data ?? [];
}

export async function createInvite(memberId?: string): Promise<{ path: string } | { error: string }> {
  const { data, error } = await client.POST('/invites', { body: { memberId } });
  return data ?? { error: error?.error ?? 'Could not create invite' };
}

export async function setMemberStatus(id: string, action: 'disable' | 'enable') {
  const path = action === 'disable' ? '/members/{id}/disable' : '/members/{id}/enable';
  await client.POST(path, { params: { path: { id } } });
}
