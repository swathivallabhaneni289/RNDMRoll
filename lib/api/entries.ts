import * as FileSystem from 'expo-file-system/legacy';
import { api, ApiError } from '@/lib/api/client';
import { fileSize } from '@/lib/api/profile';
import type { ApiErrorCode, Today } from '@/lib/api/types';
import type { ReactionName } from '@/lib/entry/reactions';

/**
 * The entry calls (plan 03-02 Task 1). Modelled on `lib/api/profile.ts`: thin calls over the typed
 * client, no reshaping. Paths, field names and error codes are the wire contract of plan 03-01
 * Task 3b exactly. The phone sends the date and the spin number it showed and never works out
 * the date, the window, the state or whether a post is late: all of that comes back from the server.
 *
 * Plan 03-03 adds get, update, delete and skip to this file.
 */

/** Where a photo came from. Stored with the photo and never shown. */
export type PhotoSource = 'camera' | 'library';

export type EntryCategory = { id: string; name: string };

/** One photo of an entry. `url` is a link; a storage key is never in an Entry. */
export type EntryPhoto = { id: string; url: string; position: number };

export type Entry = {
  id: string;
  /** The civil date the entry belongs to, "2026-10-08". */
  local_date: string;
  /** The category as written on that day. */
  category: EntryCategory;
  title: string;
  /** The stored whole number 1 to 10 (7 means 3.5 stars). */
  stars: number;
  reaction: ReactionName | null;
  comment: string | null;
  /** One to ten photos in position order; the first is the cover. */
  photos: EntryPhoto[];
  /** The cover's link (the first item of `photos`). */
  photo_url: string;
  /** The cover's source. */
  photo_source: PhotoSource;
  late: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateEntryRequest = {
  /** The `local_date` of the spin that was on screen. */
  for_date: string;
  /** The `seq` of that spin. */
  spin_seq: number;
  title: string;
  stars: number;
  reaction?: ReactionName | null;
  comment?: string;
  /** One to ten uploaded photos in order; the first is the cover. */
  photos: { photo_key: string; photo_source: PhotoSource }[];
};

export type CreateEntryResponse = {
  /** True when the server already had this post (a retry, or another phone). */
  already: boolean;
  entry: Entry;
  today: Today;
};

type EntryPhotoTicket = { upload_url: string; photo_key: string; expires_in: number };

/**
 * Uploads one picked photo and returns its `photo_key`, which `createEntry` then names. The same
 * three steps as `uploadAvatar`: a ticket from POST /entry-photos/upload-url with the file's exact
 * byte length (`fileSize`), the bytes PUT straight to storage with the phone's file uploader
 * (never the `api` client: that request must not carry our Authorization header), and a check of
 * the PUT's outcome. One ticket is asked per photo (30 a minute); the caller uploads one photo
 * after another and keeps each key with its picked file so a retry never uploads a photo twice.
 *
 * Errors, as `uploadAvatar` raises them: a refused ticket is `photo_rejected`; storage refusing the
 * bytes (4xx) is `photo_rejected`; any other failed PUT is `photo_upload_failed`; a PUT that
 * throws is `network_unavailable`.
 */
export async function uploadEntryPhoto(localUri: string, contentType: string): Promise<string> {
  const contentLength = await fileSize(localUri);

  let ticket: EntryPhotoTicket;
  try {
    ticket = await api.post<EntryPhotoTicket>('/entry-photos/upload-url', {
      content_type: contentType,
      content_length: contentLength,
    });
  } catch (err) {
    // A refused ticket means the file itself is the problem (type or size).
    if (err instanceof ApiError && (err.code === 'validation_failed' || err.code === 'payload_too_large')) {
      throw new ApiError(err.status, 'photo_rejected');
    }
    throw err;
  }

  let result: FileSystem.FileSystemUploadResult;
  try {
    result = await FileSystem.uploadAsync(ticket.upload_url, localUri, {
      httpMethod: 'PUT',
      headers: { 'Content-Type': contentType },
      uploadType: FileSystem.FileSystemUploadType.BINARY_CONTENT,
    });
  } catch {
    throw new ApiError(0, 'network_unavailable');
  }

  // Never hand back a key for an object that was not written.
  if (result.status < 200 || result.status >= 300) {
    // Storage refusing the bytes (4xx) is a type or size problem; anything else is a failed upload.
    const rejected = result.status >= 400 && result.status < 500;
    throw new ApiError(result.status, rejected ? 'photo_rejected' : 'photo_upload_failed');
  }

  return ticket.photo_key;
}

/**
 * Posts the entry (POST /entries). 201 for a new one, 200 for the same post again (`already`).
 * The answer carries the new Today, which the caller puts into the Today store.
 */
export async function createEntry(body: CreateEntryRequest): Promise<CreateEntryResponse> {
  return api.post<CreateEntryResponse>('/entries', body);
}

/** The four answers that end the attempt: UI spec 3.6, word for word. */
const TERMINAL_MESSAGES: Partial<Record<ApiErrorCode, string>> = {
  day_settled: 'This day already has a post or a skip.',
  day_closed: "This day has closed. It can't be posted or skipped any more.",
  no_spin: 'There is no spin for this day.',
  spin_changed: 'Your spin changed on another phone. Go back to Today to see it.',
};

/** The client's own text for a code, so the existing strings are not written a second time here. */
function clientText(code: ApiErrorCode): string {
  return new ApiError(0, code).userMessage;
}

/**
 * The line for a failed post (or photo upload) and whether it ends the attempt.
 *
 * Terminal: `day_settled`, `day_closed`, `no_spin` and `spin_changed`. The screen shows the
 * message, the button becomes "Back to Today" and Today is asked for again.
 *
 * Not terminal (the form stays as it was and Save is live again): a lost connection, which uses
 * the existing offline string; a photo that was refused or failed to upload, which use their
 * existing strings (UI spec 7); and everything else (a 400 `validation_failed` on `photos`, a
 * 429, a 500, a thrown error), which uses the existing generic string.
 */
export function entryErrorMessage(err: unknown): { text: string; terminal: boolean } {
  if (err instanceof ApiError) {
    const terminal = TERMINAL_MESSAGES[err.code];
    if (terminal !== undefined) return { text: terminal, terminal: true };
    if (
      err.code === 'network_unavailable' ||
      err.code === 'photo_rejected' ||
      err.code === 'photo_upload_failed'
    ) {
      return { text: clientText(err.code), terminal: false };
    }
  }
  return { text: clientText('server_error'), terminal: false };
}
