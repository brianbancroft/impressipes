import {
  beginPasskeyLogin, beginPasskeyRegistration, finishPasskeyLogin, finishPasskeyRegistration,
  linkApple, signInWithApple, type AuthResult, type SimpleResult,
} from './api';

// WebAuthn carries binary values as ArrayBuffers; the API carries them as base64url.
const fromB64 = (s: string): ArrayBuffer => {
  const bin = atob(s.replace(/-/g, '+').replace(/_/g, '/'));
  return Uint8Array.from(bin, (c) => c.charCodeAt(0)).buffer;
};

const toB64 = (buf: ArrayBuffer | null): string | undefined =>
  buf ? btoa(String.fromCharCode(...new Uint8Array(buf))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '') : undefined;

type Descriptor = { id: string; type: string; transports?: string[] };
type Wire = Record<string, unknown> & { challenge: string; allowCredentials?: Descriptor[]; excludeCredentials?: Descriptor[]; user?: { id: string } };

function decode(publicKey: Wire): PublicKeyCredentialCreationOptions & PublicKeyCredentialRequestOptions {
  const withIds = (list?: Descriptor[]) => list?.map((d) => ({ ...d, id: fromB64(d.id) }));
  return {
    ...publicKey,
    challenge: fromB64(publicKey.challenge),
    user: publicKey.user && { ...publicKey.user, id: fromB64(publicKey.user.id) },
    allowCredentials: withIds(publicKey.allowCredentials),
    excludeCredentials: withIds(publicKey.excludeCredentials),
  } as never;
}

export const passkeysSupported = () => typeof window !== 'undefined' && !!window.PublicKeyCredential;

const cancelled = (e: unknown) => e instanceof DOMException && (e.name === 'NotAllowedError' || e.name === 'AbortError');
const quiet = { ok: false as const, status: 0, message: '' };

/** Signs in with whichever passkey the device offers. A dismissed prompt returns an empty message. */
export async function signInWithPasskey(): Promise<AuthResult> {
  const begin = await beginPasskeyLogin();
  if (!begin.ok) return begin;
  try {
    const cred = (await navigator.credentials.get({
      publicKey: decode((begin.ceremony.options as { publicKey: Wire }).publicKey),
    })) as PublicKeyCredential | null;
    if (!cred) return quiet;
    const r = cred.response as AuthenticatorAssertionResponse;
    return await finishPasskeyLogin(begin.ceremony.ceremonyId, {
      id: cred.id, rawId: toB64(cred.rawId), type: cred.type,
      response: {
        clientDataJSON: toB64(r.clientDataJSON), authenticatorData: toB64(r.authenticatorData),
        signature: toB64(r.signature), userHandle: toB64(r.userHandle),
      },
    });
  } catch (e) {
    return cancelled(e) ? quiet : { ok: false, status: 0, message: 'Your device couldn’t use a passkey. Try another way to sign in.' };
  }
}

/** Creates a passkey on this device (iCloud Keychain on Apple devices, so it follows the member everywhere). */
export async function addPasskey(name: string): Promise<SimpleResult> {
  const begin = await beginPasskeyRegistration();
  if (!begin.ok) return begin;
  try {
    const cred = (await navigator.credentials.create({
      publicKey: decode((begin.ceremony.options as { publicKey: Wire }).publicKey),
    })) as PublicKeyCredential | null;
    if (!cred) return quiet;
    const r = cred.response as AuthenticatorAttestationResponse;
    return await finishPasskeyRegistration(
      begin.ceremony.ceremonyId,
      {
        id: cred.id, rawId: toB64(cred.rawId), type: cred.type,
        response: {
          clientDataJSON: toB64(r.clientDataJSON), attestationObject: toB64(r.attestationObject),
          transports: r.getTransports?.() ?? [],
        },
      },
      name,
    );
  } catch (e) {
    if (e instanceof DOMException && e.name === 'InvalidStateError') {
      return { ok: false, status: 0, message: 'This device already has a passkey for your account.' };
    }
    return cancelled(e) ? quiet : { ok: false, status: 0, message: 'Your device couldn’t create a passkey.' };
  }
}

/** A short label for the passkey list, from the browser's platform. */
export function deviceName() {
  const ua = navigator.userAgent;
  if (/iPhone/.test(ua)) return 'iPhone';
  if (/iPad/.test(ua)) return 'iPad';
  if (/Mac/.test(ua)) return 'Mac';
  if (/Android/.test(ua)) return 'Android';
  if (/Windows/.test(ua)) return 'Windows';
  return 'This device';
}

// ---- Sign in with Apple ----

declare global {
  interface Window {
    AppleID?: {
      auth: {
        init(c: Record<string, unknown>): void;
        signIn(): Promise<{ authorization: { id_token: string } }>;
      };
    };
  }
}

const APPLE_JS = 'https://appleid.cdn-apple.com/appleauth/static/jsapi/appleid/1/en_US/appleid.auth.js';

function loadAppleJS(): Promise<void> {
  if (window.AppleID) return Promise.resolve();
  return new Promise((resolve, reject) => {
    const el = document.createElement('script');
    el.src = APPLE_JS;
    el.onload = () => resolve();
    el.onerror = () => reject(new Error('Apple sign-in could not load'));
    document.head.append(el);
  });
}

async function sha256Hex(s: string) {
  const sum = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(s));
  return [...new Uint8Array(sum)].map((b) => b.toString(16).padStart(2, '0')).join('');
}

/** Opens Apple's popup and hands back the identity token with the raw nonce it was bound to. */
async function appleToken(clientId: string): Promise<{ token: string; nonce: string } | null> {
  await loadAppleJS();
  const nonce = crypto.randomUUID();
  window.AppleID!.auth.init({
    clientId, scope: 'email', redirectURI: window.location.origin, usePopup: true, nonce: await sha256Hex(nonce),
  });
  try {
    const res = await window.AppleID!.auth.signIn();
    return { token: res.authorization.id_token, nonce };
  } catch {
    return null; // closed the popup
  }
}

export async function signInApple(clientId: string): Promise<AuthResult> {
  try {
    const t = await appleToken(clientId);
    return t ? await signInWithApple(t.token, t.nonce) : quiet;
  } catch {
    return { ok: false, status: 0, message: 'Apple sign-in could not start. Try another way to sign in.' };
  }
}

export async function connectApple(clientId: string): Promise<SimpleResult> {
  try {
    const t = await appleToken(clientId);
    return t ? await linkApple(t.token, t.nonce) : quiet;
  } catch {
    return { ok: false, status: 0, message: 'Apple sign-in could not start.' };
  }
}
