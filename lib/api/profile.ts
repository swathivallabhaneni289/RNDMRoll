import { api, ApiError } from '@/lib/api/client';
import type { ApiUser, AvatarUploadTicket } from '@/lib/api/types';

/**
 * Shared profile read/write calls used by both the profile view and profile
 * edit screens. Request/response shapes are the exact GET/PATCH /me and
 * POST /me/avatar/upload-url contract plan 01-11 built and documented in
 * 01-11-SUMMARY.md; this module does not reshape any of it.
 */

/**
 * Every field is optional; an omitted key means unchanged, matching the
 * server's nil-means-unchanged contract for PATCH /me.
 */
export interface ProfilePatch {
  name?: string;
  username?: string;
  bio?: string;
  avatar_url?: string;
  /** YYYY-MM-DD. Accepted once, only for an unfinished account with no birthday on file. */
  birthday?: string;
}

const UPLOAD_TIMEOUT_MS = 30_000;

export async function fetchProfile(): Promise<ApiUser> {
  return api.get<ApiUser>('/me');
}

/**
 * Sends only the keys actually present on `patch` to PATCH /me, so saving
 * one field (e.g. a bio edit) never rewrites an untouched field (e.g.
 * username) and cannot trip a spurious username conflict. A 409
 * `username_taken` response surfaces as an `ApiError` with its
 * `suggestions` array carried through unchanged; this function does not
 * catch or reshape that error, so the caller (the edit screen) can render
 * the same taken state the create-profile screen renders.
 */
export async function updateProfile(patch: ProfilePatch, token?: string): Promise<ApiUser> {
  const body: ProfilePatch = {};
  if (patch.name !== undefined) body.name = patch.name;
  if (patch.username !== undefined) body.username = patch.username;
  if (patch.bio !== undefined) body.bio = patch.bio;
  if (patch.avatar_url !== undefined) body.avatar_url = patch.avatar_url;
  if (patch.birthday !== undefined) body.birthday = patch.birthday;
  return api.patch<ApiUser>('/me', body, token ? { token } : {});
}

/**
 * Performs the same three-step upload the create-profile screen performs:
 * request a ticket from POST /me/avatar/upload-url, PUT the binary directly
 * to the returned `upload_url` with a matching Content-Type header (a raw
 * `fetch`, not the `api` client, since this request goes straight to object
 * storage and must not carry our Authorization header), and return the
 * `public_url`. Never calls PATCH /me itself; the caller decides when to
 * persist, which is what lets the edit screen batch an avatar change
 * together with a name/username/bio change into a single save.
 */
export async function uploadAvatar(
  localUri: string,
  contentType: string,
  contentLength: number,
  token?: string
): Promise<string> {
  // `token` is for the one moment the session is not stored yet: sign-up uploads the
  // photo with the new account's access token before signing in.
  let ticket: AvatarUploadTicket;
  try {
    ticket = await api.post<AvatarUploadTicket>(
      '/me/avatar/upload-url',
      {
        content_type: contentType,
        content_length: contentLength,
      },
      token ? { token } : {}
    );
  } catch (err) {
    // A refused ticket means the file itself is the problem (type or size).
    if (err instanceof ApiError && (err.code === 'validation_failed' || err.code === 'payload_too_large')) {
      throw new ApiError(err.status, 'photo_rejected');
    }
    throw err;
  }

  const fileResponse = await fetch(localUri);
  const fileBlob = await fileResponse.blob();

  // A hung storage host must not hold the page for the OS's minute-long default: abort
  // after 30 s and report it like any other lost connection.
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), UPLOAD_TIMEOUT_MS);
  let putResponse: Response;
  try {
    putResponse = await fetch(ticket.upload_url, {
      method: 'PUT',
      headers: { 'Content-Type': contentType },
      body: fileBlob,
      signal: controller.signal,
    });
  } catch {
    throw new ApiError(0, 'network_unavailable');
  } finally {
    clearTimeout(timer);
  }

  // T-01-PRV-05's mitigation is the storage provider rejecting a
  // type/size mismatch bound into the presigned request -- that rejection
  // is only a real mitigation if this function actually checks the PUT's
  // outcome instead of returning a public_url for an object that was
  // never written.
  if (!putResponse.ok) {
    // Storage refusing the bytes (4xx) is a type or size problem; anything else is a failed upload.
    const rejected = putResponse.status >= 400 && putResponse.status < 500;
    throw new ApiError(putResponse.status, rejected ? 'photo_rejected' : 'photo_upload_failed');
  }

  return ticket.public_url;
}
