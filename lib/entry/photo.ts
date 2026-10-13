/**
 * Photo limits and the upload memory of the entry form (UI spec 7, plan 03-02 Task 1 and
 * binding decision 6). Pure: no React, no imports.
 */

/** The largest photo, 50 MiB: the same ceiling as profile photos. */
export const PHOTO_MAX_BYTES = 50 * 1024 * 1024;

/** The types the server's upload ticket accepts. */
export const PHOTO_TYPES: readonly string[] = ['image/jpeg', 'image/png', 'image/webp'];

/** An entry holds 1 to 10 photos; the first is the cover. */
export const PHOTOS_MAX = 10;

/** True while the entry has room for another photo. */
export function canAddPhoto(count: number): boolean {
  return count < PHOTOS_MAX;
}

export type PhotoProblem = 'type' | 'size';

/**
 * Why a picked file is refused before any upload, or null when it may be used. A missing type
 * counts as `image/jpeg` (the profile page sends `mimeType ?? 'image/jpeg'` the same way); the
 * type is checked first, then the size (over 50 MiB is `size`).
 */
export function photoProblem(mimeType: string | null | undefined, size: number): PhotoProblem | null {
  const type = mimeType ?? 'image/jpeg';
  if (!PHOTO_TYPES.includes(type)) return 'type';
  if (size > PHOTO_MAX_BYTES) return 'size';
  return null;
}

/** The line shown under the photo strip for a refused file. */
export function photoMessage(p: PhotoProblem): string {
  return p === 'type' ? 'Use a JPG, PNG or WebP photo.' : 'That photo is over 50 MB. Choose a smaller one.';
}

/**
 * The retry rule. Each picked file is uploaded once: `ensure` runs `upload()` the first time a
 * `uri` is asked for and hands every later call for that `uri` the same key (two calls made at
 * the same moment share the one upload). An upload that fails is not remembered, so the next
 * try uploads again. `forget(uri)` drops that one file's key (a photo was removed) and
 * `forget()` drops every key.
 */
export type UploadMemory = {
  ensure(uri: string, upload: () => Promise<string>): Promise<string>;
  forget(uri?: string): void;
};

export function createUploadMemory(): UploadMemory {
  // The promise itself is kept, set before `ensure` returns, so a second caller that arrives
  // while the upload is still running gets the same promise and no second upload starts.
  const attempts = new Map<string, Promise<string>>();

  return {
    ensure(uri, upload) {
      const known = attempts.get(uri);
      if (known) return known;
      // Wrapped so a function that throws at once becomes a rejected promise like any failure.
      const attempt = new Promise<string>((resolve) => {
        resolve(upload());
      });
      attempts.set(uri, attempt);
      attempt.catch(() => {
        // Only this attempt's own entry is dropped: a forget followed by a newer try stays.
        if (attempts.get(uri) === attempt) attempts.delete(uri);
      });
      return attempt;
    },
    forget(uri) {
      if (uri === undefined) attempts.clear();
      else attempts.delete(uri);
    },
  };
}
