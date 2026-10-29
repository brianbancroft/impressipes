import createClient from 'openapi-fetch';
import type { components, paths } from './api-gen/schema.gen';

export type Member = components['schemas']['Member'];
export type RecipeSummary = components['schemas']['RecipeSummary'];
export type Recipe = components['schemas']['Recipe'];
export type RecipeInput = components['schemas']['RecipeInput'];
export type RecipeMark = components['schemas']['RecipeMark'];
export type Ingredient = components['schemas']['Ingredient'];
export type Step = components['schemas']['Step'];
export type ShoppingList = components['schemas']['List'];
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

export const apiUrl = (path?: string) => (path ? `/api${path}` : undefined);

function must<T>(data: T | undefined, error: { error?: string } | undefined, fallback: string): T {
  if (data === undefined) throw new Error(error?.error ?? fallback);
  return data;
}

export async function listRecipes(params: { q?: string; tag?: string; mark?: RecipeMark }) {
  const { data, error } = await client.GET('/recipes', { params: { query: params } });
  return must(data, error, 'Could not load recipes');
}

export async function listTags() {
  const { data, error } = await client.GET('/tags');
  return must(data, error, 'Could not load tags');
}

export async function getRecipe(recipeId: string) {
  const { data, error } = await client.GET('/recipes/{recipeId}', { params: { path: { recipeId } } });
  return must(data, error, 'Could not load recipe');
}

export async function saveRecipe(recipeId: string | undefined, body: RecipeInput) {
  if (recipeId) {
    const { data, error } = await client.PUT('/recipes/{recipeId}', { params: { path: { recipeId } }, body });
    return must(data, error, 'Could not save recipe');
  }
  const { data, error } = await client.POST('/recipes', { body });
  return must(data, error, 'Could not save recipe');
}

export async function deleteRecipe(recipeId: string) {
  await client.DELETE('/recipes/{recipeId}', { params: { path: { recipeId } } });
}

export async function setMark(recipeId: string, mark: RecipeMark, on: boolean) {
  const params = { params: { path: { recipeId, mark } } };
  await (on ? client.PUT('/recipes/{recipeId}/marks/{mark}', params) : client.DELETE('/recipes/{recipeId}/marks/{mark}', params));
}

export async function listLists() {
  const { data, error } = await client.GET('/lists');
  return must(data, error, 'Could not load lists');
}

export async function addRecipeToList(listId: string, recipeId: string, servings?: number) {
  const { data, error } = await client.POST('/lists/{listId}/add-recipe', {
    params: { path: { listId } },
    body: { recipeId, servings },
  });
  return must(data, error, 'Could not add to list');
}

export type ListDetail = components['schemas']['ListDetail'];
export type Item = components['schemas']['Item'];
export type Category = components['schemas']['Category'];
export type ItemPatch = components['schemas']['ItemPatch'];
export type ItemInput = components['schemas']['ItemInput'];

export async function createList(body: components['schemas']['ListInput']) {
  const { data, error } = await client.POST('/lists', { body });
  return must(data, error, 'Could not create list');
}

export async function getList(listId: string) {
  const { data, error } = await client.GET('/lists/{listId}', { params: { path: { listId } } });
  return must(data, error, 'Could not load list');
}

export async function archiveList(listId: string) {
  await client.DELETE('/lists/{listId}', { params: { path: { listId } } });
}

export async function createItem(listId: string, body: ItemInput) {
  const { data, error } = await client.POST('/lists/{listId}/items', { params: { path: { listId } }, body });
  return must(data, error, 'Could not add item');
}

export async function updateItem(itemId: string, body: ItemPatch) {
  const { data, error } = await client.PATCH('/items/{itemId}', { params: { path: { itemId } }, body });
  return must(data, error, 'Could not update item');
}

export async function deleteItem(itemId: string) {
  await client.DELETE('/items/{itemId}', { params: { path: { itemId } } });
}

export async function suggestItems(listId: string, q: string) {
  const { data } = await client.GET('/lists/{listId}/suggestions', { params: { path: { listId }, query: { q } } });
  return data ?? [];
}
