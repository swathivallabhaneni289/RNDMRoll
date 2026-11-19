import { api } from '@/lib/api/client';

/**
 * Ask for a second look at a post or comment an admin removed (developer, 2026-11-19): why it
 * should come back, 1 to 500 characters once trimmed, and up to 3 photos, each uploaded first with
 * uploadEntryPhoto (lib/api/entries.ts) and named by its key. `removal` is the bell item's
 * `removal`, the removal's full id. One request per removal, inside its 30 days: 201 for a new one
 * and 200 for the same words and photos sent again (an answer the phone gave up on after its 10
 * seconds), both {"second_look": "asked"}. The refusals are a 400 validation_failed on `text` or
 * `photo_keys`, a 404, and the 409s second_look_asked, second_look_too_late and second_look_back;
 * secondLookFailure (lib/notifications/moderation.ts) words them.
 */

/** POST /removals/{id}/second-look {text, photo_keys}. */
export async function askSecondLook(removal: string, text: string, photoKeys: string[]): Promise<void> {
  await api.post<unknown>(`/removals/${encodeURIComponent(removal)}/second-look`, { text, photo_keys: photoKeys });
}
